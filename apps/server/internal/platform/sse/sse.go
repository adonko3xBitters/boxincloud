/*
Package sse diffuse des événements vers des navigateurs, en Server-Sent Events.

Rien ici ne connaît eD2k. Le concentrateur est générique parce que le besoin
l'est : la progression d'un scan de bibliothèque, aujourd'hui sondée toutes les
deux secondes par l'interface, en est le second usage naturel.

# Pourquoi SSE plutôt qu'un WebSocket

Le besoin est unidirectionnel. Les commandes partent en REST ordinaire, où elles
bénéficient de l'authentification, de la limitation de débit et du format
d'erreur du projet ; seuls les changements d'état redescendent. SSE fait
exactement cela, sur net/http, sans dépendance, et se reconnecte tout seul —
logique qu'un WebSocket aurait fallu écrire.

Voir docs/adr/005-temps-reel-sse-evenements-derives.md.

# Ce qui tue un flux SSE en production

Trois choses, toutes traitées ici, et aucune ne se voit en développement :

  - un proxy qui met la réponse en tampon et n'envoie rien avant la fin — d'où
    `X-Accel-Buffering: no` et un vidage explicite après chaque écriture ;
  - un intermédiaire qui coupe une connexion muette au bout d'une minute — d'où
    le battement de cœur ;
  - un délai de requête posé par un middleware — le routeur doit donc placer la
    route de flux hors du groupe qui borne les requêtes ordinaires.
*/
package sse

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// Réglages par défaut du concentrateur.
const (
	// defaultHeartbeat passe sous les délais d'inactivité usuels des
	// reverse-proxies, qui commencent à 60 s chez nginx.
	defaultHeartbeat = 20 * time.Second

	// defaultBuffer est le nombre d'événements qu'un abonné peut prendre de
	// retard avant d'être déconnecté. Voir la note sur les lents, plus bas.
	defaultBuffer = 64

	// reconnectDelay est le délai que le navigateur respectera avant de se
	// reconnecter. Annoncé une fois, à l'ouverture.
	reconnectDelay = 3 * time.Second
)

// Hub diffuse des événements à tous les abonnés connectés.
//
// Sûr d'emploi concurrent : Publish peut être appelé depuis n'importe quelle
// goroutine, Serve depuis autant de requêtes que nécessaire.
type Hub struct {
	log       *slog.Logger
	heartbeat time.Duration

	mu          sync.Mutex
	subscribers map[*subscriber]struct{}
	onChange    func(count int)
}

type subscriber struct {
	ch chan []byte
}

// Options règle le concentrateur. Le zéro convient.
type Options struct {
	// Heartbeat espace les commentaires de maintien. Zéro prend le défaut.
	Heartbeat time.Duration
}

func NewHub(log *slog.Logger, opts Options) *Hub {
	heartbeat := opts.Heartbeat
	if heartbeat <= 0 {
		heartbeat = defaultHeartbeat
	}
	return &Hub{
		log:         log,
		heartbeat:   heartbeat,
		subscribers: make(map[*subscriber]struct{}),
	}
}

/*
OnChange enregistre un rappel déclenché quand le nombre d'abonnés change.

C'est ce qui permet à un producteur de ne rien produire quand personne ne
regarde. Le module eD2k s'en sert pour arrêter complètement sa scrutation du
démon dès le départ du dernier navigateur : une instance que personne n'observe
ne doit générer aucun trafic.

Le rappel est appelé en tenant le verrou du concentrateur : il doit être court
et ne jamais rappeler le concentrateur, sous peine d'interblocage.
*/
func (h *Hub) OnChange(fn func(count int)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onChange = fn
}

// Subscribers indique combien de navigateurs sont connectés.
func (h *Hub) Subscribers() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.subscribers)
}

/*
Publish diffuse un événement à tous les abonnés.

L'encodage est fait UNE fois, hors du verrou : diffuser à vingt onglets ne doit
pas coûter vingt sérialisations du même état.

Un payload inencodable est journalisé et ignoré. Faire remonter l'erreur
obligerait chaque appelant — souvent une boucle de fond sans personne pour la
lire — à décider quoi en faire, et la réponse serait partout la même.
*/
func (h *Hub) Publish(event string, payload any) {
	frame, err := encode(event, payload)
	if err != nil {
		h.log.Error("événement SSE inencodable",
			slog.String("event", event), slog.Any("err", err))
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	for sub := range h.subscribers {
		select {
		case sub.ch <- frame:
		default:
			/*
				Abonné trop lent : on le déconnecte au lieu de lui sauter des
				événements.

				Nos événements portent un ÉTAT, pas un incrément. Un client qui
				en reçoit un sous-ensemble arbitraire affiche donc une vue
				fausse, et rien ne la corrigera. Fermer le canal le fait se
				reconnecter — EventSource le fait seul — et repartir d'un état
				complet, ce qui est toujours préférable à une vue plausible et
				erronée.
			*/
			close(sub.ch)
			delete(h.subscribers, sub)
			h.log.Warn("abonné SSE trop lent, déconnecté",
				slog.String("event", event), slog.Int("buffer", defaultBuffer))
			h.notifyLocked()
		}
	}
}

/*
Serve tient un flux ouvert jusqu'à la déconnexion du client.

Bloque : c'est un handler HTTP à part entière, et il ne rend la main que lorsque
le navigateur s'en va ou que le contexte de la requête est annulé.

`initial` est envoyé immédiatement, avant tout autre événement. Sans lui, une
interface fraîchement ouverte reste vide jusqu'au premier changement — ce qui,
sur une file au repos, peut durer indéfiniment.
*/
func (h *Hub) Serve(w http.ResponseWriter, r *http.Request, event string, initial any) {
	/*
		http.NewResponseController plutôt qu'une assertion vers http.Flusher.

		Le ResponseWriter reçu ici est emballé au moins deux fois — par le
		journal des requêtes et par la compression — et une assertion de type
		échouerait sur l'emballage extérieur. Le contrôleur, lui, déballe.
	*/
	rc := http.NewResponseController(w)

	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Connection", "keep-alive")
	// nginx, et tout ce qui l'imite, met la réponse en tampon par défaut : sans
	// cet en-tête le flux fonctionne en direct et meurt derrière le proxy.
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	if err := rc.Flush(); err != nil {
		// Un ResponseWriter qui ne sait pas vider ne peut pas porter de flux :
		// mieux vaut le constater ici que laisser le client attendre.
		h.log.Error("flux SSE impossible : le ResponseWriter ne sait pas vider",
			slog.Any("err", err))
		return
	}

	// Le délai de reconnexion est annoncé une fois, à l'ouverture.
	if _, err := fmt.Fprintf(w, "retry: %d\n\n", reconnectDelay.Milliseconds()); err != nil {
		return
	}

	if frame, err := encode(event, initial); err == nil {
		if _, err := w.Write(frame); err != nil {
			return
		}
	}
	if err := rc.Flush(); err != nil {
		return
	}

	sub := h.subscribe()
	defer h.unsubscribe(sub)

	ticker := time.NewTicker(h.heartbeat)
	defer ticker.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return

		case frame, open := <-sub.ch:
			if !open {
				// Fermé par Publish : abonné trop lent. On rend la main, le
				// navigateur se reconnectera et repartira d'un état complet.
				return
			}
			if _, err := w.Write(frame); err != nil {
				return
			}
			if err := rc.Flush(); err != nil {
				return
			}

		case <-ticker.C:
			// Un commentaire, que la spécification autorise et que les clients
			// ignorent. Il ne sert qu'à faire passer des octets.
			if _, err := fmt.Fprint(w, ": battement\n\n"); err != nil {
				return
			}
			if err := rc.Flush(); err != nil {
				return
			}
		}
	}
}

func (h *Hub) subscribe() *subscriber {
	sub := &subscriber{ch: make(chan []byte, defaultBuffer)}

	h.mu.Lock()
	defer h.mu.Unlock()
	h.subscribers[sub] = struct{}{}
	h.notifyLocked()
	return sub
}

func (h *Hub) unsubscribe(sub *subscriber) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Publish a pu le retirer et fermer son canal entre-temps : le retirer deux
	// fois est sans effet, le refermer paniquerait.
	if _, present := h.subscribers[sub]; !present {
		return
	}
	delete(h.subscribers, sub)
	close(sub.ch)
	h.notifyLocked()
}

// notifyLocked appelle le rappel de changement. Le verrou doit être tenu.
func (h *Hub) notifyLocked() {
	if h.onChange != nil {
		h.onChange(len(h.subscribers))
	}
}

/*
encode produit une trame SSE complète.

Un seul champ `data:`, et c'est un invariant, pas une simplification.

Le cadrage SSE se termine sur une ligne vide : un saut de ligne littéral dans le
corps ferait voir au client la fin du message là où elle n'est pas. La règle de
la spécification est donc un champ `data:` par ligne — et il se trouve que la
question ne se pose jamais ici, parce que `json.Marshal` compacte toujours sa
sortie, y compris celle d'un json.Marshaler ou d'un json.RawMessage.

Découper sur les sauts de ligne serait donc du code qu'aucun appel ne peut
atteindre, et qu'aucun test ne peut couvrir. Le jour où une fonction publierait
des octets déjà formatés sans passer par json.Marshal, c'est ELLE qui devra
porter la découpe — l'invariant tombe avec elle, pas avant.
*/
func encode(event string, payload any) ([]byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	var out []byte
	if event != "" {
		out = append(out, "event: "...)
		out = append(out, event...)
		out = append(out, '\n')
	}

	out = append(out, "data: "...)
	out = append(out, body...)
	return append(out, '\n', '\n'), nil
}
