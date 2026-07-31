package discovery

import (
	"context"
	"sync"
	"time"
)

/*
Limitation de débit SORTANTE.

À ne pas confondre avec celle du paquet `middleware`, qui protège cette instance
de ses clients. Celle-ci protège les services TIERS de cette instance, et c'est
une obligation autant qu'une prudence : les bases publiques qu'on interroge —
Open Library, Internet Archive — sont gratuites, financées par des dons, et
publient des limites qu'un client correct respecte.

Le cas qui rend cela indispensable n'est pas la recherche d'un utilisateur seul.
C'est l'enrichissement : rapprocher les métadonnées d'une bibliothèque de mille
albums, c'est mille requêtes vers la même API. Sans limite, boxincloud
ressemblerait à une attaque, et se ferait bloquer par adresse — pénalisant tous
ses utilisateurs, pas seulement celui qui a lancé le traitement.

# Attendre plutôt que refuser

C'est l'inverse du choix fait pour la limitation entrante, où un client trop
bavard reçoit un 429.

Ici, celui qu'on limite est notre propre code, dans une tâche de fond qui n'a
rien d'urgent. Refuser l'obligerait à réessayer, donc à réimplémenter l'attente
ailleurs, moins bien. Attendre est plus simple et plus juste : l'enrichissement
de mille albums prendra le temps qu'il faut, sans jamais dépasser le débit
annoncé.

Le contexte reste maître : une annulation interrompt l'attente.
*/

// Rate décrit un débit autorisé vers un service tiers.
type Rate struct {
	// Every est l'intervalle minimal entre deux requêtes.
	Every time.Duration
	// Burst est le nombre de requêtes tolérées d'un coup, avant que l'intervalle
	// ne s'applique. Une rafale courte est normale — trois recherches lancées de
	// front — et l'interdire rendrait l'interface poussive sans rien protéger.
	Burst int
}

/*
Débits par défaut vers les bases publiques.

Choisis d'après ce que ces services annoncent, pas d'après ce qu'ils tolèrent :
un service qui ne bloque pas n'est pas un service qui consent.

Ces valeurs sont volontairement prudentes. Le coût d'être trop lent est un
enrichissement qui dure plus longtemps, en tâche de fond, sans que personne
attende devant. Le coût d'être trop rapide est un blocage par adresse qui prive
tous les utilisateurs de l'instance.
*/
var (
	// RateOpenLibrary : Open Library demande explicitement de la modération sur
	// ses points d'accès de recherche.
	RateOpenLibrary = Rate{Every: 1500 * time.Millisecond, Burst: 3}
	// RateInternetArchive : même esprit, service financé par des dons.
	RateInternetArchive = Rate{Every: time.Second, Burst: 3}
	// RateGoogleBooks : quota journalier plutôt qu'un débit instantané, mais
	// espacer reste la seule façon de ne pas l'épuiser en une matinée.
	RateGoogleBooks = Rate{Every: time.Second, Burst: 5}
	/*
		RateOPDS est bien plus permissif, et pour une raison de fond : un
		catalogue OPDS appartient le plus souvent à l'utilisateur lui-même — son
		Komga, sa seconde instance. Le brider reviendrait à le brider sur son
		propre matériel.
	*/
	RateOPDS = Rate{Every: 100 * time.Millisecond, Burst: 10}
)

/*
Throttle espace les requêtes vers les services tiers, par compartiment.

Un compartiment par genre de fournisseur, pas par source : deux instances Komga
sont deux machines distinctes, mais toutes les requêtes vers Open Library
partent de la même adresse et comptent pour la même.
*/
type Throttle struct {
	mu      sync.Mutex
	rates   map[string]Rate
	buckets map[string]*bucket

	// now est remplaçable en test. Sans cela, vérifier qu'un débit est respecté
	// demanderait d'attendre réellement, et le test durerait ce qu'il mesure.
	now func() time.Time
	// sleep est remplaçable pour la même raison.
	sleep func(context.Context, time.Duration) error
}

type bucket struct {
	// tokens est fractionnaire : il se recharge continûment plutôt que par
	// paliers, ce qui évite qu'une requête arrivée juste avant un palier
	// attende un intervalle entier pour rien.
	tokens float64
	last   time.Time
}

func NewThrottle() *Throttle {
	return &Throttle{
		rates:   map[string]Rate{},
		buckets: map[string]*bucket{},
		now:     time.Now,
		sleep:   sleepCtx,
	}
}

// SetRate fixe le débit d'un compartiment.
func (t *Throttle) SetRate(kind string, rate Rate) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.rates[kind] = rate
}

/*
Wait bloque jusqu'à ce qu'une requête soit permise.

Retourne l'erreur du contexte s'il est annulé pendant l'attente — un import
abandonné ne doit pas continuer à retenir un jeton.
*/
func (t *Throttle) Wait(ctx context.Context, kind string) error {
	for {
		delay, ok := t.reserve(kind)
		if ok {
			return nil
		}
		if err := t.sleep(ctx, delay); err != nil {
			return err
		}
	}
}

/*
reserve tente de prendre un jeton, et dit sinon combien de temps attendre.

Séparée de Wait pour que toute la manipulation d'état tienne sous le verrou, et
que l'attente se fasse en dehors. Dormir en tenant le verrou bloquerait tous les
autres compartiments.
*/
func (t *Throttle) reserve(kind string) (time.Duration, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	rate, ok := t.rates[kind]
	if !ok || rate.Every <= 0 {
		// Aucun débit déclaré : rien à limiter. Un fournisseur qu'on a oublié de
		// déclarer doit fonctionner, quitte à être trop rapide — le contraire
		// bloquerait une fonctionnalité entière sur un oubli de configuration.
		return 0, true
	}
	if rate.Burst < 1 {
		rate.Burst = 1
	}

	now := t.now()
	b, exists := t.buckets[kind]
	if !exists {
		b = &bucket{tokens: float64(rate.Burst), last: now}
		t.buckets[kind] = b
	}

	// Recharge au prorata du temps écoulé.
	elapsed := now.Sub(b.last)
	if elapsed > 0 {
		b.tokens += float64(elapsed) / float64(rate.Every)
		if b.tokens > float64(rate.Burst) {
			b.tokens = float64(rate.Burst)
		}
		b.last = now
	}

	if b.tokens >= 1 {
		b.tokens--
		return 0, true
	}

	// Temps qu'il manque pour reconstituer un jeton entier.
	missing := 1 - b.tokens
	return time.Duration(missing * float64(rate.Every)), false
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
