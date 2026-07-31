package discovery

import (
	"strconv"
	"testing"
	"time"
)

func newTestMemo(ttl time.Duration, size int) (*Memo, *time.Time) {
	now := time.Unix(0, 0)
	memo := NewMemo(ttl, size)
	memo.now = func() time.Time { return now }
	return memo, &now
}

func TestMemoRoundTrip(t *testing.T) {
	memo, _ := newTestMemo(time.Minute, 10)

	if _, ok := memo.Get("absent"); ok {
		t.Error("une clé absente ne doit rien rendre")
	}

	memo.Put("k", []Result{{Title: "L'Incal"}})

	value, ok := memo.Get("k")
	if !ok {
		t.Fatal("la valeur mémorisée n'est pas relue")
	}
	if results, _ := value.([]Result); len(results) != 1 || results[0].Title != "L'Incal" {
		t.Errorf("valeur = %+v", value)
	}
}

func TestMemoExpires(t *testing.T) {
	memo, now := newTestMemo(time.Minute, 10)
	memo.Put("k", "v")

	*now = now.Add(59 * time.Second)
	if _, ok := memo.Get("k"); !ok {
		t.Error("l'entrée a expiré trop tôt")
	}

	*now = now.Add(2 * time.Second)
	if _, ok := memo.Get("k"); ok {
		t.Error("l'entrée aurait dû expirer")
	}
}

/*
TestMemoEvictsBeyondMaxSize est le test qui justifie la borne.

Un cache sans limite de taille est une fuite de mémoire à retardement, et le cas
se déclenche tout seul : une interface qui cherche à la frappe crée une entrée
par préfixe de mot. L'expiration ne borne rien — mille requêtes distinctes en
une minute tiennent toutes dans leur durée de vie.
*/
func TestMemoEvictsBeyondMaxSize(t *testing.T) {
	const size = 50
	memo, _ := newTestMemo(time.Hour, size)

	for i := 0; i < size*20; i++ {
		memo.Put("clé-"+strconv.Itoa(i), i)
	}

	entries, _, _ := memo.Stats()
	if entries > size {
		t.Errorf("%d entrées pour un plafond de %d : le cache ne borne rien", entries, size)
	}

	// Les plus anciennes sont parties, les plus récentes sont là.
	if _, ok := memo.Get("clé-0"); ok {
		t.Error("la plus ancienne entrée n'a pas été évincée")
	}
	if _, ok := memo.Get("clé-" + strconv.Itoa(size*20-1)); !ok {
		t.Error("la plus récente entrée a été évincée")
	}
}

// TestMemoEvictsLeastRecentlyUsed : consulter une entrée la protège.
//
// Sans cela, l'éviction serait chronologique et jetterait la recherche qu'on
// répète le plus au profit de celle qu'on n'a faite qu'une fois.
func TestMemoEvictsLeastRecentlyUsed(t *testing.T) {
	memo, _ := newTestMemo(time.Hour, 3)

	memo.Put("a", 1)
	memo.Put("b", 2)
	memo.Put("c", 3)

	// « a » redevient la plus récemment utilisée.
	if _, ok := memo.Get("a"); !ok {
		t.Fatal("a devrait être présente")
	}

	memo.Put("d", 4)

	if _, ok := memo.Get("a"); !ok {
		t.Error("« a » a été évincée alors qu'elle venait d'être consultée")
	}
	if _, ok := memo.Get("b"); ok {
		t.Error("« b », la moins récemment utilisée, aurait dû partir")
	}
}

// TestMemoInvalidate : retirer un catalogue doit retirer ses réponses.
//
// Les laisser expirer d'elles-mêmes ferait afficher pendant plusieurs minutes
// les résultats d'une source qu'on vient de supprimer.
func TestMemoInvalidate(t *testing.T) {
	memo, _ := newTestMemo(time.Hour, 10)

	memo.Put(MemoKey("opds", "source-1", "incal"), "x")
	memo.Put(MemoKey("opds", "source-1", "moebius"), "y")
	memo.Put(MemoKey("opds", "source-2", "incal"), "z")

	memo.Invalidate(MemoKey("opds", "source-1", ""))

	if _, ok := memo.Get(MemoKey("opds", "source-1", "incal")); ok {
		t.Error("les réponses de la source retirée sont toujours là")
	}
	if _, ok := memo.Get(MemoKey("opds", "source-2", "incal")); !ok {
		t.Error("les réponses d'une autre source ont été emportées")
	}
}

// TestMemoKeyNormalizes : la même recherche écrite autrement est la même entrée.
//
// Sans cela, le cache raterait précisément là où il sert le plus — quelqu'un qui
// reformule sa recherche à une majuscule près.
func TestMemoKeyNormalizes(t *testing.T) {
	same := []string{"Moebius", "moebius", "  MOEBIUS  ", "Mœbius"}
	want := MemoKey("opds", "s", same[0])

	for _, form := range same[1:] {
		if got := MemoKey("opds", "s", form); got != want {
			t.Errorf("MemoKey(%q) = %q, attendu %q", form, got, want)
		}
	}

	if MemoKey("opds", "s1", "x") == MemoKey("opds", "s2", "x") {
		t.Error("deux sources partagent une entrée de cache")
	}
}

func TestMemoUpdateKeepsOneEntry(t *testing.T) {
	memo, _ := newTestMemo(time.Hour, 10)

	memo.Put("k", 1)
	memo.Put("k", 2)

	entries, _, _ := memo.Stats()
	if entries != 1 {
		t.Errorf("%d entrées après deux écritures de la même clé", entries)
	}
	if value, _ := memo.Get("k"); value != 2 {
		t.Errorf("valeur = %v, attendu 2", value)
	}
}
