package discovery

import (
	"context"
	"errors"
	"io"
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

func importService(source Source, client *fakeClient) *Service {
	return quietService(&fakeRepo{sources: []Source{source}}, client)
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
		return Deposited{ComicID: uuid.New(), Title: p.Filename}, nil
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

	result, err := importService(source, client).Import(context.Background(), ImportParams{
		SourceID:  source.ID,
		Href:      href,
		LibraryID: uuid.New(),
		Folder:    "Mœbius",
		Title:     "Le Garage hermétique",
	}, capture(&got, &body))
	if err != nil {
		t.Fatalf("import : %v", err)
	}

	if body != "PK\x03\x04 contenu" {
		t.Errorf("contenu déposé = %q", body)
	}
	if got.Folder != "Mœbius" {
		t.Errorf("dossier = %q", got.Folder)
	}
	if got.Filename != "garage.cbz" {
		t.Errorf("nom = %q, attendu celui de l'adresse", got.Filename)
	}
	if result.ComicID == uuid.Nil {
		t.Error("aucun album rendu")
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

			var got DepositParams
			var body string

			_, err := importService(source, client).Import(context.Background(), ImportParams{
				SourceID:  source.ID,
				Href:      href,
				LibraryID: uuid.New(),
			}, capture(&got, &body))

			if err == nil {
				t.Fatalf("l'import de %s a été accepté", href)
			}
			if !errors.Is(err, ErrForeignHost) && !errors.Is(err, ErrInvalidSource) {
				t.Errorf("err = %v, attendu un refus d'adresse", err)
			}
			if body != "" {
				t.Error("un contenu a été déposé malgré le refus")
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

	var got DepositParams
	var body string

	_, err := importService(source, client).Import(context.Background(), ImportParams{
		// Un identifiant que la base ne connaît pas : sans catalogue, il n'y a
		// pas d'hôte autorisé, donc pas d'import.
		SourceID:  uuid.New(),
		Href:      "https://voisin.test/x.cbz",
		LibraryID: uuid.New(),
	}, capture(&got, &body))

	if !errors.Is(err, ErrSourceNotFound) {
		t.Fatalf("err = %v, attendu ErrSourceNotFound", err)
	}
	if body != "" {
		t.Error("un contenu a été déposé pour un catalogue inconnu")
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
