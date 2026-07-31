package discovery

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

/*
Enrichissement après import.

Ce que ces tests protègent n'est pas le chemin heureux — il est court — mais les
trois refus qui le rendent acceptable : ne pas écrire sous le seuil, ne pas
faire échouer un import à cause d'une base publique, ne pas toucher au titre.
*/

type fakeWriter struct {
	calls    int
	comicID  uuid.UUID
	summary  string
	language string
	err      error
}

func (w *fakeWriter) Enrich(_ context.Context, id uuid.UUID, summary, language string) error {
	w.calls++
	w.comicID, w.summary, w.language = id, summary, language
	return w.err
}

// fakeDescriber rend une fiche fixe, quelle que soit l'œuvre demandée.
type fakeDescriber struct {
	kind        string
	description Description
	err         error
}

func (d *fakeDescriber) Kind() string             { return d.kind }
func (d *fakeDescriber) Name() string             { return d.kind }
func (d *fakeDescriber) Capabilities() Capability { return CanDescribe }

func (d *fakeDescriber) Describe(_ context.Context, w Work) ([]Description, error) {
	if d.err != nil {
		return nil, d.err
	}
	out := d.description
	out.ProviderKind = d.kind
	out.ProviderName = d.kind
	out.Confidence = MatchConfidence(w, out)
	return []Description{out}, nil
}

func enrichService(describer DescriptionProvider, writer ComicWriter) *Service {
	service := quietService(&fakeRepo{}, &fakeClient{})

	registry := NewRegistry()
	if describer != nil {
		registry.Register(describer)
	}
	service.SetMetadata(registry)
	service.SetComicWriter(writer)
	return service
}

func TestEnrichAppliesConfidentMatch(t *testing.T) {
	writer := &fakeWriter{}
	service := enrichService(&fakeDescriber{
		kind: "essai",
		description: Description{
			Title:    "L'Incal",
			Authors:  []string{"Moebius"},
			Summary:  "John Difool et l'Incal lumière.",
			Language: "fr",
		},
	}, writer)

	comicID := uuid.New()
	service.enrichImported(context.Background(), comicID, "L'Incal")

	if writer.calls != 1 {
		t.Fatalf("%d écritures, attendu 1", writer.calls)
	}
	if writer.comicID != comicID {
		t.Error("l'enrichissement a visé un autre album")
	}
	if writer.summary == "" || writer.language != "fr" {
		t.Errorf("fiche appliquée = %+v", writer)
	}
}

/*
TestEnrichRefusesWeakMatch est le test qui justifie le seuil.

Personne ne regarde pendant qu'un job tourne. Une fiche fausse mais plausible ne
se remarque pas — elle se découvre des mois plus tard, quand on cherche pourquoi
le résumé d'un album parle d'autre chose. Une absence, elle, se voit tout de
suite.
*/
func TestEnrichRefusesWeakMatch(t *testing.T) {
	writer := &fakeWriter{}
	service := enrichService(&fakeDescriber{
		kind: "essai",
		description: Description{
			// Titre proche mais pas identique, et aucun auteur commun : la
			// confiance restera sous le seuil.
			Title:   "L'Incal, une autre édition",
			Summary: "Résumé d'une œuvre voisine.",
		},
	}, writer)

	service.enrichImported(context.Background(), uuid.New(), "L'Incal")

	if writer.calls != 0 {
		t.Errorf("%d écritures : un rapprochement faible a été appliqué", writer.calls)
	}
}

// TestEnrichSurvivesProviderFailure : un import réussi le reste.
//
// L'album est là et lisible ; son résumé manquera, ce qui est exactement l'état
// dans lequel il serait arrivé sans cette étape.
func TestEnrichSurvivesProviderFailure(t *testing.T) {
	writer := &fakeWriter{}
	service := enrichService(
		&fakeDescriber{kind: "essai", err: errors.New("base injoignable")}, writer)

	service.enrichImported(context.Background(), uuid.New(), "L'Incal")

	if writer.calls != 0 {
		t.Errorf("%d écritures malgré l'échec de la base", writer.calls)
	}
}

// TestEnrichWithoutRegistry : une instance hors ligne n'écrit rien et ne panique
// pas.
func TestEnrichWithoutRegistry(t *testing.T) {
	writer := &fakeWriter{}
	service := quietService(&fakeRepo{}, &fakeClient{})
	service.SetComicWriter(writer)

	service.enrichImported(context.Background(), uuid.New(), "L'Incal")

	if writer.calls != 0 {
		t.Errorf("%d écritures sans registre", writer.calls)
	}
}

// TestEnrichIgnoresEmptyTitle : sans titre, il n'y a rien à rapprocher.
func TestEnrichIgnoresEmptyTitle(t *testing.T) {
	writer := &fakeWriter{}
	service := enrichService(&fakeDescriber{
		kind:        "essai",
		description: Description{Title: "Peu importe", Summary: "x"},
	}, writer)

	service.enrichImported(context.Background(), uuid.New(), "")

	if writer.calls != 0 {
		t.Errorf("%d écritures pour un album sans titre", writer.calls)
	}
}

// TestEnrichSkipsEmptyDescription : une fiche sûre mais vide n'apporte rien, et
// l'écrire ferait une requête pour rien.
func TestEnrichSkipsEmptyDescription(t *testing.T) {
	writer := &fakeWriter{}
	service := enrichService(&fakeDescriber{
		kind:        "essai",
		description: Description{Title: "L'Incal", Authors: []string{"Moebius"}},
	}, writer)

	service.enrichImported(context.Background(), uuid.New(), "L'Incal")

	if writer.calls != 0 {
		t.Errorf("%d écritures pour une fiche sans contenu", writer.calls)
	}
}

/*
TestEnrichRejectsGenericTitles est le garde-fou qui porte le vrai poids.

« Intégrale », « Tome 1 », « Volume 2 » sont des titres exacts que des dizaines
d'œuvres portent, et les noms de fichiers de bande dessinée en sont pleins. Deux
bases qui s'accordent sur un tel titre ne s'accordent sur rien : elles décrivent
deux œuvres différentes qui partagent une étiquette.

Aucun score ne détecte cela, puisque le titre correspond exactement.
*/
func TestEnrichRejectsGenericTitles(t *testing.T) {
	generic := []string{
		"Intégrale", "intégrales", "Tome", "Tome 3", "Volume 2", "Album",
		"Hors-série", "One Shot", "Recueil", "Coffret", "Anthologie",
		"01", "2024", "T3", "",
	}

	for _, title := range generic {
		t.Run(title, func(t *testing.T) {
			writer := &fakeWriter{}
			service := enrichService(&fakeDescriber{
				kind:        "essai",
				description: Description{Title: title, Summary: "Un résumé quelconque."},
			}, writer)

			service.enrichImported(context.Background(), uuid.New(), title)

			if writer.calls != 0 {
				t.Errorf("%q a été enrichi : un titre générique n'identifie rien", title)
			}
		})
	}

	// Le contrepoids : un titre court mais distinctif doit passer.
	for _, title := range []string{"Akira", "Persepolis", "L'Incal"} {
		t.Run(title, func(t *testing.T) {
			if tooGeneric(title) {
				t.Errorf("%q écarté à tort : c'est un titre d'œuvre", title)
			}
		})
	}
}

/*
TestCorroboratedRefusesDisagreement vérifie la règle sur la fonction elle-même.

Elle n'est pas observable depuis `enrichImported` : celui-ci ne connaît que le
titre, or un candidat au titre divergent tombe sous le seuil et se trouve
écarté avant d'avoir pu contredire quoi que ce soit. La contradiction n'apparaît
que lorsqu'un accord d'auteur remonte un titre divergent au-dessus du seuil —
le cas de l'écran de correction, où l'auteur est connu.

C'est aussi ce qui montre que ce garde-fou couvre moins qu'il n'y paraît, et
pourquoi le filtre de généricité existe.
*/
func TestCorroboratedRefusesDisagreement(t *testing.T) {
	want := Work{Title: "L'Incal", Authors: []string{"Moebius"}}

	accord := []Description{
		{ProviderKind: "a", Title: "L'Incal", Authors: []string{"Moebius"}, Summary: "x"},
		{ProviderKind: "b", Title: "l'incal", Authors: []string{"Moebius"}, Language: "fr"},
	}
	for i := range accord {
		accord[i].Confidence = MatchConfidence(want, accord[i])
	}
	if _, ok := corroborated(accord, EnrichThreshold); !ok {
		t.Error("deux bases qui désignent la même œuvre doivent se corroborer")
	}

	desaccord := []Description{
		{ProviderKind: "a", Title: "L'Incal", Authors: []string{"Moebius"}, Summary: "x"},
		{ProviderKind: "b", Title: "L'Incal noir", Authors: []string{"Moebius"}, Summary: "y"},
	}
	for i := range desaccord {
		desaccord[i].Confidence = MatchConfidence(want, desaccord[i])
		if desaccord[i].Confidence < EnrichThreshold {
			t.Fatalf("le cas testé est mal construit : %q est sous le seuil (%.2f)",
				desaccord[i].Title, desaccord[i].Confidence)
		}
	}
	if _, ok := corroborated(desaccord, EnrichThreshold); ok {
		t.Error("deux bases qui proposent des œuvres différentes ne doivent rien écrire")
	}
}

// TestEnrichPrefersTheMostCompleteAgreement : à œuvre égale, la fiche la plus
// utile l'emporte.
//
// Les bases ne remplissent pas les mêmes champs — l'une a le résumé, l'autre la
// langue. Prendre la première rendrait le résultat dépendant de l'ordre de tri.
func TestEnrichPrefersTheMostCompleteAgreement(t *testing.T) {
	writer := &fakeWriter{}

	service := quietService(&fakeRepo{}, &fakeClient{})
	registry := NewRegistry()
	registry.Register(&fakeDescriber{
		kind:        "aaa-pauvre",
		description: Description{Title: "L'Incal", Authors: []string{"Moebius"}},
	})
	registry.Register(&fakeDescriber{
		kind: "zzz-riche",
		description: Description{
			Title:    "L'Incal",
			Authors:  []string{"Moebius"},
			Summary:  "John Difool.",
			Language: "fr",
		},
	})
	service.SetMetadata(registry)
	service.SetComicWriter(writer)

	service.enrichImported(context.Background(), uuid.New(), "L'Incal")

	if writer.calls != 1 {
		t.Fatalf("%d écritures, attendu 1", writer.calls)
	}
	if writer.summary == "" || writer.language != "fr" {
		t.Errorf("la fiche la plus complète n'a pas été retenue : %+v", writer)
	}
}
