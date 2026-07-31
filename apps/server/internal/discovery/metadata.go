package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

/*
Socle commun aux bases de métadonnées.

Les trois fournisseurs — Open Library, Google Books, Internet Archive — diffèrent
par leur URL et la forme de leur JSON. Tout le reste est identique, et mérite
d'être écrit une fois : espacer les requêtes, mémoriser les réponses, borner le
temps et la taille, décoder.

# Ce qu'ils ont en commun et qui n'est pas évident

Aucun n'a d'adresse configurable. C'est ce qui les sépare d'un catalogue OPDS,
et ce qui justifie qu'ils soient enregistrés au démarrage plutôt que déclarés en
base : leur hôte est connu, écrit dans le code, et un administrateur n'a rien à
y saisir. Le contrôle anti-SSRF ne s'y applique donc pas — il protège des
adresses saisies, il n'y en a aucune ici.

Le champ `baseURL` existe malgré tout, et uniquement pour les tests : les
vérifier contre les vrais services les rendrait lents, fragiles et impolis.
*/

const (
	// maxMetadataBytes borne une réponse de base de métadonnées.
	//
	// Une page de résultats fait quelques dizaines de kilo-octets. La borne
	// n'existe pas contre les grosses réponses mais contre un service qui
	// répondrait sans fin à un serveur qui lui fait confiance.
	maxMetadataBytes = 4 << 20

	// metadataTimeout borne une interrogation.
	//
	// Plus court que pour un catalogue OPDS : une base de métadonnées sert à
	// enrichir, jamais à fournir. Elle ne doit pas retenir une recherche
	// fédérée dont elle n'est qu'un complément.
	metadataTimeout = 5 * time.Second

	// metadataLimit borne les candidats demandés à une base.
	//
	// Au-delà, on ne fait plus du rapprochement mais du parcours : l'écran de
	// correction montre une poignée de candidats, pas une liste à explorer.
	metadataLimit = 10
)

/*
metadataSource porte ce qui est commun aux trois fournisseurs.

Composée plutôt qu'héritée : chaque fournisseur l'embarque et n'expose que sa
propre traduction du JSON, ce qui laisse le contrat `DescriptionProvider` net.
*/
type metadataSource struct {
	kind    string
	name    string
	baseURL string

	http     *http.Client
	throttle *Throttle
	memo     *Memo
}

func newMetadataSource(kind, name, baseURL string, deps MetadataDeps) metadataSource {
	client := deps.HTTP
	if client == nil {
		client = &http.Client{Timeout: metadataTimeout}
	}
	return metadataSource{
		kind:     kind,
		name:     name,
		baseURL:  baseURL,
		http:     client,
		throttle: deps.Throttle,
		memo:     deps.Memo,
	}
}

/*
MetadataDeps rassemble ce que partagent les fournisseurs.

Le débit et le cache sont fournis plutôt que construits : ils doivent être
partagés entre tous les appelants pour valoir quelque chose. Un cache par
fournisseur-instance ne mémoriserait rien d'une recherche à l'autre, et un
limiteur par instance laisserait passer autant de requêtes qu'on a construit
d'objets.
*/
type MetadataDeps struct {
	HTTP     *http.Client
	Throttle *Throttle
	Memo     *Memo
}

func (m metadataSource) Kind() string             { return m.kind }
func (m metadataSource) Name() string             { return m.name }
func (m metadataSource) Capabilities() Capability { return CanDescribe }

/*
fetchJSON interroge la base et décode sa réponse.

L'ordre des opérations compte : le cache AVANT le limiteur de débit. L'inverse
ferait attendre son tour à une requête dont la réponse est déjà connue, ce qui
transformerait une politesse envers un service tiers en lenteur pour rien.
*/
func (m metadataSource) fetchJSON(ctx context.Context, target string, into any) error {
	key := MemoKey(m.kind, "", target)

	if m.memo != nil {
		if raw, ok := m.memo.Get(key); ok {
			if body, isBytes := raw.([]byte); isBytes {
				return json.Unmarshal(body, into)
			}
		}
	}

	if m.throttle != nil {
		if err := m.throttle.Wait(ctx, m.kind); err != nil {
			return err
		}
	}

	ctx, cancel := context.WithTimeout(ctx, metadataTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return fmt.Errorf("%s : requête invalide : %w", m.kind, err)
	}
	req.Header.Set("Accept", "application/json")
	// Un agent identifiable est une exigence explicite d'Open Library et une
	// politesse partout ailleurs : un service financé par des dons doit pouvoir
	// nommer qui le sollicite avant de décider de bloquer.
	req.Header.Set("User-Agent", "boxincloud (+https://github.com/adonko3xBitters/boxincloud)")

	resp, err := m.http.Do(req)
	if err != nil {
		return fmt.Errorf("%s injoignable : %w", m.kind, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s a répondu %s", m.kind, resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxMetadataBytes))
	if err != nil {
		return fmt.Errorf("%s : lecture : %w", m.kind, err)
	}

	// Les octets sont mémorisés, pas la structure décodée : deux appelants
	// partageraient sinon la même valeur, et le premier qui la modifierait
	// corromprait la copie du second.
	if m.memo != nil {
		m.memo.Put(key, body)
	}

	return json.Unmarshal(body, into)
}

/*
queryFor compose les termes de recherche d'une œuvre.

Un ISBN seul quand on l'a : c'est le seul identifiant qui rende le rapprochement
certain, et y ajouter un titre approximatif ne ferait que du bruit.
*/
func queryFor(w Work) string {
	if w.ISBN != "" {
		return normalizeISBN(w.ISBN)
	}

	terms := w.Title
	if len(w.Authors) > 0 && w.Authors[0] != "" {
		terms += " " + w.Authors[0]
	}
	return terms
}

// scored calcule la confiance et écarte le bruit manifeste.
//
// Zéro signifie « aucun rapport » : une base rend toujours quelque chose, et
// laisser passer un candidat sans point commun encombrerait l'écran de
// correction de propositions absurdes.
func scored(want Work, candidates []Description) []Description {
	out := make([]Description, 0, len(candidates))
	for _, candidate := range candidates {
		candidate.Confidence = MatchConfidence(want, candidate)
		if candidate.Confidence <= 0 {
			continue
		}
		out = append(out, candidate)
	}
	return out
}

// firstString rend la première chaîne d'un champ que ces API écrivent
// indifféremment en chaîne ou en tableau.
func firstString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []any:
		for _, item := range typed {
			if s, ok := item.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}

// allStrings normalise un champ chaîne-ou-tableau en liste.
func allStrings(value any) []string {
	switch typed := value.(type) {
	case string:
		if typed == "" {
			return nil
		}
		return []string{typed}
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if s, ok := item.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func escape(value string) string { return url.QueryEscape(value) }
