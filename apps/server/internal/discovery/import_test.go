package discovery

import (
	"context"
	"errors"
	"io"
	"path"
	"testing"

	"github.com/google/uuid"
)

/*
L'import.

Un test domine tous les autres ici : celui qui vérifie qu'on ne peut pas faire
télécharger n'importe quoi par l'instance. Sans ce garde, la route d'import est
un relais anonyme doublé d'un sondeur de réseau interne — le client choisit
l'adresse, et c'est le serveur qui va la chercher, depuis l'intérieur.

Le reste vérifie le nom de fichier, qui n'est pas cosmétique : il devient la clé
de l'objet, et c'est lui que l'indexation analyse pour en tirer série et tome.
*/

// importService monte un service prêt à enfiler et à exécuter des imports.
func importService(source Source, client *fakeClient) (*Service, *fakeQueue) {
	queue := &fakeQueue{}
	service := quietService(&fakeRepo{sources: []Source{source}}, client)
	service.SetImportQueue(queue)
	return service, queue
}

/*
runOne enchaîne la demande et son exécution.

Le worker le fait en deux temps, séparés par la file ; les tests qui portent sur
le RÉSULTAT de l'import n'ont pas besoin de cette séparation, et la rejouer à
chaque cas noierait ce qu'ils vérifient. Ceux qui portent sur la file, eux,
appellent RequestImport directement.
*/
func runOne(
	t *testing.T, service *Service, queue *fakeQueue, p ImportParams, deposit Deposit,
) Import {
	t.Helper()

	record, err := service.RequestImport(context.Background(), p)
	if err != nil {
		t.Fatalf("demande d'import : %v", err)
	}
	if len(queue.enqueued) == 0 {
		t.Fatal("rien n'a été enfilé")
	}

	if err := service.RunImport(context.Background(), record.ID, deposit); err != nil {
		t.Fatalf("exécution de l'import : %v", err)
	}

	done, err := service.repo.GetImport(context.Background(), record.ID)
	if err != nil {
		t.Fatal(err)
	}
	return done
}

// capture retient ce que l'import a voulu déposer, sans backend de stockage.
func capture(into *DepositParams, body *string) Deposit {
	return func(_ context.Context, p DepositParams) (Deposited, error) {
		raw, err := io.ReadAll(p.Content)
		if err != nil {
			return Deposited{}, err
		}
		*body = string(raw)
		*into = p
		return Deposited{
			ComicID:   uuid.New(),
			ObjectKey: path.Join(p.Folder, p.Filename),
			Title:     p.Filename,
			Format:    "cbz",
			Size:      int64(len(raw)),
		}, nil
	}
}

func TestImportDeposits(t *testing.T) {
	source := Source{
		ID: uuid.New(), Name: "Voisin", URL: "https://voisin.test/opds", Enabled: true,
	}
	href := "https://voisin.test/dl/garage.cbz"

	client := &fakeClient{files: map[string]string{href: "PK\x03\x04 contenu"}}

	var got DepositParams
	var body string

	service, queue := importService(source, client)
	done := runOne(t, service, queue, ImportParams{
		SourceID:  source.ID,
		Href:      href,
		LibraryID: uuid.New(),
		Folder:    "Mœbius",
		Title:     "Le Garage hermétique",
	}, capture(&got, &body))

	if body != "PK\x03\x04 contenu" {
		t.Errorf("contenu déposé = %q", body)
	}
	if got.Folder != "Mœbius" {
		t.Errorf("dossier = %q", got.Folder)
	}
	if got.Filename != "garage.cbz" {
		t.Errorf("nom = %q, attendu celui de l'adresse", got.Filename)
	}
	if done.Status != ImportDone {
		t.Errorf("statut = %q, attendu done (%s)", done.Status, done.ErrorCode)
	}
	if done.ComicID == nil {
		t.Error("aucun album rattaché à l'import")
	}
	if done.ObjectKey == "" {
		t.Error("la clé de l'objet doit être consignée : c'est ce que l'interface montre")
	}
}

/*
TestImportRefusesForeignHost est le test qui justifie ce fichier.

Le client envoie l'adresse. Sans contrainte, il obtiendrait de l'instance
qu'elle télécharge n'importe quoi — un fichier arbitraire sur Internet, ou une
adresse interne que l'extérieur n'atteint pas.

La contrainte est que l'adresse appartienne au catalogue annoncé. Un
administrateur a déclaré cet hôte ; c'est lui, et lui seul, qui délimite ce que
le serveur va chercher.
*/
func TestImportRefusesForeignHost(t *testing.T) {
	source := Source{
		ID: uuid.New(), Name: "Voisin", URL: "https://voisin.test/opds", Enabled: true,
	}

	cases := map[string]string{
		"un autre hôte":            "https://ailleurs.test/malware.cbz",
		"un sous-domaine voisin":   "https://mechant.voisin.test.attaquant.fr/x.cbz",
		"le réseau interne":        "http://192.168.1.1/admin",
		"les métadonnées du nuage": "http://169.254.169.254/latest/meta-data/",
		"un autre schéma":          "http://voisin.test/dl/garage.cbz",
	}

	for name, href := range cases {
		t.Run(name, func(t *testing.T) {
			client := &fakeClient{files: map[string]string{href: "contenu"}}

			service, queue := importService(source, client)

			_, err := service.RequestImport(context.Background(), ImportParams{
				SourceID:  source.ID,
				Href:      href,
				LibraryID: uuid.New(),
			})

			// Le refus est IMMÉDIAT, pas différé dans une ligne de suivi :
			// l'adresse est une propriété de la demande, et rien ne justifie
			// de faire croire à l'utilisateur que son import est parti.
			if err == nil {
				t.Fatalf("l'import de %s a été accepté", href)
			}
			if !errors.Is(err, ErrForeignHost) && !errors.Is(err, ErrInvalidSource) {
				t.Errorf("err = %v, attendu un refus d'adresse", err)
			}
			if len(queue.enqueued) != 0 {
				t.Error("un job a été enfilé malgré le refus")
			}
		})
	}
}

// TestImportUnknownSource : un catalogue absent n'autorise aucune adresse.
func TestImportUnknownSource(t *testing.T) {
	source := Source{
		ID: uuid.New(), Name: "Voisin", URL: "https://voisin.test/opds", Enabled: true,
	}
	client := &fakeClient{files: map[string]string{"https://voisin.test/x.cbz": "x"}}

	service, queue := importService(source, client)

	_, err := service.RequestImport(context.Background(), ImportParams{
		// Un identifiant que la base ne connaît pas : sans catalogue, il n'y a
		// pas d'hôte autorisé, donc pas d'import.
		SourceID:  uuid.New(),
		Href:      "https://voisin.test/x.cbz",
		LibraryID: uuid.New(),
	})

	if !errors.Is(err, ErrSourceNotFound) {
		t.Fatalf("err = %v, attendu ErrSourceNotFound", err)
	}
	if len(queue.enqueued) != 0 {
		t.Error("un job a été enfilé pour un catalogue inconnu")
	}
}

/*
TestImportFailureIsRecordedNotRetried couvre le cœur du passage en tâche de fond.

Un import qui échoue ne rend pas d'erreur à River, et c'est délibéré : ses
échecs sont déterministes — catalogue éteint, format refusé, fichier déjà
présent — et les retenter trois fois à intervalles croissants ferait patienter
l'utilisateur devant un « en cours » qui ne peut pas aboutir.

L'échec doit donc atterrir dans la ligne de suivi, avec un code que l'interface
sait traduire. Sans cela, un import de fond serait une action qu'on lance et qui
disparaît.
*/
func TestImportFailureIsRecordedNotRetried(t *testing.T) {
	source := Source{
		ID: uuid.New(), Name: "Voisin", URL: "https://voisin.test/opds", Enabled: true,
	}

	// Aucun fichier servi à cette adresse : le catalogue répondra 404.
	client := &fakeClient{files: map[string]string{}}

	service, queue := importService(source, client)

	record, err := service.RequestImport(context.Background(), ImportParams{
		SourceID:  source.ID,
		Href:      "https://voisin.test/dl/absent.cbz",
		LibraryID: uuid.New(),
	})
	if err != nil {
		t.Fatalf("la demande doit être acceptée : l'échec ne se voit qu'en essayant (%v)", err)
	}
	if record.Status != ImportQueued {
		t.Errorf("statut initial = %q, attendu queued", record.Status)
	}
	if len(queue.enqueued) != 1 {
		t.Fatalf("%d jobs enfilés, attendu 1", len(queue.enqueued))
	}

	var got DepositParams
	var body string

	if err := service.RunImport(
		context.Background(), record.ID, capture(&got, &body),
	); err != nil {
		t.Fatalf("l'échec a été rendu à la file : %v — il serait retenté en vain", err)
	}

	done, err := service.repo.GetImport(context.Background(), record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if done.Status != ImportFailed {
		t.Errorf("statut = %q, attendu failed", done.Status)
	}
	if done.ErrorCode == "" {
		t.Error("un échec doit porter un code : l'interface le traduit")
	}
	if done.ErrorDetail == "" {
		t.Error("un échec doit porter un diagnostic : c'est ce qui permet de comprendre")
	}
	if body != "" {
		t.Error("un contenu a été déposé malgré l'échec")
	}
}

/*
TestImportIsNotReplayed protège contre la double écriture.

River peut relancer un job dont la réussite n'a pas été enregistrée à temps.
Sans garde, la seconde tentative se heurterait au refus d'écraser et
marquerait en échec un import qui a parfaitement fonctionné.
*/
func TestImportIsNotReplayed(t *testing.T) {
	source := Source{
		ID: uuid.New(), Name: "Voisin", URL: "https://voisin.test/opds", Enabled: true,
	}
	href := "https://voisin.test/dl/garage.cbz"
	client := &fakeClient{files: map[string]string{href: "contenu"}}

	service, queue := importService(source, client)

	var got DepositParams
	var body string
	done := runOne(t, service, queue, ImportParams{
		SourceID: source.ID, Href: href, LibraryID: uuid.New(),
	}, capture(&got, &body))

	deposits := 0
	counting := func(ctx context.Context, p DepositParams) (Deposited, error) {
		deposits++
		return capture(&got, &body)(ctx, p)
	}

	if err := service.RunImport(context.Background(), done.ID, counting); err != nil {
		t.Fatalf("rejeu : %v", err)
	}
	if deposits != 0 {
		t.Errorf("%d dépôts au rejeu : un import abouti ne doit pas se rejouer", deposits)
	}

	after, err := service.repo.GetImport(context.Background(), done.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Status != ImportDone {
		t.Errorf("statut après rejeu = %q, attendu done", after.Status)
	}
}

/*
TestImportQueueFailureIsVisible : une file en panne doit se voir.

Sans ce traitement, la ligne resterait « en attente » indéfiniment, ce qui est
indiscernable d'un simple retard pour qui la regarde — le pire des deux mondes.
*/
func TestImportQueueFailureIsVisible(t *testing.T) {
	source := Source{
		ID: uuid.New(), Name: "Voisin", URL: "https://voisin.test/opds", Enabled: true,
	}
	client := &fakeClient{files: map[string]string{}}

	service, queue := importService(source, client)
	queue.err = errors.New("file indisponible")

	_, err := service.RequestImport(context.Background(), ImportParams{
		SourceID:  source.ID,
		Href:      "https://voisin.test/dl/x.cbz",
		LibraryID: uuid.New(),
	})
	if err == nil {
		t.Fatal("une file en panne doit faire échouer la demande")
	}

	records, err := service.ListImports(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("%d lignes de suivi, attendu 1", len(records))
	}
	if records[0].Status != ImportFailed || records[0].ErrorCode != "queue" {
		t.Errorf("ligne = %+v, attendue en échec avec le code queue", records[0])
	}
}

/*
TestFilenameFor couvre les trois façons de nommer un import.

Le nom devient la clé de l'objet et nourrit l'analyse du nom de fichier, d'où
série et tome sont tirés. Un import nommé « file » donne un album nommé
« file » — et un catalogue sur deux sert précisément `/api/v1/books/42/file`.
*/
func TestFilenameFor(t *testing.T) {
	cases := []struct {
		name    string
		fetched Fetched
		href    string
		title   string
		want    string
	}{
		{
			name:    "le catalogue déclare un nom",
			fetched: Fetched{Filename: "Le Garage hermétique.cbz"},
			href:    "https://x.test/api/v1/books/42/file",
			title:   "Autre chose",
			want:    "Le Garage hermétique.cbz",
		},
		{
			name:    "un nom déclaré avec un chemin est ramené à sa base",
			fetched: Fetched{Filename: "../../etc/passwd"},
			href:    "https://x.test/f",
			want:    "passwd",
		},
		{
			name:    "l'adresse porte une extension reconnue",
			fetched: Fetched{},
			href:    "https://x.test/download/le-garage.cbz",
			title:   "Le Garage hermétique",
			want:    "le-garage.cbz",
		},
		{
			name:    "adresse muette : le titre et le type de contenu",
			fetched: Fetched{ContentType: "application/epub+zip"},
			href:    "https://x.test/api/v1/books/42/file",
			title:   "Le Garage hermétique",
			want:    "Le Garage hermétique.epub",
		},
		{
			name:    "type de contenu avec paramètres",
			fetched: Fetched{ContentType: "application/pdf; charset=binary"},
			href:    "https://x.test/f",
			title:   "Album",
			want:    "Album.pdf",
		},
		{
			name:    "rien du tout : le CBZ, que l'ingestion vérifiera",
			fetched: Fetched{},
			href:    "https://x.test/f",
			title:   "",
			want:    "import.cbz",
		},
		{
			name:    "une extension non reconnue dans l'adresse ne suffit pas",
			fetched: Fetched{ContentType: "application/vnd.comicbook+zip"},
			href:    "https://x.test/telecharger.php",
			title:   "Album",
			want:    "Album.cbz",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := filenameFor(tc.fetched, tc.href, tc.title); got != tc.want {
				t.Errorf("filenameFor = %q, attendu %q", got, tc.want)
			}
		})
	}
}
