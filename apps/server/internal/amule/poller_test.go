package amule

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/adonko3xBitters/boxincloud/server/internal/amule/ec"
	"github.com/adonko3xBitters/boxincloud/server/internal/platform/sse"
)

/*
Les tests de la scrutation.

Aucun démon, aucun conteneur, aucun réseau : ce qui est vérifié ici — cadence,
arrêt, reconnexion, partage de l'instantané — ne dépend pas du décodage des
trames EC. C'est précisément ce que l'interface `collector` achète.

# Comment la cadence se teste sans attendre

Une horloge doublée. `Timer` ENREGISTRE le délai demandé puis rend un canal déjà
armé : la boucle croit avoir dormi cinq secondes, le test dure une
microseconde, et l'assertion porte sur la séquence des délais — 1, 2, 4, 5,
puis retour à 1 dès qu'un événement apparaît. Dormir réellement n'apporterait
aucune garantie supplémentaire et rendrait la suite lente et intermittente sur
une machine chargée.

Les tests qui observent l'ARRÊT de la boucle, eux, ont besoin que le temps passe
vraiment : ils utilisent `tickClock`, qui dort une milliseconde quoi qu'on lui
demande.
*/

// Contrats vérifiés à la compilation : les implémentations réelles doivent
// rester assignables aux interfaces que la boucle consomme.
var (
	_ collector = ecCollector{}
	_ dialer    = dialerFunc(nil)
	_ clock     = realClock{}
)

// fakeClock n'avance que sur demande, et retient ce qu'on lui a demandé.
type fakeClock struct {
	mu     sync.Mutex
	now    time.Time
	delays []time.Duration
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Timer(d time.Duration) (<-chan time.Time, func()) {
	c.mu.Lock()
	c.delays = append(c.delays, d)
	c.now = c.now.Add(d)
	fired := c.now
	c.mu.Unlock()

	// Canal déjà armé : la boucle repart immédiatement.
	ch := make(chan time.Time, 1)
	ch <- fired
	return ch, func() {}
}

func (c *fakeClock) recorded() []time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]time.Duration(nil), c.delays...)
}

// tickClock laisse le temps passer, mais très peu : pour les tests qui
// observent l'arrêt de la boucle plutôt que sa cadence.
type tickClock struct{ every time.Duration }

func (tickClock) Now() time.Time { return time.Now() }

func (c tickClock) Timer(time.Duration) (<-chan time.Time, func()) {
	timer := time.NewTimer(c.every)
	return timer.C, func() { timer.Stop() }
}

// collectorFunc adapte une fonction en collecteur.
type collectorFunc func(ctx context.Context, conn *ec.Conn) (Snapshot, error)

func (f collectorFunc) Collect(ctx context.Context, conn *ec.Conn) (Snapshot, error) {
	return f(ctx, conn)
}

/*
scriptedCollector rend des instantanés écrits à la main, puis se met en attente.

L'attente finale est ce qui rend les tests de cadence déterministes : une fois
le script épuisé, la boucle ne demande plus aucun délai, et la séquence observée
est donc finie et reproductible.
*/
type scriptedCollector struct {
	snapshots []Snapshot

	mu    sync.Mutex
	calls int

	done sync.Once
	end  chan struct{}
}

func newScript(snapshots ...Snapshot) *scriptedCollector {
	return &scriptedCollector{snapshots: snapshots, end: make(chan struct{})}
}

func (s *scriptedCollector) Collect(ctx context.Context, _ *ec.Conn) (Snapshot, error) {
	s.mu.Lock()
	index := s.calls
	s.calls++
	s.mu.Unlock()

	if index >= len(s.snapshots) {
		s.done.Do(func() { close(s.end) })
		<-ctx.Done()
		return Snapshot{}, ctx.Err()
	}
	return s.snapshots[index], nil
}

func newTestPoller(t *testing.T, dial dialer, collect collector, opts pollerOptions) *poller {
	t.Helper()

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	p := newPoller(dial, collect, nil, opts, log)
	t.Cleanup(p.Stop)
	return p
}

// sansDemon rend un dialer qui « réussit » sans rien ouvrir : les doublés de
// collecteur n'ont besoin d'aucune session.
func sansDemon(opens *atomic.Int64) dialer {
	return dialerFunc(func(context.Context) (*ec.Conn, error) {
		opens.Add(1)
		return nil, nil
	})
}

func attend(t *testing.T, ch <-chan struct{}, quoi string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatalf("délai dépassé en attendant %s", quoi)
	}
}

/*
TestScrutationNeToucheAuDemonQueSiQuelquunRegarde.

La promesse centrale de l'ADR : une instance que personne n'observe ne produit
AUCUN trafic EC. Pas des requêtes plus espacées — pas de requête du tout, et pas
même une session ouverte.
*/
func TestScrutationNeToucheAuDemonQueSiQuelquunRegarde(t *testing.T) {
	var opens atomic.Int64
	script := newScript()

	p := newTestPoller(t, sansDemon(&opens), script, pollerOptions{})
	p.clock = tickClock{every: time.Millisecond}
	p.Start()

	// Plusieurs réveils sans abonné : la boucle doit les consommer et se
	// rendormir, pas en profiter pour aller voir le démon.
	for range 3 {
		p.subscribersChanged(0)
	}
	time.Sleep(20 * time.Millisecond)

	if got := opens.Load(); got != 0 {
		t.Fatalf("%d session(s) ouverte(s) sans abonné — la scrutation doit être muette", got)
	}

	p.subscribersChanged(1)
	attend(t, script.end, "la première collecte")

	if got := opens.Load(); got != 1 {
		t.Errorf("sessions ouvertes = %d, attendu 1 pour un abonné", got)
	}
}

/*
TestScrutationSarreteQuandLeDernierAbonnePart.

Et s'arrête vraiment : la session est refermée, pas seulement mise en veille.
C'est ce que prouve la seconde ouverture au retour d'un abonné — une boucle qui
aurait simplement ralenti n'en aurait pas eu besoin.
*/
func TestScrutationSarreteQuandLeDernierAbonnePart(t *testing.T) {
	var opens, collects atomic.Int64
	premiere := make(chan struct{})
	var once sync.Once

	collect := collectorFunc(func(context.Context, *ec.Conn) (Snapshot, error) {
		collects.Add(1)
		once.Do(func() { close(premiere) })
		return Snapshot{TakenAt: time.Now()}, nil
	})

	p := newTestPoller(t, sansDemon(&opens), collect, pollerOptions{})
	p.clock = tickClock{every: time.Millisecond}

	p.subscribersChanged(1)
	p.Start()
	attend(t, premiere, "la première collecte")

	p.subscribersChanged(0)

	// La boucle peut être en train de finir un cycle : on laisse passer, puis
	// on vérifie que le compteur ne bouge plus.
	time.Sleep(30 * time.Millisecond)
	fige := collects.Load()
	time.Sleep(30 * time.Millisecond)

	if repris := collects.Load(); repris != fige {
		t.Errorf("%d collectes après le départ du dernier abonné — la boucle continue",
			repris-fige)
	}
	if got := opens.Load(); got != 1 {
		t.Fatalf("sessions ouvertes = %d, attendu 1", got)
	}

	p.subscribersChanged(1)
	deadline := time.Now().Add(2 * time.Second)
	for opens.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if got := opens.Load(); got != 2 {
		t.Errorf("sessions ouvertes = %d au retour d'un abonné, attendu 2 — la session "+
			"précédente n'a donc pas été refermée", got)
	}
}

/*
TestCadenceSeRelacheAuReposEtRevientAuChangement.

L'assertion porte sur les délais DEMANDÉS, pas sur le temps écoulé : le test est
instantané et ne dépend pas de la charge de la machine.

La séquence attendue dit tout : le premier instantané d'une session tient la
cadence de base bien qu'il ne produise aucun événement — on vient de se
connecter, c'est le moment où l'on veut voir bouger — puis le relâchement
double jusqu'au plafond, et un seul événement suffit à tout ramener à la
cadence de base.
*/
func TestCadenceSeRelacheAuReposEtRevientAuChangement(t *testing.T) {
	repos := Snapshot{TakenAt: quand}
	bouge := Snapshot{
		TakenAt:   quand.Add(time.Second),
		Downloads: []Download{fichier("a", DownloadDownloading)},
	}

	script := newScript(repos, repos, repos, repos, repos, bouge)

	var opens atomic.Int64
	p := newTestPoller(t, sansDemon(&opens), script, pollerOptions{
		Interval:     100 * time.Millisecond,
		IdleInterval: 500 * time.Millisecond,
	})
	horloge := &fakeClock{now: quand}
	p.clock = horloge

	p.subscribersChanged(1)
	p.Start()
	attend(t, script.end, "l'épuisement du script")
	p.Stop()

	attendus := []time.Duration{
		100 * time.Millisecond, // premier instantané : cadence de base
		200 * time.Millisecond, // rien n'a changé
		400 * time.Millisecond,
		500 * time.Millisecond, // plafond
		500 * time.Millisecond, // et pas au-delà
		100 * time.Millisecond, // un événement : retour immédiat à la base
	}
	verifieDelais(t, horloge.recorded(), attendus)
}

/*
TestReconnexionEspaceLesTentatives.

Un démon peut disparaître — mise à jour, redémarrage du NAS, conteneur
recréé. Marteler sa porte toutes les secondes pendant des heures ne le fait pas
revenir plus vite ; attendre dix minutes après son retour est tout aussi
mauvais. D'où un espacement qui double, mais borné.
*/
func TestReconnexionEspaceLesTentatives(t *testing.T) {
	const tentatives = 5

	var attempts atomic.Int64
	epuise := make(chan struct{})
	var once sync.Once

	dial := dialerFunc(func(ctx context.Context) (*ec.Conn, error) {
		if attempts.Add(1) > tentatives {
			once.Do(func() { close(epuise) })
			<-ctx.Done()
			return nil, ctx.Err()
		}
		return nil, errors.New("démon injoignable")
	})

	var etats []State
	var mu sync.Mutex

	p := newTestPoller(t, dial, newScript(), pollerOptions{
		Interval:     time.Second,
		MinReconnect: 10 * time.Millisecond,
		MaxReconnect: 40 * time.Millisecond,
	})
	horloge := &fakeClock{now: quand}
	p.clock = horloge
	p.onState = func(state State, _ string) {
		mu.Lock()
		etats = append(etats, state)
		mu.Unlock()
	}

	p.subscribersChanged(1)
	p.Start()
	attend(t, epuise, "les tentatives de reconnexion")
	p.Stop()

	// Une attente par tentative échouée : la sixième est celle qui bloque, et
	// n'a donc pas encore demandé de délai.
	verifieDelais(t, horloge.recorded(), []time.Duration{
		10 * time.Millisecond,
		20 * time.Millisecond,
		40 * time.Millisecond, // plafond
		40 * time.Millisecond,
		40 * time.Millisecond,
	})

	// Cinq échecs pour la même raison sont UN fait : les réannoncer ferait
	// clignoter l'interface pendant toute la panne.
	mu.Lock()
	defer mu.Unlock()
	if len(etats) != 1 || etats[0] != StateDisconnected {
		t.Errorf("états publiés = %v, attendu un seul %q", etats, StateDisconnected)
	}
}

/*
TestEtatDeSessionSuitLaVieDeLaConnexion.

L'interface doit pouvoir dire « démon joint » puis « démon perdu ». Sans cette
remontée, une coupure se traduirait par des chiffres qui cessent simplement de
bouger — ce qu'aucun utilisateur ne distingue d'une file au repos.
*/
func TestEtatDeSessionSuitLaVieDeLaConnexion(t *testing.T) {
	var etats []State
	var mu sync.Mutex

	script := newScript(Snapshot{TakenAt: quand})
	var opens atomic.Int64

	p := newTestPoller(t, sansDemon(&opens), script, pollerOptions{})
	p.clock = &fakeClock{now: quand}
	p.onState = func(state State, detail string) {
		if detail == "" {
			t.Error("un changement d'état doit dire pourquoi il a eu lieu")
		}
		mu.Lock()
		etats = append(etats, state)
		mu.Unlock()
	}

	p.subscribersChanged(1)
	p.Start()
	attend(t, script.end, "la première collecte")
	p.Stop()

	mu.Lock()
	defer mu.Unlock()
	if len(etats) != 2 || etats[0] != StateConnected || etats[1] != StateDisconnected {
		t.Errorf("états publiés = %v, attendu [%s %s]", etats, StateConnected, StateDisconnected)
	}
}

/*
TestInstantanePartageLeMemePointeur.

Snapshot est immuable une fois construit : vingt onglets partagent le même
pointeur pour le prix d'un verrou tenu le temps d'une lecture de champ. Si un
jour Current copiait, la lecture deviendrait proportionnelle à la taille de la
file — pour rien.
*/
func TestInstantanePartageLeMemePointeur(t *testing.T) {
	script := newScript(Snapshot{TakenAt: quand})
	var opens atomic.Int64

	var recu *Snapshot
	var mu sync.Mutex

	p := newTestPoller(t, sansDemon(&opens), script, pollerOptions{})
	p.clock = &fakeClock{now: quand}
	p.onSnapshot = func(snapshot *Snapshot) {
		mu.Lock()
		recu = snapshot
		mu.Unlock()
	}

	if p.Current() != nil {
		t.Error("aucun instantané n'a encore été pris, Current doit rendre nil")
	}

	p.subscribersChanged(1)
	p.Start()
	attend(t, script.end, "la première collecte")
	p.Stop()

	current := p.Current()
	if current == nil {
		t.Fatal("Current rend nil après une collecte réussie")
	}
	if current != p.Current() {
		t.Error("deux lectures rendent deux pointeurs : l'instantané est copié")
	}
	if !current.TakenAt.Equal(quand) {
		t.Errorf("TakenAt = %v, attendu %v", current.TakenAt, quand)
	}

	mu.Lock()
	defer mu.Unlock()
	if recu != current {
		t.Error("le rappel d'instantané ne reçoit pas le pointeur partagé")
	}
}

/*
TestScrutationSuitLesAbonnesDuConcentrateur branche la boucle sur un vrai
concentrateur.

Les autres tests pilotent le compteur d'abonnés à la main. Celui-ci vérifie le
raccordement lui-même — que `OnChange` est bien posé, et qu'un abonné SSE réel
suffit à réveiller la scrutation puis à l'endormir en partant.
*/
func TestScrutationSuitLesAbonnesDuConcentrateur(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	hub := sse.NewHub(log, sse.Options{})

	var opens, collects atomic.Int64
	premiere := make(chan struct{})
	var once sync.Once

	collect := collectorFunc(func(context.Context, *ec.Conn) (Snapshot, error) {
		collects.Add(1)
		once.Do(func() { close(premiere) })
		return Snapshot{TakenAt: time.Now()}, nil
	})

	p := newPoller(sansDemon(&opens), collect, hub, pollerOptions{}, log)
	p.clock = tickClock{every: time.Millisecond}
	t.Cleanup(p.Stop)
	p.Start()

	time.Sleep(20 * time.Millisecond)
	if got := opens.Load(); got != 0 {
		t.Fatalf("%d session(s) ouverte(s) alors qu'aucun navigateur n'est abonné", got)
	}

	ctx, cancel := context.WithCancel(context.Background())
	request := httptest.NewRequest(http.MethodGet, "/ed2k/flux", nil).WithContext(ctx)
	recorder := httptest.NewRecorder()

	servi := make(chan struct{})
	go func() {
		defer close(servi)
		hub.Serve(recorder, request, EventStatus, Status{State: StateDisconnected})
	}()

	attend(t, premiere, "la première collecte déclenchée par un abonné")

	cancel()
	attend(t, servi, "la fin du flux SSE")

	time.Sleep(30 * time.Millisecond)
	fige := collects.Load()
	time.Sleep(30 * time.Millisecond)

	if repris := collects.Load(); repris != fige {
		t.Errorf("%d collectes après le départ du dernier navigateur", repris-fige)
	}
}

// TestStopEstIdempotentEtSansEffetSurUnPollerJamaisDemarre : l'arrêt d'un module
// désactivé ne doit pas bloquer l'arrêt du serveur.
func TestStopEstIdempotentEtSansEffetSurUnPollerJamaisDemarre(t *testing.T) {
	var opens atomic.Int64
	p := newTestPoller(t, sansDemon(&opens), newScript(), pollerOptions{})

	fini := make(chan struct{})
	go func() {
		defer close(fini)
		p.Stop()
		p.Stop()
	}()
	attend(t, fini, "l'arrêt d'un poller jamais démarré")
}

/*
TestCollecteurECRefuseUneSessionAbsente.

Le collecteur réel ne collecte encore que l'horodatage — les traductions
arrivent avec les mapping_*.go — mais il refuse déjà de faire semblant : sans
session, il n'y a pas d'instantané, et rendre un instantané vide ferait dériver
des événements « tout a disparu ».
*/
func TestCollecteurECRefuseUneSessionAbsente(t *testing.T) {
	if _, err := (ecCollector{}).Collect(context.Background(), nil); err == nil {
		t.Error("une collecte sans session EC doit échouer")
	}
}

// TestOptionsDeScrutationOntUnDefautUtilisable : un zéro ne doit pas produire
// une boucle qui tourne à vide sans jamais attendre.
func TestOptionsDeScrutationOntUnDefautUtilisable(t *testing.T) {
	opts := pollerOptions{}.normalized()

	if opts.Interval != defaultPollInterval {
		t.Errorf("Interval = %v, attendu %v", opts.Interval, defaultPollInterval)
	}
	if opts.IdleInterval != idleFactor*opts.Interval {
		t.Errorf("IdleInterval = %v, attendu %v", opts.IdleInterval, idleFactor*opts.Interval)
	}
	if opts.MinReconnect <= 0 || opts.MaxReconnect < opts.MinReconnect {
		t.Errorf("bornes de reconnexion incohérentes : %v..%v", opts.MinReconnect, opts.MaxReconnect)
	}

	// Un plafond de repos annoncé plus court que la cadence de base est une
	// erreur de configuration, pas une instruction à suivre.
	serre := pollerOptions{Interval: time.Second, IdleInterval: time.Millisecond}.normalized()
	if serre.IdleInterval < serre.Interval {
		t.Errorf("IdleInterval = %v, il ne peut pas être plus court que la cadence de base",
			serre.IdleInterval)
	}
}

func verifieDelais(t *testing.T, got, attendus []time.Duration) {
	t.Helper()

	if len(got) != len(attendus) {
		t.Fatalf("délais demandés = %v, attendu %v", got, attendus)
	}
	for i := range got {
		if got[i] != attendus[i] {
			t.Fatalf("délais demandés = %v, attendu %v", got, attendus)
		}
	}
}
