package discovery

import (
	"context"
	"log/slog"
	"sort"
	"sync"
	"time"
)

/*
Rapprochement de métadonnées.

# Pourquoi ce n'est PAS branché sur la recherche fédérée

L'intention de départ était d'enrichir chaque résultat de recherche dont le
catalogue n'a rendu qu'un titre. Le calcul l'interdit.

Une page rend jusqu'à quarante résultats. Enrichir chacun demande une requête
par base et par résultat, et le débit sortant impose une seconde et demie entre
deux appels à Open Library — la seule façon correcte d'interroger un service
financé par des dons. Quarante résultats font donc une minute d'attente, pour
compléter des lignes qu'on ne regardera pas.

Réduire le débit rendrait la chose rapide et impolie. Réduire le nombre de
résultats enrichis rendrait l'enrichissement arbitraire : pourquoi ces huit-là.

Le rapprochement est donc **toujours déclenché pour UNE œuvre** : celle dont on
corrige la fiche, celle qu'on vient d'importer. Une requête par base, quelques
centaines de millisecondes, et un résultat qu'on a demandé.

# Ce que la fusion fait, et ne fait pas

Elle rassemble les candidats de toutes les bases, les classe par confiance, et
s'arrête là. Elle ne choisit pas.

Choisir revient à écraser des métadonnées, et c'est une décision qui appartient
à l'utilisateur devant son écran de correction — ou, quand personne ne regarde,
au seuil de confiance. Une fiche fausse mais plausible ne se voit pas ; une
absence se voit et se corrige.
*/

// describeTimeout borne l'ensemble du rapprochement.
//
// Il vient d'une action de l'utilisateur, qui attend devant : une base lente ne
// doit pas retenir les autres au-delà de ce qu'on tolère devant un écran.
const describeTimeout = 12 * time.Second

// DescribeResult agrège les propositions de toutes les bases interrogées.
type DescribeResult struct {
	Candidates []Description `json:"candidates"`
	// Sources rapporte ce que chaque base a rendu, ou pourquoi elle n'a rien
	// rendu. Même raison que pour la recherche fédérée : sans cela, une liste
	// courte est indiscernable d'une base en panne.
	Sources []SourceStatus `json:"sources"`
}

// SetMetadata branche le registre de fournisseurs de métadonnées.
//
// Après coup, comme la file d'import : les fournisseurs sont construits au
// démarrage avec le débit et le cache partagés, que ce service ne fabrique pas.
func (s *Service) SetMetadata(registry *Registry) { s.registry = registry }

/*
Describe interroge les bases de métadonnées sur une œuvre.

Les échecs sont rapportés, jamais fatals — même raisonnement que pour la
recherche fédérée : une base publique injoignable est un fait à montrer, pas
une panne de l'instance.
*/
func (s *Service) Describe(ctx context.Context, w Work) (DescribeResult, error) {
	empty := DescribeResult{Candidates: []Description{}, Sources: []SourceStatus{}}

	if s.registry == nil {
		return empty, nil
	}
	if queryFor(w) == "" {
		return empty, nil
	}

	providers := s.registry.Describers()
	if len(providers) == 0 {
		return empty, nil
	}

	ctx, cancel := context.WithTimeout(ctx, describeTimeout)
	defer cancel()

	var (
		mu         sync.Mutex
		candidates []Description
		statuses   []SourceStatus
		wg         sync.WaitGroup
	)

	for _, provider := range providers {
		wg.Add(1)
		go func(provider DescriptionProvider) {
			defer wg.Done()

			started := time.Now()
			found, err := provider.Describe(ctx, w)

			status := SourceStatus{
				Name:      provider.Name(),
				Count:     len(found),
				ElapsedMs: time.Since(started).Milliseconds(),
			}
			if err != nil {
				status.Error = failureCode(err)
				// Au niveau info : une base publique qui ne répond pas n'est pas
				// un défaut de cette instance, et la journaliser en erreur
				// donnerait un bruit permanent.
				s.log.Info("base de métadonnées sans réponse",
					slog.String("source", provider.Kind()), slog.Any("err", err))
			}

			mu.Lock()
			candidates = append(candidates, found...)
			statuses = append(statuses, status)
			mu.Unlock()
		}(provider)
	}

	wg.Wait()

	/*
		Classement par confiance décroissante, puis par genre de fournisseur.

		Le second critère n'est pas décoratif : sans lui, deux candidats de même
		confiance — le cas normal quand deux bases décrivent la même œuvre —
		sortiraient dans un ordre dépendant de qui a répondu le premier, donc
		différent à chaque appel. L'écran de correction semblerait alors se
		réorganiser tout seul entre deux consultations.
	*/
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Confidence != candidates[j].Confidence {
			return candidates[i].Confidence > candidates[j].Confidence
		}
		if candidates[i].ProviderKind != candidates[j].ProviderKind {
			return candidates[i].ProviderKind < candidates[j].ProviderKind
		}
		return candidates[i].Title < candidates[j].Title
	})
	sort.SliceStable(statuses, func(i, j int) bool { return statuses[i].Name < statuses[j].Name })

	if candidates == nil {
		candidates = []Description{}
	}
	if statuses == nil {
		statuses = []SourceStatus{}
	}
	return DescribeResult{Candidates: candidates, Sources: statuses}, nil
}

/*
DescribeBest rend la meilleure fiche, ou rien.

Destinée aux traitements sans personne devant : l'enrichissement d'un album qui
vient d'être importé, plus tard celui d'une bibliothèque entière. Le seuil est
obligatoire et l'appelant doit le choisir en connaissance de cause — c'est lui
qui décide du risque d'écraser une fiche par celle d'un homonyme.
*/
func (s *Service) DescribeBest(
	ctx context.Context, w Work, minConfidence float64,
) (Description, bool, error) {
	result, err := s.Describe(ctx, w)
	if err != nil {
		return Description{}, false, err
	}
	best, ok := Best(result.Candidates, minConfidence)
	return best, ok, nil
}
