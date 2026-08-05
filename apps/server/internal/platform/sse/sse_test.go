package sse

import (
	"bufio"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newTestHub(t *testing.T) *Hub {
	t.Helper()
	return NewHub(slog.New(slog.NewTextHandler(io.Discard, nil)), Options{})
}

func TestEncodeProduitUneTrameValide(t *testing.T) {
	frame, err := encode("status", map[string]string{"state": "disabled"})
	if err != nil {
		t.Fatalf("encode : %v", err)
	}

	want := "event: status\ndata: {\"state\":\"disabled\"}\n\n"
	if string(frame) != want {
		t.Errorf("trame =\n%q\nattendu\n%q", frame, want)
	}
}

/*
TestEncodeNeCoupeJamaisLeCadrage vérifie l'invariant sur lequel repose `encode`.

Le cadrage SSE se termine sur une ligne vide : un saut de ligne littéral dans le
corps ferait voir au client la fin du message là où elle n'est pas. `encode`
n'écrit qu'un seul champ `data:` parce que `json.Marshal` compacte toujours sa
sortie — y compris celle d'un json.RawMessage saisi sur plusieurs lignes, qui
est le cas le plus tordu qu'un appelant puisse produire.

Ce test EST cet invariant. S'il tombe un jour, c'est `encode` qui doit apprendre
à découper, pas ce test qu'il faut ajuster.
*/
func TestEncodeNeCoupeJamaisLeCadrage(t *testing.T) {
	cases := []struct {
		nom     string
		payload any
	}{
		{"structure", map[string]string{"a": "première\nseconde"}},
		{"json déjà formaté", json.RawMessage("{\n  \"ligne\": 2\n}")},
		{"chaîne multiligne", "première\nseconde"},
	}

	for _, c := range cases {
		t.Run(c.nom, func(t *testing.T) {
			frame, err := encode("log", c.payload)
			if err != nil {
				t.Fatalf("encode : %v", err)
			}

			if strings.Count(string(frame), "data: ") != 1 {
				t.Errorf("un seul champ data attendu, obtenu :\n%q", frame)
			}
			// Une ligne vide ailleurs qu'à la fin couperait le message.
			if strings.Contains(strings.TrimSuffix(string(frame), "\n\n"), "\n\n") {
				t.Errorf("ligne vide au milieu de la trame :\n%q", frame)
			}
		})
	}
}

/*
TestServeEnvoieEtatInitialPuisEvenements teste ce qui compte de bout en bout.

Un vrai serveur HTTP et un vrai client, pas un ResponseRecorder : ce qu'on
vérifie ici — que les octets partent AVANT la fin de la réponse — est
précisément ce qu'un enregistreur en mémoire ne peut pas montrer.
*/
func TestServeEnvoieEtatInitialPuisEvenements(t *testing.T) {
	hub := newTestHub(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hub.Serve(w, r, "status", map[string]string{"state": "unconfigured"})
	}))
	defer srv.Close()

	res, err := http.Get(srv.URL) //nolint:noctx // le flux est fermé par le défer
	if err != nil {
		t.Fatalf("connexion : %v", err)
	}
	defer func() { _ = res.Body.Close() }()

	if got := res.Header.Get("Content-Type"); !strings.HasPrefix(got, "text/event-stream") {
		t.Errorf("Content-Type = %q, attendu text/event-stream", got)
	}
	if got := res.Header.Get("X-Accel-Buffering"); got != "no" {
		t.Errorf("X-Accel-Buffering = %q — sans lui le flux meurt derrière un proxy", got)
	}

	reader := bufio.NewReader(res.Body)
	if body := readEvent(t, reader); !strings.Contains(body, "unconfigured") {
		t.Errorf("premier événement = %q, attendu l'état initial", body)
	}

	// L'abonnement est pris pendant Serve : on attend qu'il soit enregistré
	// avant de publier, sinon l'événement part dans le vide.
	waitFor(t, func() bool { return hub.Subscribers() == 1 })

	hub.Publish("status", map[string]string{"state": "disconnected"})

	if body := readEvent(t, reader); !strings.Contains(body, "disconnected") {
		t.Errorf("second événement = %q, attendu l'état publié", body)
	}
}

// TestOnChangeSuitLesAbonnes vérifie le signal qui permet à un producteur de ne
// rien produire quand personne ne regarde.
func TestOnChangeSuitLesAbonnes(t *testing.T) {
	hub := newTestHub(t)

	counts := make(chan int, 8)
	hub.OnChange(func(n int) { counts <- n })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hub.Serve(w, r, "status", struct{}{})
	}))
	defer srv.Close()

	res, err := http.Get(srv.URL) //nolint:noctx // le flux est fermé juste après
	if err != nil {
		t.Fatalf("connexion : %v", err)
	}

	if got := <-counts; got != 1 {
		t.Errorf("après connexion, %d abonnés, attendu 1", got)
	}

	_ = res.Body.Close()

	if got := <-counts; got != 0 {
		t.Errorf("après déconnexion, %d abonnés, attendu 0", got)
	}
}

/*
TestAbonneLentEstDeconnecte : nos événements portent un état, pas un incrément.

Sauter des événements pour un client en retard lui ferait afficher une vue
fausse que rien ne corrigerait. On le déconnecte donc, et EventSource le fera
repartir d'un état complet.
*/
func TestAbonneLentEstDeconnecte(t *testing.T) {
	hub := newTestHub(t)

	// Un abonné qui ne lit jamais : on l'inscrit directement, sans passer par
	// Serve, pour contrôler exactement ce qui est consommé.
	sub := hub.subscribe()

	for i := 0; i < defaultBuffer+1; i++ {
		hub.Publish("status", map[string]int{"n": i})
	}

	if hub.Subscribers() != 0 {
		t.Fatal("un abonné qui ne lit pas doit finir déconnecté")
	}

	// Le canal porte encore ses trames en attente : c'est en le VIDANT qu'on
	// atteint la fermeture. Serve fait exactement cela, et rend la main quand
	// le canal est clos.
	for range sub.ch { //nolint:revive // on vide jusqu'à la fermeture
	}
}

// readEvent lit une trame complète, terminée par une ligne vide.
func readEvent(t *testing.T, reader *bufio.Reader) string {
	t.Helper()

	var frame strings.Builder
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("lecture du flux : %v", err)
		}
		// Les commentaires de maintien et l'annonce de reconnexion ne sont pas
		// des événements : on les traverse.
		if strings.HasPrefix(line, ":") || strings.HasPrefix(line, "retry:") {
			continue
		}
		if line == "\n" {
			if frame.Len() == 0 {
				continue
			}
			return frame.String()
		}
		frame.WriteString(line)
	}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition non atteinte dans le délai")
}
