package scraper

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

// Le répertoire configuré mais absent doit AVERTIR, et dire quel chemin a été
// consulté. Un chemin relatif se résout contre le répertoire de lancement : la
// chaîne saisie ne suffit pas à comprendre l'échec.
func TestLoadDirWarnsWithResolvedPath(t *testing.T) {
	var out bytes.Buffer
	log := slog.New(slog.NewTextHandler(&out, &slog.HandlerOptions{Level: slog.LevelDebug}))

	catalog := &Catalog{byID: map[string]*Compiled{}}
	if err := catalog.LoadDir("gabarits/absents", log); err != nil {
		t.Fatalf("un répertoire absent ne doit pas être une erreur : %v", err)
	}

	logged := out.String()
	if !strings.Contains(logged, "level=WARN") {
		t.Errorf("attendu un avertissement, pas une information :\n%s", logged)
	}
	if !strings.Contains(logged, "gabarits/absents") {
		t.Errorf("le chemin configuré n'est pas signalé :\n%s", logged)
	}
	// Le chemin résolu est ABSOLU : c'est lui qui explique l'échec, et il est
	// entouré de guillemets par slog dès qu'il contient un espace.
	if !strings.Contains(logged, "résolu=/") && !strings.Contains(logged, `résolu="/`) {
		t.Errorf("le chemin résolu n'est pas signalé :\n%s", logged)
	}
}
