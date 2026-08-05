package amule

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/adonko3xBitters/boxincloud/server/internal/amule/ec"
	"github.com/adonko3xBitters/boxincloud/server/internal/platform/sse"
)

/*
La scrutation du démon.

EC ne pousse rien. Pour que l'interface vive, il faut donc demander, en boucle,
et comparer ce qu'on obtient à ce qu'on avait — c'est ce que fait ce fichier.
Voir docs/adr/005-temps-reel-sse-evenements-derives.md.

Trois règles gouvernent la boucle, et chacune répond à un abus qu'on aurait
sinon commis :

  - **Rien quand personne ne regarde.** Zéro abonné au flux d'événements, zéro
    octet vers amuled. Une instance oubliée sur un NAS ne doit pas interroger
    son démon toutes les secondes pendant des mois.
  - **Une scrutation pour tout le monde.** Vingt onglets ouverts, une session EC,
    un instantané partagé par pointeur. La charge ne dépend pas du nombre de
    spectateurs.
  - **Une cadence qui suit l'activité.** La cadence de base quand quelque chose
    bouge, un relâchement progressif jusqu'à un plafond quand rien ne change,
    et retour immédiat à la cadence de base au premier changement.
*/

// Réglages par défaut de la scrutation.
const (
	// defaultPollInterval est la cadence quand quelque chose bouge. Même valeur
	// que le défaut de BOXINCLOUD_ED2K_POLL_INTERVAL : une barre qui avance par
	// paliers d'une seconde est honnête, et le démon ne le sent pas passer.
	defaultPollInterval = time.Second

	// idleFactor fixe le plafond au repos, en multiples de la cadence de base.
	// L'ADR dit « une seconde quand ça bouge, cinq au repos ».
	idleFactor = 5

	// Bornes de l'espacement des tentatives de reconnexion. Le minimum évite de
	// marteler un démon qui redémarre ; le maximum évite qu'une coupure de
	// quelques heures se solde par une reprise dix minutes après le retour.
	defaultMinReconnect = time.Second
	defaultMaxReconnect = 30 * time.Second
)

/*
collector rassemble un instantané complet depuis une session EC.

Interface, et non appel direct aux fonctions de traduction : la scrutation se
teste alors avec des instantanés écrits à la main, sans démon, sans Docker et
sans réseau. Ce qui est testé ici — cadence, arrêt, reconnexion, dérivation —
n'a rien à voir avec le décodage des trames.
*/
type collector interface {
	Collect(ctx context.Context, conn *ec.Conn) (Snapshot, error)
}

/*
dialer ouvre une session EC pour la scrutation.

Injecté plutôt qu'appelé en dur : lire l'adresse en base et desceller le mot de
passe est l'affaire du service, et la boucle n'a aucune raison de connaître ni
le dépôt ni le sceau. Un doublé rend les tests possibles sans démon.
*/
type dialer interface {
	Open(ctx context.Context) (*ec.Conn, error)
}

// dialerFunc adapte une fonction, pour que le branchement au service tienne en
// une fermeture.
type dialerFunc func(ctx context.Context) (*ec.Conn, error)

func (f dialerFunc) Open(ctx context.Context) (*ec.Conn, error) { return f(ctx) }

/*
ecCollector est le collecteur réel, celui qui interroge le démon.

À CETTE ÉTAPE il ne rapporte que l'horodatage. Les fonctions qui demandent la
file, les serveurs, l'état de connexion, les statistiques, les envois et les
fichiers partagés — requestXxx/decodeXxx — vivent dans les fichiers mapping_*.go
du même paquet et sont écrites en parallèle. Leur branchement est le geste
d'INTÉGRATION : une ligne par domaine dans Collect, et rien d'autre à changer
ici. C'est tout l'intérêt d'avoir passé par une interface.

L'aller-retour EC_OP_GET_CONNSTATE, lui, est déjà indispensable et le restera :
c'est la requête la moins coûteuse du protocole, et c'est ce qui fait CONSTATER
la perte de la session. Sans elle, la boucle tournerait indéfiniment sans jamais
s'apercevoir que le démon est parti.
*/
type ecCollector struct{}

func (ecCollector) Collect(ctx context.Context, conn *ec.Conn) (Snapshot, error) {
	if conn == nil {
		return Snapshot{}, errors.New("amule : instantané demandé sans session EC")
	}

	snapshot := Snapshot{TakenAt: time.Now()}

	if _, err := conn.Do(ctx, ec.New(ec.OpGetConnstate)); err != nil {
		return Snapshot{}, err
	}

	// Intégration : renseigner ici Connection, Stats, Downloads, Uploads,
	// QueuedPeers, Servers et SharedFiles depuis les mapping_*.go.

	return snapshot, nil
}

/*
clock isole la boucle du temps réel.

Un test de cadence adaptative ne doit pas durer le temps qu'il mesure. Avec une
horloge doublée, la vérification porte sur les délais DEMANDÉS — 1 s, 2 s, 4 s,
5 s, puis retour à 1 s — et la suite reste instantanée. Attendre réellement
n'apporterait aucune garantie de plus, et rendrait le test lent et fragile sur
une machine chargée.
*/
type clock interface {
	Now() time.Time

	// Timer arme un délai et rend le canal qui s'ouvrira à son terme, plus la
	// fonction qui libère la ressource.
	Timer(d time.Duration) (<-chan time.Time, func())
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

func (realClock) Timer(d time.Duration) (<-chan time.Time, func()) {
	timer := time.NewTimer(d)
	return timer.C, func() { timer.Stop() }
}

// pollerOptions règle la boucle. Le zéro est utilisable : tous les champs ont un
// défaut.
type pollerOptions struct {
	// Interval est la cadence de base, celle qu'on tient quand ça bouge.
	Interval time.Duration

	// IdleInterval est le plafond du relâchement au repos. Zéro prend
	// idleFactor × Interval.
	IdleInterval time.Duration

	// MinReconnect et MaxReconnect bornent l'espacement des tentatives de
	// reconnexion.
	MinReconnect time.Duration
	MaxReconnect time.Duration
}

func (o pollerOptions) normalized() pollerOptions {
	if o.Interval <= 0 {
		o.Interval = defaultPollInterval
	}
	if o.IdleInterval < o.Interval {
		o.IdleInterval = idleFactor * o.Interval
	}
	if o.MinReconnect <= 0 {
		o.MinReconnect = defaultMinReconnect
	}
	if o.MaxReconnect < o.MinReconnect {
		o.MaxReconnect = max(defaultMaxReconnect, o.MinReconnect)
	}
	return o
}

/*
poller tient la session EC, produit les instantanés et publie les événements.

Un seul par instance : c'est le point où « une scrutation pour tout le monde »
devient vrai.
*/
type poller struct {
	dial    dialer
	collect collector
	hub     *sse.Hub
	log     *slog.Logger
	clock   clock
	opts    pollerOptions

	/*
		onState est prévenu de chaque changement d'état de la session.

		C'est ce qui permet au service de persister le dernier état constaté
		sans que la boucle connaisse le dépôt. À renseigner AVANT Start : la
		valeur est lue par la goroutine de scrutation.
	*/
	onState func(state State, detail string)

	/*
		onSnapshot est appelé après chaque instantané, avec le pointeur partagé.

		C'est par là que l'état complet redescend vers le navigateur — les
		événements disent ce qui a changé, l'instantané dit où on en est. La
		mise en forme (le DTO du contrat d'API) appartient à la couche HTTP, pas
		ici : la boucle ne publie donc pas l'instantané elle-même.

		Le pointeur reçu désigne une valeur IMMUABLE. Le rappel ne doit pas
		l'écrire, et n'a pas besoin de le copier.
	*/
	onSnapshot func(snapshot *Snapshot)

	// watchers est le nombre d'abonnés au flux, tenu à jour par le rappel du
	// concentrateur. Atomique : ce rappel est appelé en tenant le verrou du
	// concentrateur et ne doit rien bloquer.
	watchers atomic.Int64

	// wake réveille la boucle quand le nombre d'abonnés change. Un canal, parce
	// que le rappel ne peut ni bloquer ni rappeler le concentrateur.
	wake chan struct{}

	mu       sync.Mutex
	snapshot *Snapshot
	state    State

	started   atomic.Bool
	startOnce sync.Once
	stopOnce  sync.Once
	cancel    context.CancelFunc
	done      chan struct{}
}

func newPoller(dial dialer, collect collector, hub *sse.Hub, opts pollerOptions, log *slog.Logger) *poller {
	return &poller{
		dial:    dial,
		collect: collect,
		hub:     hub,
		log:     log,
		clock:   realClock{},
		opts:    opts.normalized(),
		// Tampon de 1 : le rappel du concentrateur dépose son signal et rend la
		// main immédiatement, même si la boucle est occupée ailleurs.
		wake: make(chan struct{}, 1),

		// État de départ vide, et non StateDisconnected : la PREMIÈRE
		// observation doit être annoncée, y compris un échec. Partir de
		// « déconnecté » ferait passer sous silence un démon injoignable au
		// démarrage — l'état n'aurait pas changé, donc rien ne serait publié ni
		// persisté.
		state: "",

		done: make(chan struct{}),
	}
}

/*
Start lance la scrutation et la branche sur le concentrateur.

Ne connecte rien tout de suite : tant qu'aucun navigateur n'est abonné, la
boucle attend, et aucun octet ne part vers le démon.
*/
func (p *poller) Start() {
	p.startOnce.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		p.cancel = cancel
		p.started.Store(true)

		if p.hub != nil {
			p.hub.OnChange(p.subscribersChanged)
			// Des abonnés ont pu arriver avant nous — au redémarrage d'un
			// service reconstruit à chaud, par exemple.
			p.subscribersChanged(p.hub.Subscribers())
		}

		go p.run(ctx)
	})
}

// Stop arrête la scrutation et attend que la session soit refermée.
//
// Idempotent, et sans effet sur un poller jamais démarré : l'arrêt d'un module
// désactivé ne doit pas bloquer l'arrêt du serveur.
func (p *poller) Stop() {
	p.stopOnce.Do(func() {
		if !p.started.Load() {
			return
		}
		p.cancel()
		<-p.done
	})
}

/*
Current rend le dernier instantané connu, ou nil si aucun n'a encore été pris.

Sans copie : Snapshot est immuable une fois construit, et vingt lecteurs
partagent donc le même pointeur pour le prix d'un verrou tenu le temps d'une
lecture de champ. Le lecteur ne doit rien y écrire.

Le pointeur peut être vieux — plus personne ne regardait, la boucle dormait.
TakenAt le dit, et c'est à l'interface d'en tirer les conséquences plutôt qu'à
la boucle de mentir en effaçant l'état.
*/
func (p *poller) Current() *Snapshot {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.snapshot
}

/*
subscribersChanged est le rappel du concentrateur.

APPELÉ EN TENANT LE VERROU DU CONCENTRATEUR. Il doit donc être court, ne jamais
rappeler le concentrateur, et ne jamais bloquer — d'où le dépôt non bloquant
d'un signal dans un canal tamponné plutôt qu'un réveil direct.
*/
func (p *poller) subscribersChanged(count int) {
	p.watchers.Store(int64(count))

	select {
	case p.wake <- struct{}{}:
	default:
		// Un signal est déjà en attente : la boucle le prendra et lira le
		// compte à jour. Deux réveils pour deux changements successifs ne
		// valent pas mieux qu'un seul.
	}
}

/*
run est la boucle de supervision : elle ouvre une session, scrute tant qu'elle
tient, et recommence.

Trois façons de sortir de la scrutation, et elles ne se traitent pas pareil :
le contexte annulé arrête tout, la session perdue déclenche une reconnexion
espacée, et le départ du dernier abonné rend simplement la main à l'attente.
*/
func (p *poller) run(ctx context.Context) {
	defer close(p.done)

	delay := p.opts.MinReconnect

	for {
		if !p.waitForWatchers(ctx) {
			return
		}

		conn, err := p.dial.Open(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}

			p.setState(StateDisconnected, err.Error())
			p.log.Warn("session EC impossible, nouvelle tentative",
				slog.Duration("dans", delay), slog.Any("err", err))

			if !p.wait(ctx, delay) {
				return
			}
			delay = min(2*delay, p.opts.MaxReconnect)
			continue
		}

		// La session tient : l'espacement repart de zéro, sans quoi une coupure
		// ancienne pénaliserait encore la prochaine.
		delay = p.opts.MinReconnect
		p.setState(StateConnected, "session External Connections ouverte")

		err = p.pollLoop(ctx, conn)
		closeSession(conn)

		switch {
		case ctx.Err() != nil:
			p.setState(StateDisconnected, "scrutation arrêtée")
			return

		case err != nil:
			p.setState(StateDisconnected, err.Error())
			p.log.Warn("session EC perdue", slog.Any("err", err))
			if !p.wait(ctx, delay) {
				return
			}
			delay = min(2*delay, p.opts.MaxReconnect)

		default:
			// Plus personne ne regarde : on referme et on attend. C'est le seul
			// cas où l'on se déconnecte sans que rien n'ait échoué.
			p.setState(StateDisconnected, "aucun abonné — scrutation suspendue")
		}
	}
}

/*
pollLoop scrute tant que la session tient et que quelqu'un regarde.

Rend nil quand le dernier abonné est parti, une erreur quand la session est
perdue.

`previous` est LOCAL à la session, et c'est un choix : à chaque reprise — après
une coupure comme après une nuit sans spectateur — la première comparaison n'a
pas de précédent et ne produit donc aucun événement. Inventer un « démarré » ou
un « terminé » pour des changements survenus pendant qu'on ne regardait pas
serait daté de maintenant, donc faux, et noierait le premier écran d'un abonné
qui vient d'arriver sous des notifications qui ne le concernent pas. L'état
réel, lui, lui parvient entier par l'instantané.
*/
func (p *poller) pollLoop(ctx context.Context, conn *ec.Conn) error {
	var previous *Snapshot
	interval := p.opts.Interval

	for {
		if p.watchers.Load() == 0 {
			return nil
		}

		snapshot, err := p.collect.Collect(ctx, conn)
		if err != nil {
			// Un contexte annulé n'est pas une panne : c'est l'arrêt demandé,
			// et le remonter comme erreur ferait journaliser un échec à chaque
			// extinction du serveur.
			if ctx.Err() != nil {
				return nil //nolint:nilerr // arrêt demandé, pas un échec de collecte
			}
			return err
		}

		current := &snapshot
		p.store(current)

		events := diff(previous, current)
		first := previous == nil
		previous = current

		if p.onSnapshot != nil {
			p.onSnapshot(current)
		}
		for _, event := range events {
			p.publish(event)
		}

		/*
			La cadence suit l'activité.

			Le premier instantané d'une session compte comme un changement bien
			qu'il ne produise aucun événement : on vient de se connecter, c'est
			le moment où l'on veut voir bouger, pas celui où l'on se relâche.
		*/
		if first || len(events) > 0 {
			interval = p.opts.Interval
		} else {
			interval = min(2*interval, p.opts.IdleInterval)
		}

		if !p.wait(ctx, interval) {
			return nil
		}
	}
}

// waitForWatchers bloque tant que personne ne regarde. Rend false si le
// contexte tombe.
//
// C'est ici que se tient la promesse de l'ADR : une instance que personne
// n'observe ne produit aucun trafic EC. Pas une requête moins fréquente : pas
// de requête du tout, et pas même une session ouverte.
func (p *poller) waitForWatchers(ctx context.Context) bool {
	for {
		if p.watchers.Load() > 0 {
			return true
		}

		select {
		case <-ctx.Done():
			return false
		case <-p.wake:
		}
	}
}

/*
wait attend au plus d, et rend false si le contexte tombe.

Rend la main plus tôt si le nombre d'abonnés change. Dans les deux sens, c'est
ce qu'on veut : le dernier abonné part, la session se referme sans attendre la
fin du cycle en cours ; un onglet s'ouvre, il ne patiente pas cinq secondes
devant une file au repos pour voir son premier rafraîchissement.
*/
func (p *poller) wait(ctx context.Context, d time.Duration) bool {
	tick, release := p.clock.Timer(d)
	defer release()

	select {
	case <-ctx.Done():
		return false
	case <-p.wake:
		return true
	case <-tick:
		return true
	}
}

// store publie le nouvel instantané pour les lecteurs. Verrou tenu le temps
// d'une affectation de pointeur : c'est tout ce que coûte le partage.
func (p *poller) store(snapshot *Snapshot) {
	p.mu.Lock()
	p.snapshot = snapshot
	p.mu.Unlock()
}

/*
setState note l'état de la session et n'annonce que les changements.

Le détail seul ne rejoue pas l'annonce : dix tentatives de reconnexion qui
échouent pour la même raison sont un seul fait, et en faire dix événements
ferait clignoter l'interface pendant toute la panne.
*/
func (p *poller) setState(state State, detail string) {
	p.mu.Lock()
	changed := p.state != state
	p.state = state
	p.mu.Unlock()

	if !changed {
		return
	}

	kind := EventDaemonDisconnected
	if state == StateConnected {
		kind = EventDaemonConnected
	}
	p.publish(Event{Kind: kind, Detail: detail, At: p.clock.Now()})

	if p.onState != nil {
		p.onState(state, detail)
	}
}

func (p *poller) publish(event Event) {
	if p.hub == nil {
		return
	}
	p.hub.Publish(string(event.Kind), event)
}

// closeSession referme une session, en tolérant l'absence de session.
//
// Un dialer peut rendre nil sans erreur — le doublé des tests le fait, n'ayant
// aucune connexion à donner — et fermer un pointeur nul paniquerait.
func closeSession(conn *ec.Conn) {
	if conn == nil {
		return
	}
	_ = conn.Close()
}
