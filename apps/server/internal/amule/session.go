package amule

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"sync"

	"github.com/adonko3xBitters/boxincloud/server/internal/amule/ec"
)

/*
Le branchement de la scrutation au service.

Le service sait lire la configuration du démon et desceller son mot de passe ;
la scrutation sait tenir une session et produire des instantanés. Ni l'un ni
l'autre n'a de raison de connaître le métier de l'autre — ce fichier est la
couture, et il tient en une poignée de fonctions.

# Pourquoi le démarrage ne joint personne

Start ne se connecte pas. Il arme la boucle, qui attend un abonné au flux
d'événements avant d'ouvrir quoi que ce soit. Une instance dont l'interface
n'est jamais ouverte ne parle donc jamais au démon, et c'est exactement ce que
promet l'ADR-005.

Conséquence à assumer : après un démarrage du serveur, `Snapshot()` rend nil
tant que personne n'a regardé. Ce n'est pas une panne, et les handlers le
disent plutôt que d'inventer un état vide.
*/

// serviceDialer ouvre une session EC à partir de la configuration enregistrée.
//
// Relit la configuration à CHAQUE ouverture, délibérément : un administrateur
// qui corrige l'adresse du démon doit voir la reconnexion suivante en tenir
// compte, sans redémarrer le serveur.
type serviceDialer struct {
	svc *Service
}

func (d serviceDialer) Open(ctx context.Context) (*ec.Conn, error) {
	stored, err := d.svc.repo.GetDaemon(ctx)
	if errors.Is(err, ErrNoDaemonRow) {
		return nil, ErrNotConfigured
	}
	if err != nil {
		return nil, err
	}

	password, err := d.svc.sealer.Open(stored.PasswordEnc)
	if err != nil {
		return nil, fmt.Errorf(
			"mot de passe EC indéchiffrable — BOXINCLOUD_SECRET_KEY a-t-elle changé ? : %w", err)
	}

	return ec.Dial(ctx,
		net.JoinHostPort(stored.Host, strconv.Itoa(stored.Port)),
		string(password),
		ec.Options{ClientName: "boxincloud", ClientVersion: d.svc.opts.Version},
	)
}

// sessionState porte ce que la scrutation ajoute au service.
type sessionState struct {
	mu     sync.Mutex
	poller *poller
}

/*
Start arme la scrutation. Sans effet si le module est désactivé.

Appelé au démarrage du serveur, et idempotent : deux appels ne produisent pas
deux boucles. La garantie compte, parce qu'une seconde boucle ouvrirait une
seconde session EC et doublerait discrètement la charge sur le démon.
*/
func (s *Service) Start() {
	if !s.opts.Enabled {
		return
	}

	s.session.mu.Lock()
	defer s.session.mu.Unlock()

	if s.session.poller != nil {
		return
	}

	p := newPoller(
		serviceDialer{svc: s},
		ecCollector{},
		s.hub,
		pollerOptions{Interval: s.opts.PollInterval},
		s.log,
	)

	/*
		L'état constaté par la boucle est persisté par le service.

		C'est ce qui permet à l'interface de dire, après un redémarrage, « la
		dernière connexion a échoué, voici pourquoi » au lieu d'afficher un état
		neutre qui laisserait croire que rien n'a jamais été tenté.

		Le contexte est détaché : ce rappel vient de la boucle, dont le contexte
		est celui de la scrutation. L'utiliser ferait échouer l'écriture au
		moment précis où elle est la plus utile — l'extinction.
	*/
	p.onState = func(state State, detail string) {
		ctx := context.Background()
		if err := s.repo.SetState(ctx, state, detail, state == StateConnected); err != nil {
			s.log.Warn("état du démon non persisté", slog.Any("err", err))
		}
	}

	/*
		Après chaque instantané, le pont regarde ce qui est terminé.

		Branché ICI plutôt que dans la boucle : la scrutation n'a pas à savoir
		qu'une bibliothèque existe, et le pont n'a pas à savoir qu'il y a une
		boucle. Le contexte est détaché — publier un fichier de plusieurs
		gigaoctets peut durer bien plus qu'un tour de scrutation, et le lier au
		tour l'interromprait à mi-course.
	*/
	p.onSnapshot = func(snapshot *Snapshot) {
		go s.publishCompleted(context.Background(), snapshot)
	}

	s.session.poller = p
	p.Start()

	s.log.Info("scrutation du démon aMule armée",
		slog.Duration("interval", s.opts.PollInterval))
}

// Stop arrête la scrutation et ferme la session.
//
// Idempotent, et sûr sur un service jamais démarré : l'arrêt du serveur ne doit
// pas avoir à savoir si le module avait été activé.
func (s *Service) Stop() {
	s.session.mu.Lock()
	p := s.session.poller
	s.session.poller = nil
	s.session.mu.Unlock()

	if p != nil {
		p.Stop()
	}
}

/*
Snapshot rend le dernier état connu du démon, ou nil.

Nil a un sens précis et n'est pas une erreur : personne n'a encore regardé, donc
rien n'a encore été demandé au démon. Les handlers traduisent cela en « pas
encore d'instantané », ce qui est exact, plutôt qu'en file vide, ce qui serait
faux.
*/
func (s *Service) Snapshot() *Snapshot {
	s.session.mu.Lock()
	p := s.session.poller
	s.session.mu.Unlock()

	if p == nil {
		return nil
	}
	return p.Current()
}

/*
Sources demande les sources d'un fichier, hors instantané.

Elles n'y figurent pas, et c'est délibéré : une file de cent fichiers demanderait
cent requêtes supplémentaires à chaque tour de scrutation, pour des données que
l'interface n'affiche que sur le fichier ouvert. On les demande donc à la
demande, sur une session neuve.

Le coût — une connexion et une authentification par consultation — est payé par
un geste explicite de l'utilisateur, pas par la boucle de fond.
*/
func (s *Service) Sources(ctx context.Context, hash string) ([]Source, error) {
	if !s.opts.Enabled {
		return nil, ErrDisabled
	}

	conn, err := serviceDialer{svc: s}.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	resp, err := conn.Do(ctx, requestSources(hash))
	if err != nil {
		return nil, err
	}
	return decodeSources(resp)
}
