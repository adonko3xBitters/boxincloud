package discovery

import (
	"container/list"
	"strings"
	"sync"
	"time"
)

/*
Cache des réponses de fournisseurs.

Deux problèmes distincts qu'un même mécanisme résout, et il vaut mieux les
nommer séparément parce qu'ils appellent des durées de vie différentes.

Le premier est la politesse. Trois utilisateurs qui cherchent « Moebius » dans
la même heure ne justifient pas trois interrogations de chaque catalogue, et
l'enrichissement d'une série de trente tomes redemande trente fois la même
fiche d'auteur.

Le second est la latence. Une recherche fédérée coûte le temps du catalogue le
plus lent ; le même mot cherché deux fois de suite doit être instantané la
seconde fois.

# La borne est la partie sérieuse

Un cache sans limite de taille est une fuite de mémoire à retardement : chaque
requête distincte y ajoute une entrée, aucune n'en sort, et une instance qui
tourne des mois finit par tomber. Le cas se déclenche tout seul — il suffit
d'une interface qui cherche à la frappe pour créer une entrée par préfixe de
mot.

D'où deux bornes simultanées, l'une temporelle et l'autre en nombre :

  - une entrée expire, parce qu'un catalogue distant change et qu'on ne peut pas
    le savoir ;
  - la plus anciennement utilisée est évincée quand le nombre plafonne, parce
    que l'expiration seule ne borne rien — mille requêtes distinctes en une
    minute tiennent toutes dans leur durée de vie.

C'est la seconde qui protège la mémoire. La première ne fait que rafraîchir.
*/

// Memo est un cache borné, à durée de vie et éviction LRU.
//
// Sûr d'emploi concurrent : il est lu et écrit depuis autant de goroutines
// qu'il y a de sources interrogées de front.
type Memo struct {
	mu sync.Mutex

	ttl     time.Duration
	maxSize int

	entries map[string]*list.Element
	// order porte les clés de la moins récemment utilisée à la plus récente.
	// Une liste chaînée plutôt qu'un tri : l'éviction et le rafraîchissement
	// doivent être en temps constant, sinon le cache coûte plus qu'il ne rend.
	order *list.List

	// now est remplaçable en test : vérifier une expiration ne doit pas demander
	// d'attendre réellement.
	now func() time.Time

	hits, misses int64
}

type memoEntry struct {
	key       string
	value     any
	expiresAt time.Time
}

/*
NewMemo construit un cache.

Les valeurs par défaut visent l'usage réel : quelques minutes suffisent à
absorber les recherches répétées d'une même session sans jamais servir une liste
franchement périmée, et quelques centaines d'entrées tiennent dans un mégaoctet
tout en couvrant largement l'activité d'une instance familiale.
*/
func NewMemo(ttl time.Duration, maxSize int) *Memo {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	if maxSize <= 0 {
		maxSize = 500
	}
	return &Memo{
		ttl:     ttl,
		maxSize: maxSize,
		entries: make(map[string]*list.Element, maxSize),
		order:   list.New(),
		now:     time.Now,
	}
}

// Get rend la valeur mémorisée, si elle existe et n'a pas expiré.
func (m *Memo) Get(key string) (any, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	element, ok := m.entries[key]
	if !ok {
		m.misses++
		return nil, false
	}

	entry := element.Value.(*memoEntry)
	if m.now().After(entry.expiresAt) {
		// Retirée à la lecture plutôt qu'attendue par un balayage : une entrée
		// périmée qu'on vient de consulter est exactement celle qui ne resservira
		// pas telle quelle, et un ramasse-miettes périodique serait une goroutine
		// de plus pour le même résultat.
		m.removeElement(element)
		m.misses++
		return nil, false
	}

	m.order.MoveToBack(element)
	m.hits++
	return entry.value, true
}

// Put mémorise une valeur, en évinçant la plus ancienne si nécessaire.
func (m *Memo) Put(key string, value any) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if element, ok := m.entries[key]; ok {
		entry := element.Value.(*memoEntry)
		entry.value = value
		entry.expiresAt = m.now().Add(m.ttl)
		m.order.MoveToBack(element)
		return
	}

	element := m.order.PushBack(&memoEntry{
		key:       key,
		value:     value,
		expiresAt: m.now().Add(m.ttl),
	})
	m.entries[key] = element

	for m.order.Len() > m.maxSize {
		if oldest := m.order.Front(); oldest != nil {
			m.removeElement(oldest)
		}
	}
}

// Invalidate retire les entrées d'un préfixe.
//
// Sert quand un catalogue change d'adresse ou est retiré : ses réponses
// mémorisées ne valent plus rien, et les laisser expirer d'elles-mêmes ferait
// afficher pendant plusieurs minutes les résultats d'une source qu'on vient de
// supprimer.
func (m *Memo) Invalidate(prefix string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for key, element := range m.entries {
		if strings.HasPrefix(key, prefix) {
			m.removeElement(element)
		}
	}
}

// Stats rend les compteurs, pour que l'efficacité du cache soit observable
// plutôt que supposée.
func (m *Memo) Stats() (entries int, hits, misses int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.order.Len(), m.hits, m.misses
}

func (m *Memo) removeElement(element *list.Element) {
	entry := element.Value.(*memoEntry)
	m.order.Remove(element)
	delete(m.entries, entry.key)
}

/*
MemoKey compose une clé de cache.

Le genre du fournisseur préfixe tout, pour que `Invalidate` puisse cibler une
source sans toucher aux autres. La requête est normalisée par la même fonction
que les titres : « Moebius », « moebius » et « Mœbius  » sont la même recherche,
et les mémoriser trois fois annulerait l'intérêt du cache là où il sert le plus.
*/
func MemoKey(kind, sourceID, query string) string {
	return kind + "\x00" + sourceID + "\x00" + normalizeTitle(query)
}
