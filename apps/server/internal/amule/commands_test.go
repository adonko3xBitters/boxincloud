package amule

import (
	"errors"
	"strings"
	"testing"
)

/*
TestPriorityCodesEtLectureSontDAccord.

Deux tables décrivent la même correspondance : `mapPartfilePriority` la lit,
`priorityCodes` l'écrit. Elles sont séparées à dessein — la lecture accepte le
décalage « auto » que l'écriture ne doit jamais produire — et cette séparation
est exactement ce qui les laisse diverger en silence.

Ce test est le lien manquant : tout code que l'écriture produit doit être relu
comme la priorité de départ.
*/
func TestPriorityCodesEtLectureSontDAccord(t *testing.T) {
	for priority, code := range priorityCodes {
		relu := mapPartfilePriority(code)
		if relu != priority {
			t.Errorf("priorité %q écrite en %d, relue %q", priority, code, relu)
		}
	}

	// Et toute priorité du domaine doit pouvoir s'écrire : une priorité qu'on
	// sait afficher mais pas régler est un bouton qui ne marche pas.
	for _, priority := range []Priority{
		PriorityLow, PriorityNormal, PriorityHigh,
		PriorityVeryLow, PriorityVeryHigh, PriorityAuto,
	} {
		if _, ok := priorityCodes[priority]; !ok {
			t.Errorf("la priorité %q ne peut pas être réglée", priority)
		}
	}
}

// TestParseHashRefuseCeQuiNEstPasUneEmpreinte.
//
// Une empreinte tronquée produirait un tag mal formé, donc une trame rejetée
// par le démon avec un message qui ne parle pas de l'empreinte. Autant refuser
// ici, où l'on sait de quoi il s'agit.
func TestParseHashRefuseCeQuiNEstPasUneEmpreinte(t *testing.T) {
	cases := []struct {
		nom    string
		hash   string
		valide bool
	}{
		{"seize octets", strings.Repeat("ab", 16), true},
		{"majuscules", strings.Repeat("AB", 16), true},
		{"espaces autour", "  " + strings.Repeat("ab", 16) + "  ", true},
		{"trop court", strings.Repeat("ab", 15), false},
		{"trop long", strings.Repeat("ab", 17), false},
		{"pas hexadécimal", strings.Repeat("zz", 16), false},
		{"vide", "", false},
	}

	for _, c := range cases {
		t.Run(c.nom, func(t *testing.T) {
			raw, err := parseHash(c.hash)

			if c.valide {
				if err != nil {
					t.Fatalf("empreinte valide refusée : %v", err)
				}
				if len(raw) != 16 {
					t.Errorf("%d octets décodés, attendu 16", len(raw))
				}
				return
			}

			if err == nil {
				t.Fatal("empreinte invalide acceptée")
			}
			if !errors.Is(err, ErrInvalidHash) {
				t.Errorf("erreur = %v, attendu ErrInvalidHash", err)
			}
		})
	}
}

/*
TestAddLinkRefuseLesAutresProtocoles.

Le module ne traite que l'eD2k, et c'est une contrainte de conception, pas une
limite temporaire. Un magnet ou une adresse HTTP collés ici doivent recevoir un
refus qui NOMME la raison — le démon les rejetterait aussi, mais son message ne
dirait pas ce qu'on attendait à la place.
*/
func TestAddLinkRefuseLesAutresProtocoles(t *testing.T) {
	svc := newTestService(t, &fakeRepo{}, enabled())

	for _, link := range []string{
		"magnet:?xt=urn:btih:abcdef",
		"https://exemple.org/fichier.iso",
		"/home/moi/fichier.iso",
		"",
	} {
		t.Run(link, func(t *testing.T) {
			err := svc.AddLink(t.Context(), link)
			if !errors.Is(err, ErrInvalidLink) {
				t.Errorf("erreur = %v, attendu ErrInvalidLink", err)
			}
		})
	}
}

// TestActOnDownloadRefuseUnGesteInconnu : la table des gestes est fermée, et
// un geste absent doit se voir plutôt que de partir en trame vide.
func TestActOnDownloadRefuseUnGesteInconnu(t *testing.T) {
	svc := newTestService(t, &fakeRepo{}, enabled())

	err := svc.ActOnDownload(t.Context(), strings.Repeat("ab", 16), DownloadAction("effacer-tout"))
	if err == nil {
		t.Fatal("geste inconnu accepté")
	}
	if !strings.Contains(err.Error(), "effacer-tout") {
		t.Errorf("le message devrait nommer le geste fautif : %v", err)
	}
}

// TestCommandesRefuseesModuleEteint : le drapeau vaut aussi pour ce qui agit.
func TestCommandesRefuseesModuleEteint(t *testing.T) {
	svc := newTestService(t, &fakeRepo{}, Options{Enabled: false})
	ctx := t.Context()
	hash := strings.Repeat("ab", 16)

	checks := map[string]error{
		"pause":       svc.ActOnDownload(ctx, hash, DownloadPause),
		"priorité":    svc.SetDownloadPriority(ctx, hash, PriorityHigh),
		"connexion":   svc.ConnectServer(ctx, "", 0),
		"déconnexion": svc.DisconnectServer(ctx),
		"kad":         svc.StartKad(ctx),
		"lien":        svc.AddLink(ctx, "ed2k://|file|essai|1|"+strings.Repeat("AB", 16)+"|/"),
		"partage":     svc.ReloadSharedFiles(ctx),
	}

	for nom, err := range checks {
		if !errors.Is(err, ErrDisabled) {
			t.Errorf("%s : erreur = %v, attendu ErrDisabled", nom, err)
		}
	}
}
