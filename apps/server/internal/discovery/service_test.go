package discovery

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

/*
L'agrégation, sans réseau.

Ce que ce fichier vérifie n'est pas la lecture d'un flux — c'est le
comportement qui distingue une recherche fédérée d'une recherche tout court :
elle réussit partiellement, et doit le dire.
*/

type fakeRepo struct {
	sources []Source
}

func (r *fakeRepo) ListSources(context.Context) ([]Source, error) { return r.sources, nil }

func (r *fakeRepo) GetSource(_ context.Context, id uuid.UUID) (Source, error) {
	for _, source := range r.sources {
		if source.ID == id {
			return source, nil
		}
	}
	return Source{}, ErrSourceNotFound
}

func (r *fakeRepo) SourceSecret(context.Context, uuid.UUID) ([]byte, error) { return nil, nil }

func (r *fakeRepo) CreateSource(_ context.Context, s Source, _ []byte) (Source, error) {
	return s, nil
}

func (r *fakeRepo) UpdateSource(_ context.Context, s Source, _ []byte, _ bool) (Source, error) {
	return s, nil
}

func (r *fakeRepo) DeleteSource(context.Context, uuid.UUID) error        { return nil }
func (r *fakeRepo) RecordProbe(context.Context, uuid.UUID, string) error { return nil }

// fakeClient rend ce qu'on lui a dit de rendre, par catalogue.
type fakeClient struct {
	mu      sync.Mutex
	results map[uuid.UUID][]Result
	errs    map[uuid.UUID]error
	delays  map[uuid.UUID]time.Duration
	calls   int

	// Contenus servis par Open, indexés par adresse.
	files        map[string]string
	filenames    map[string]string
	contentTypes map[string]string
}

func (c *fakeClient) Search(
	ctx context.Context, source Source, _ string, _ Query,
) ([]Result, error) {
	c.mu.Lock()
	c.calls++
	delay := c.delays[source.ID]
	err := c.errs[source.ID]
	results := c.results[source.ID]
	c.mu.Unlock()

	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if err != nil {
		return nil, err
	}

	out := make([]Result, len(results))
	copy(out, results)
	for i := range out {
		out[i].SourceID = source.ID
		out[i].SourceName = source.Name
	}
	return out, nil
}

// Open sert le contenu que le catalogue de test est censé héberger.
func (c *fakeClient) Open(
	_ context.Context, source Source, _, href string,
) (Fetched, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.errs[source.ID]; err != nil {
		return Fetched{}, err
	}
	body := c.files[href]
	if body == "" {
		return Fetched{}, errors.New("404")
	}
	return Fetched{
		Body:        io.NopCloser(strings.NewReader(body)),
		Size:        int64(len(body)),
		Filename:    c.filenames[href],
		ContentType: c.contentTypes[href],
	}, nil
}

func (c *fakeClient) Probe(_ context.Context, source Source, _ string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.errs[source.ID]
}

func quietService(repo Repository, client Client) *Service {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewService(repo, client, nil, log)
}

/*
TestSearchPartialFailure est le test central de ce paquet.

Trois catalogues : un qui répond, un éteint, un désactivé. La recherche doit
rendre ce qu'elle a ET dire ce qui manque. Sans cela, l'utilisateur ne peut pas
distinguer « ce titre n'existe nulle part » de « la moitié de tes catalogues
n'a pas répondu » — deux situations qui appellent des actions opposées.
*/
func TestSearchPartialFailure(t *testing.T) {
	ok := Source{ID: uuid.New(), Name: "Debout", Enabled: true}
	down := Source{ID: uuid.New(), Name: "Éteint", Enabled: true}
	off := Source{ID: uuid.New(), Name: "Désactivé", Enabled: false}

	client := &fakeClient{
		results: map[uuid.UUID][]Result{
			ok.ID:  {{Title: "L'Incal"}, {Title: "Le Garage hermétique"}},
			off.ID: {{Title: "Jamais demandé"}},
		},
		errs: map[uuid.UUID]error{down.ID: errors.New("connexion refusée")},
	}

	service := quietService(&fakeRepo{sources: []Source{ok, down, off}}, client)

	got, err := service.Search(context.Background(), Query{Text: "moebius"}, nil)
	if err != nil {
		t.Fatalf("recherche : %v", err)
	}

	if len(got.Results) != 2 {
		t.Errorf("%d résultats, attendu 2 : %+v", len(got.Results), got.Results)
	}

	// Le catalogue désactivé n'est pas interrogé : ni résultat, ni état.
	if len(got.Sources) != 2 {
		t.Fatalf("%d états, attendu 2 (le désactivé ne compte pas)", len(got.Sources))
	}

	states := map[string]SourceStatus{}
	for _, status := range got.Sources {
		states[status.Name] = status
	}

	if states["Debout"].Error != "" || states["Debout"].Count != 2 {
		t.Errorf("catalogue debout mal rapporté : %+v", states["Debout"])
	}
	if states["Éteint"].Error != "unreachable" {
		t.Errorf("catalogue éteint : erreur = %q, attendu unreachable", states["Éteint"].Error)
	}
	if _, asked := states["Désactivé"]; asked {
		t.Error("un catalogue désactivé ne doit pas être interrogé")
	}
}

// TestSearchDedupe vérifie la fusion des doublons entre catalogues.
//
// Le cas se produit dès qu'on fédère deux de ses propres instances. Les liens
// d'acquisition des deux doivent survivre : le choix du catalogue d'où l'on
// télécharge appartient à l'utilisateur.
func TestSearchDedupe(t *testing.T) {
	first := Source{ID: uuid.New(), Name: "A", Enabled: true}
	second := Source{ID: uuid.New(), Name: "B", Enabled: true}

	client := &fakeClient{results: map[uuid.UUID][]Result{
		first.ID: {{
			Title:        "L'Incal",
			Authors:      []string{"Jodorowsky"},
			Acquisitions: []Link{{Href: "https://a.fr/1.cbz"}},
			CoverURL:     "https://a.fr/1.jpg",
		}},
		second.ID: {{
			Title:        "l'incal !",
			Authors:      []string{"JODOROWSKY"},
			Acquisitions: []Link{{Href: "https://b.fr/9.cbz"}},
			Summary:      "Résumé que A n'avait pas.",
		}},
	}}

	service := quietService(&fakeRepo{sources: []Source{first, second}}, client)

	got, err := service.Search(context.Background(), Query{Text: "incal"}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(got.Results) != 1 {
		t.Fatalf("%d résultats, attendu 1 après fusion : %+v", len(got.Results), got.Results)
	}
	merged := got.Results[0]
	if len(merged.Acquisitions) != 2 {
		t.Errorf("%d liens d'acquisition, attendu 2 : le choix du catalogue doit rester",
			len(merged.Acquisitions))
	}
	if merged.CoverURL == "" {
		t.Error("la couverture du premier a été perdue")
	}
	if merged.Summary == "" {
		t.Error("le résumé du second n'a pas complété le premier")
	}
}

// TestSearchMarksOwned vérifie le marqueur « déjà dans votre bibliothèque ».
func TestSearchMarksOwned(t *testing.T) {
	source := Source{ID: uuid.New(), Name: "Distant", Enabled: true}
	client := &fakeClient{results: map[uuid.UUID][]Result{
		source.ID: {{Title: "L'Incal"}, {Title: "Le Garage hermétique"}},
	}}

	service := quietService(&fakeRepo{sources: []Source{source}}, client)

	local := func(context.Context, string, int) ([]string, error) {
		// Écrit autrement que dans le catalogue distant, comme dans la vraie
		// vie : le rapprochement se fait sur le titre normalisé.
		return []string{"l'incal"}, nil
	}

	got, err := service.Search(context.Background(), Query{Text: "moebius"}, local)
	if err != nil {
		t.Fatal(err)
	}

	owned := map[string]bool{}
	for _, result := range got.Results {
		owned[result.Title] = result.InLibrary
	}
	if !owned["L'Incal"] {
		t.Error("un titre déjà possédé n'est pas marqué")
	}
	if owned["Le Garage hermétique"] {
		t.Error("un titre absent est marqué comme possédé")
	}
}

// TestSearchLocalFailureIsNotFatal : ne pas savoir ce qu'on possède est moins
// grave que de ne rien afficher.
func TestSearchLocalFailureIsNotFatal(t *testing.T) {
	source := Source{ID: uuid.New(), Name: "Distant", Enabled: true}
	client := &fakeClient{results: map[uuid.UUID][]Result{
		source.ID: {{Title: "L'Incal"}},
	}}

	service := quietService(&fakeRepo{sources: []Source{source}}, client)

	local := func(context.Context, string, int) ([]string, error) {
		return nil, errors.New("base indisponible")
	}

	got, err := service.Search(context.Background(), Query{Text: "incal"}, local)
	if err != nil {
		t.Fatalf("la recherche a échoué à cause du catalogue local : %v", err)
	}
	if len(got.Results) != 1 {
		t.Errorf("%d résultats, attendu 1", len(got.Results))
	}
}

/*
TestSearchIsParallel vérifie que les catalogues sont interrogés de front.

C'est la fonctionnalité, pas une optimisation : en série, cinq catalogues lents
additionneraient leurs délais et l'utilisateur attendrait une minute pour une
page à moitié vide.
*/
func TestSearchIsParallel(t *testing.T) {
	const delay = 150 * time.Millisecond

	sources := make([]Source, 4)
	client := &fakeClient{
		results: map[uuid.UUID][]Result{},
		delays:  map[uuid.UUID]time.Duration{},
	}
	for i := range sources {
		sources[i] = Source{ID: uuid.New(), Name: string(rune('A' + i)), Enabled: true}
		client.results[sources[i].ID] = []Result{{Title: sources[i].Name}}
		client.delays[sources[i].ID] = delay
	}

	service := quietService(&fakeRepo{sources: sources}, client)

	started := time.Now()
	got, err := service.Search(context.Background(), Query{Text: "x"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	elapsed := time.Since(started)

	if len(got.Results) != len(sources) {
		t.Fatalf("%d résultats, attendu %d", len(got.Results), len(sources))
	}
	// En série il faudrait 600 ms ; en parallèle, un peu plus de 150.
	if elapsed > delay*2 {
		t.Errorf("%v pour %d catalogues à %v : les appels sont sérialisés",
			elapsed, len(sources), delay)
	}
}

// TestSearchOrderIsStable : deux recherches identiques doivent rendre le même
// ordre, sinon la page semble bouger toute seule entre deux frappes.
func TestSearchOrderIsStable(t *testing.T) {
	sources := []Source{
		{ID: uuid.New(), Name: "Zeta", Enabled: true},
		{ID: uuid.New(), Name: "Alpha", Enabled: true},
	}
	client := &fakeClient{results: map[uuid.UUID][]Result{
		sources[0].ID: {{Title: "Z1"}, {Title: "Z2"}},
		sources[1].ID: {{Title: "A1"}},
	}}
	service := quietService(&fakeRepo{sources: sources}, client)

	var reference []string
	for run := 0; run < 8; run++ {
		got, err := service.Search(context.Background(), Query{Text: "x"}, nil)
		if err != nil {
			t.Fatal(err)
		}
		var order []string
		for _, result := range got.Results {
			order = append(order, result.SourceName+"/"+result.Title)
		}
		if reference == nil {
			reference = order
			continue
		}
		for i := range order {
			if order[i] != reference[i] {
				t.Fatalf("ordre instable : %v puis %v", reference, order)
			}
		}
	}
}

// TestSearchEmptyQuery : une recherche vide n'interroge personne.
func TestSearchEmptyQuery(t *testing.T) {
	source := Source{ID: uuid.New(), Name: "Distant", Enabled: true}
	client := &fakeClient{results: map[uuid.UUID][]Result{source.ID: {{Title: "X"}}}}
	service := quietService(&fakeRepo{sources: []Source{source}}, client)

	got, err := service.Search(context.Background(), Query{Text: "   "}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if client.calls != 0 {
		t.Errorf("%d appels pour une recherche vide", client.calls)
	}
	// Des listes vides plutôt que nil : l'interface reçoit du JSON, et `null`
	// n'y est pas une liste vide.
	if got.Results == nil || got.Sources == nil {
		t.Error("les listes doivent être vides, pas nulles")
	}
}
