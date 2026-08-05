package amule

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"strings"

	"github.com/adonko3xBitters/boxincloud/server/internal/platform/crypto"
	"github.com/adonko3xBitters/boxincloud/server/internal/platform/netguard"
	"github.com/adonko3xBitters/boxincloud/server/internal/platform/sse"
)

// Options porte ce que le déploiement décide, par opposition à ce qui
// s'administre depuis l'interface.
type Options struct {
	Enabled     bool
	IncomingDir string

	// Version est annoncée au démon à l'authentification et apparaît dans son
	// journal, à la ligne « Connecting client ». C'est ce qui rend une instance
	// identifiable dans les journaux d'un démon qui en accueille plusieurs.
	Version string
}

/*
Service est la façade du module. C'est le seul type que le reste du serveur voit.

Il existe même quand le module est désactivé : il sait alors le dire, ce qui
vaut mieux qu'une dépendance nulle que chaque appelant devrait tester. C'est
aussi ce qui permet au routeur de garder sa garantie — une dépendance oubliée
fait échouer la construction, pas la requête.
*/
type Service struct {
	repo   Repository
	sealer *crypto.Sealer
	hub    *sse.Hub
	log    *slog.Logger
	opts   Options
}

func NewService(repo Repository, sealer *crypto.Sealer, hub *sse.Hub, opts Options, log *slog.Logger) *Service {
	return &Service{repo: repo, sealer: sealer, hub: hub, opts: opts, log: log}
}

// Enabled indique si le module est actif dans la configuration.
func (s *Service) Enabled() bool { return s.opts.Enabled }

/*
SetVersion renseigne la version annoncée au démon.

Par un setter plutôt que par les options : la version vient des drapeaux de
compilation, que seul le binaire connaît, alors que le service est construit
depuis la configuration. Faire remonter l'information jusqu'à BuildCore
demanderait de changer sa signature dans trois appelants pour un champ
d'affichage.

Ce que cela apporte : dans le journal d'un démon partagé, la ligne
« Connecting client » distingue enfin les instances entre elles.
*/
func (s *Service) SetVersion(version string) {
	if version != "" {
		s.opts.Version = version
	}
}

// Hub expose le concentrateur d'événements, pour le handler de flux.
func (s *Service) Hub() *sse.Hub { return s.hub }

/*
Status résume la situation du module.

Ne remonte jamais d'erreur pour un démon absent : « aucun démon déclaré » EST un
état, et le premier qu'une instance fraîche connaît. En faire une erreur
obligerait l'interface à traiter le cas nominal comme une panne.
*/
func (s *Service) Status(ctx context.Context) Status {
	status := Status{
		Enabled:     s.opts.Enabled,
		IncomingDir: s.opts.IncomingDir,
	}

	if !s.opts.Enabled {
		status.State = StateDisabled
		status.Detail = "module désactivé — BOXINCLOUD_ED2K_ENABLED=false"
		return status
	}

	stored, err := s.repo.GetDaemon(ctx)
	switch {
	case errors.Is(err, ErrNoDaemonRow):
		status.State = StateUnconfigured
		status.Detail = "aucun démon aMule déclaré"
		return status

	case err != nil:
		// Une base injoignable n'est pas une absence de configuration. Le
		// distinguer évite de proposer un formulaire de création à quelqu'un
		// dont la configuration existe et se lit mal.
		s.log.Error("lecture de la configuration du démon", slog.Any("err", err))
		status.State = StateDisconnected
		status.Detail = "configuration illisible"
		return status
	}

	status.Daemon = &Daemon{
		Host:       stored.Host,
		Port:       stored.Port,
		Label:      stored.Label,
		LastState:  stored.LastState,
		LastDetail: stored.LastDetail,
		LastSeenAt: stored.LastSeenAt,
	}

	// À l'étape 0, un démon déclaré n'est jamais joint : le client EC arrive à
	// l'étape 1. L'état le dit tel quel plutôt que d'annoncer une connexion qui
	// n'a pas lieu.
	status.State = StateDisconnected
	status.Detail = "démon déclaré, client External Connections pas encore implémenté"
	return status
}

// Daemon retourne le démon déclaré, sans son mot de passe.
func (s *Service) Daemon(ctx context.Context) (Daemon, error) {
	if !s.opts.Enabled {
		return Daemon{}, ErrDisabled
	}

	stored, err := s.repo.GetDaemon(ctx)
	if errors.Is(err, ErrNoDaemonRow) {
		return Daemon{}, ErrNotConfigured
	}
	if err != nil {
		return Daemon{}, err
	}

	return Daemon{
		Host:       stored.Host,
		Port:       stored.Port,
		Label:      stored.Label,
		LastState:  stored.LastState,
		LastDetail: stored.LastDetail,
		LastSeenAt: stored.LastSeenAt,
	}, nil
}

// SetDaemonParams décrit le démon à déclarer.
type SetDaemonParams struct {
	Host     string
	Port     int
	Password string
	Label    string
}

/*
SetDaemon déclare ou remplace le démon.

Le mot de passe est scellé avant d'atteindre la base, avec la clé maître de
l'instance. Il ne ressort jamais : ni par l'API, ni dans un journal, ni dans un
message d'erreur.

Aucun test de connexion n'est fait ici — le client EC n'existe pas encore. Ce
sera le rôle de l'étape 1, et le contrôle suivra le modèle des backends de
stockage, qui joignent le service AVANT d'enregistrer : une adresse ou un mot de
passe fautif doit se signaler à la saisie, pas au premier téléchargement.
*/
func (s *Service) SetDaemon(ctx context.Context, p SetDaemonParams) (Daemon, error) {
	if !s.opts.Enabled {
		return Daemon{}, ErrDisabled
	}

	p.Host = strings.TrimSpace(p.Host)
	p.Label = strings.TrimSpace(p.Label)

	if err := validateDaemon(p); err != nil {
		return Daemon{}, err
	}

	sealed, err := s.sealer.Seal([]byte(p.Password))
	if err != nil {
		return Daemon{}, fmt.Errorf("scellement du mot de passe EC : %w", err)
	}

	stored := StoredDaemon{
		Host:        p.Host,
		Port:        p.Port,
		PasswordEnc: sealed,
		Label:       p.Label,
	}
	if err := s.repo.SaveDaemon(ctx, stored); err != nil {
		return Daemon{}, err
	}

	s.log.Info("démon aMule déclaré",
		slog.String("host", p.Host), slog.Int("port", p.Port))

	// Les autres navigateurs ouverts sur le module doivent le voir : deux
	// administrateurs qui travaillent en même temps ne doivent pas avoir à
	// recharger pour découvrir ce que l'autre vient de faire.
	s.publishStatus(ctx)

	return s.Daemon(ctx)
}

// ForgetDaemon oublie le démon déclaré.
//
// Oublier un démon absent n'est pas une erreur : le résultat demandé — aucun
// démon déclaré — est atteint dans les deux cas.
func (s *Service) ForgetDaemon(ctx context.Context) error {
	if !s.opts.Enabled {
		return ErrDisabled
	}

	if err := s.repo.DeleteDaemon(ctx); err != nil {
		return err
	}

	s.log.Info("démon aMule oublié")
	s.publishStatus(ctx)
	return nil
}

// publishStatus diffuse l'état courant aux navigateurs abonnés.
func (s *Service) publishStatus(ctx context.Context) {
	if s.hub == nil {
		return
	}
	s.hub.Publish(EventStatus, s.Status(ctx))
}

// EventStatus est le nom de l'événement portant l'état du module.
//
// Les noms d'événements sont des constantes parce qu'ils sont un contrat avec
// le navigateur au même titre qu'un chemin d'API : une faute de frappe donne
// une interface muette, sans erreur nulle part.
const EventStatus = "status"

/*
validateDaemon refuse une saisie qui ne peut pas marcher, en nommant le champ.

Le contrôle de l'hôte passe par netguard : c'est une adresse saisie depuis
l'interface qui devient une connexion émise PAR le serveur, donc la définition
d'une SSRF. Le garde laisse passer `amuled:4712` et `192.168.1.10` — les
adresses NORMALES d'une installation auto-hébergée — et refuse ce qui n'a aucune
raison d'être un démon aMule, à commencer par les services de métadonnées
d'instance.
*/
func validateDaemon(p SetDaemonParams) error {
	fields := map[string]string{}

	if p.Host == "" {
		fields["host"] = "obligatoire"
	}
	if p.Port < 1 || p.Port > 65535 {
		fields["port"] = "doit être compris entre 1 et 65535"
	}

	// amuled REFUSE une session dont le mot de passe est vide : accepter la
	// saisie ici ne ferait que déplacer l'échec au premier essai de connexion,
	// où il serait beaucoup moins clair.
	if p.Password == "" {
		fields["password"] = "obligatoire — amuled refuse une connexion sans mot de passe EC"
	}

	if p.Host != "" && p.Port >= 1 && p.Port <= 65535 {
		endpoint := net.JoinHostPort(p.Host, strconv.Itoa(p.Port))
		var forbidden netguard.ErrForbidden
		if err := netguard.Check(endpoint); errors.As(err, &forbidden) {
			fields["host"] = forbidden.Reason
		}
	}

	if len(fields) > 0 {
		return ValidationError{Fields: fields}
	}
	return nil
}
