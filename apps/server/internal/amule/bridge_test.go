package amule

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/adonko3xBitters/boxincloud/server/internal/storage/local"
)

/*
Le pont, testé sur un VRAI répertoire.

Le publieur est doublé — l'ingestion est déjà éprouvée ailleurs, et la
réimplémenter ici ne prouverait rien — mais le répertoire d'arrivée est réel, lu
par le même `storage.Provider` qu'en production. C'est le seul moyen de vérifier
ce qui casse vraiment : un fichier qui n'est pas là, un nom qui ne correspond
pas, un montage absent.
*/

// fakePublisher retient ce qu'on lui a demandé de publier.
type fakePublisher struct {
	mu sync.Mutex

	calls    []publishCall
	failWith error
}

type publishCall struct {
	LibraryID uuid.UUID
	Folder    string
	Filename  string
	Size      int64
	Content   string
}

func (p *fakePublisher) Publish(
	_ context.Context, libraryID uuid.UUID, folder, filename string,
	size int64, content io.Reader,
) (uuid.UUID, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.failWith != nil {
		return uuid.Nil, p.failWith
	}

	// Le contenu est lu pour de bon : un pont qui passerait un lecteur vide
	// publierait des albums de zéro octet sans que rien ne le signale.
	body, err := io.ReadAll(content)
	if err != nil {
		return uuid.Nil, err
	}

	p.calls = append(p.calls, publishCall{
		LibraryID: libraryID,
		Folder:    folder,
		Filename:  filename,
		Size:      size,
		Content:   string(body),
	})
	return uuid.New(), nil
}

func (p *fakePublisher) count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.calls)
}

// bridgeFixture monte un service avec un répertoire d'arrivée réel.
func bridgeFixture(t *testing.T) (*Service, *fakeRepo, *fakePublisher, string) {
	t.Helper()

	dir := t.TempDir()
	provider, err := local.New(local.Options{Root: dir, ReadOnly: true})
	if err != nil {
		t.Fatalf("répertoire d'arrivée : %v", err)
	}

	repo := &fakeRepo{}
	svc := newTestService(t, repo, enabled())
	publisher := &fakePublisher{}

	svc.SetIncoming(provider)
	svc.SetPublisher(publisher)

	return svc, repo, publisher, dir
}

func writeIncoming(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatalf("écriture du fichier d'arrivée : %v", err)
	}
}

func completed(hash, name string, size int64, category int) Snapshot {
	return Snapshot{Downloads: []Download{{
		Hash:     hash,
		Name:     name,
		Size:     size,
		Category: category,
		Status:   DownloadCompleted,
	}}}
}

/*
TestPontPublieCeQuiEstDestineALaBibliotheque.

Le cas nominal, et la raison d'être du module dans ce projet : un fichier
terminé dont la catégorie désigne une bibliothèque y entre, avec son contenu.
*/
func TestPontPublieCeQuiEstDestineALaBibliotheque(t *testing.T) {
	svc, repo, publisher, dir := bridgeFixture(t)
	library := uuid.New()

	if _, err := svc.SetDestination(context.Background(), Destination{
		Category:  1,
		Label:     "BD",
		LibraryID: &library,
		Folder:    "Ajouts",
	}); err != nil {
		t.Fatalf("SetDestination : %v", err)
	}

	writeIncoming(t, dir, "album.cbz", "des octets de bande dessinée")
	snapshot := completed("abc", "album.cbz", 28, 1)

	svc.publishCompleted(context.Background(), &snapshot)

	if publisher.count() != 1 {
		t.Fatalf("%d publication(s), attendu 1", publisher.count())
	}

	call := publisher.calls[0]
	if call.LibraryID != library {
		t.Errorf("bibliothèque = %s, attendu %s", call.LibraryID, library)
	}
	if call.Folder != "Ajouts" || call.Filename != "album.cbz" {
		t.Errorf("destination = %q/%q", call.Folder, call.Filename)
	}
	if call.Content != "des octets de bande dessinée" {
		t.Errorf("contenu publié = %q — le pont a-t-il vraiment lu le fichier ?", call.Content)
	}

	if got := repo.publications["abc"].Status; got != PublicationPublished {
		t.Errorf("état = %q, attendu %q", got, PublicationPublished)
	}
}

/*
TestPontLaisseSurDisqueCeQuiNAPasDeDestination.

Le cas le PLUS fréquent : un client eD2k sert à récupérer toutes sortes de
choses, et seule une partie a sa place dans un catalogue d'albums.

Le résultat est inscrit malgré tout. Sans cette trace, chaque tour de scrutation
reconsidérerait les mêmes fichiers indéfiniment.
*/
func TestPontLaisseSurDisqueCeQuiNAPasDeDestination(t *testing.T) {
	svc, repo, publisher, dir := bridgeFixture(t)

	writeIncoming(t, dir, "distribution.iso", "beaucoup d'octets")
	snapshot := completed("def", "distribution.iso", 17, 0)

	svc.publishCompleted(context.Background(), &snapshot)

	if publisher.count() != 0 {
		t.Errorf("%d publication(s) pour une catégorie sans destination", publisher.count())
	}
	if got := repo.publications["def"].Status; got != PublicationSkipped {
		t.Errorf("état = %q, attendu %q — sans trace, le fichier serait "+
			"reconsidéré à chaque tour", got, PublicationSkipped)
	}
}

/*
TestPontNePubliePasDeuxFois est la garantie qui compte.

Les instantanés se succèdent toutes les secondes, et un fichier terminé le reste
jusqu'à ce que quelqu'un le retire de la file. Sans idempotence, le même album
entrerait cinquante fois dans la bibliothèque en une minute.
*/
func TestPontNePubliePasDeuxFois(t *testing.T) {
	svc, _, publisher, dir := bridgeFixture(t)
	library := uuid.New()

	if _, err := svc.SetDestination(context.Background(), Destination{
		Category: 1, Label: "BD", LibraryID: &library,
	}); err != nil {
		t.Fatalf("SetDestination : %v", err)
	}

	writeIncoming(t, dir, "album.cbz", "contenu")
	snapshot := completed("abc", "album.cbz", 7, 1)

	for range 5 {
		svc.publishCompleted(context.Background(), &snapshot)
	}

	if publisher.count() != 1 {
		t.Errorf("%d publications pour cinq passages : le pont n'est pas idempotent",
			publisher.count())
	}
}

/*
TestPontNommeLeMontageManquant.

Le fichier est terminé côté démon mais absent chez nous : c'est LA cause la plus
fréquente d'un pont qui ne fait rien, et le message doit désigner la variable à
regarder plutôt que de laisser chercher.
*/
func TestPontNommeLeMontageManquant(t *testing.T) {
	svc, repo, publisher, _ := bridgeFixture(t)
	library := uuid.New()

	if _, err := svc.SetDestination(context.Background(), Destination{
		Category: 1, Label: "BD", LibraryID: &library,
	}); err != nil {
		t.Fatalf("SetDestination : %v", err)
	}

	// Rien n'est écrit dans le répertoire : le fichier n'existe pas.
	snapshot := completed("abc", "absent.cbz", 42, 1)
	svc.publishCompleted(context.Background(), &snapshot)

	if publisher.count() != 0 {
		t.Error("une publication a eu lieu alors que le fichier est absent")
	}

	publication := repo.publications["abc"]
	if publication.Status != PublicationError {
		t.Fatalf("état = %q, attendu %q", publication.Status, PublicationError)
	}
	if !strings.Contains(publication.Detail, "BOXINCLOUD_ED2K_INCOMING_DIR") {
		t.Errorf("le détail devrait nommer la variable à vérifier : %q", publication.Detail)
	}
}

// TestPontIgnoreCeQuiNEstPasTermine : un fichier en cours n'a rien à faire dans
// la bibliothèque, et l'y mettre publierait un album tronqué.
func TestPontIgnoreCeQuiNEstPasTermine(t *testing.T) {
	svc, _, publisher, dir := bridgeFixture(t)
	library := uuid.New()

	if _, err := svc.SetDestination(context.Background(), Destination{
		Category: 1, Label: "BD", LibraryID: &library,
	}); err != nil {
		t.Fatalf("SetDestination : %v", err)
	}

	writeIncoming(t, dir, "album.cbz", "moitié")

	for _, status := range []DownloadStatus{
		DownloadDownloading, DownloadPaused, DownloadWaiting, DownloadCompleting, DownloadHashing,
	} {
		snapshot := Snapshot{Downloads: []Download{{
			Hash: "abc", Name: "album.cbz", Category: 1, Status: status,
		}}}
		svc.publishCompleted(context.Background(), &snapshot)
	}

	if publisher.count() != 0 {
		t.Errorf("%d publication(s) pour des fichiers non terminés", publisher.count())
	}
}

// TestPontInactifSansBranchement : un module dont le volume n'est pas monté doit
// fonctionner sans publier, pas planter.
func TestPontInactifSansBranchement(t *testing.T) {
	svc := newTestService(t, &fakeRepo{}, enabled())

	snapshot := completed("abc", "album.cbz", 7, 1)
	svc.publishCompleted(context.Background(), &snapshot) // ne doit pas paniquer
}

// TestSetDestinationNormaliseLeDossier : une barre de tête produirait des clés
// d'objet doublées côté backend.
func TestSetDestinationNormaliseLeDossier(t *testing.T) {
	svc, _, _, _ := bridgeFixture(t)
	library := uuid.New()

	d, err := svc.SetDestination(context.Background(), Destination{
		Category: 2, Label: "Séries", LibraryID: &library, Folder: "/Ajouts/2026/",
	})
	if err != nil {
		t.Fatalf("SetDestination : %v", err)
	}
	if d.Folder != "Ajouts/2026" {
		t.Errorf("dossier = %q, attendu %q", d.Folder, "Ajouts/2026")
	}
}

func TestSetDestinationRefuseUnLibelleVide(t *testing.T) {
	svc, _, _, _ := bridgeFixture(t)

	_, err := svc.SetDestination(context.Background(), Destination{Category: 1, Label: "  "})

	var invalid ValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf("erreur = %v, attendu ValidationError", err)
	}
	if _, named := invalid.Fields["label"]; !named {
		t.Errorf("le champ fautif doit être nommé : %v", invalid.Fields)
	}
}
