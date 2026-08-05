package amuletest

import (
	"fmt"
	"net"
	"testing"
	"time"
)

// TestIntegrationAmuledDemarre vérifie que le banc d'essai rend un démon dont
// le port EC est réellement ouvert.
//
// Test de fumée volontairement muet sur le protocole : le codec EC n'existe pas
// encore. Il garantit seulement que quand ce codec arrivera, il aura un
// interlocuteur qui écoute.
func TestIntegrationAmuledDemarre(t *testing.T) {
	env := Start(t)

	if env.Password == "" {
		t.Fatal("mot de passe EC vide")
	}

	addr := net.JoinHostPort(env.Host, fmt.Sprint(env.Port))
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		t.Fatalf("connexion TCP au port EC %s : %v", addr, err)
	}
	_ = conn.Close()
}
