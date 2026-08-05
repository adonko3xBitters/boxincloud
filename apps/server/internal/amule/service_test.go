package amule

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/adonko3xBitters/boxincloud/server/internal/platform/crypto"
	"github.com/adonko3xBitters/boxincloud/server/internal/platform/sse"
)

// fakeRepo tient la configuration en mémoire.
//
// Doubler le dépôt plutôt que la base : ce qui est testé ici est la logique du
// service — validation, scellement, états — et elle ne dépend pas de PostgreSQL.
// La persistance réelle est couverte par les tests de contrat.
type fakeRepo struct {
	stored  *StoredDaemon
	saveErr error
	getErr  error

	destinations map[int]Destination
	publications map[string]Publication
}

func (f *fakeRepo) GetDaemon(context.Context) (StoredDaemon, error) {
	if f.getErr != nil {
		return StoredDaemon{}, f.getErr
	}
	if f.stored == nil {
		return StoredDaemon{}, ErrNoDaemonRow
	}
	return *f.stored, nil
}

func (f *fakeRepo) SaveDaemon(_ context.Context, d StoredDaemon) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.stored = &d
	return nil
}

func (f *fakeRepo) DeleteDaemon(context.Context) error {
	f.stored = nil
	return nil
}

func (f *fakeRepo) SetState(_ context.Context, state State, detail string, _ bool) error {
	if f.stored != nil {
		f.stored.LastState = string(state)
		f.stored.LastDetail = detail
	}
	return nil
}

func newTestService(t *testing.T, repo Repository, opts Options) *Service {
	t.Helper()

	sealer, err := crypto.NewSealer(bytes.Repeat([]byte{0x17}, 32))
	if err != nil {
		t.Fatalf("NewSealer : %v", err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	return NewService(repo, sealer, sse.NewHub(log, sse.Options{}), opts, log)
}

func enabled() Options {
	return Options{Enabled: true, IncomingDir: "/incoming"}
}

// TestStatusDesactive : l'état par défaut d'une instance se dit, il ne s'échoue pas.
func TestStatusDesactive(t *testing.T) {
	svc := newTestService(t, &fakeRepo{}, Options{Enabled: false})

	status := svc.Status(context.Background())
	if status.State != StateDisabled {
		t.Errorf("état = %q, attendu %q", status.State, StateDisabled)
	}
	if status.Detail == "" {
		t.Error("un état doit dire pourquoi il est celui-là")
	}
}

// TestStatusSansDemon : « aucun démon déclaré » est un état, pas une panne.
func TestStatusSansDemon(t *testing.T) {
	svc := newTestService(t, &fakeRepo{}, enabled())

	status := svc.Status(context.Background())
	if status.State != StateUnconfigured {
		t.Errorf("état = %q, attendu %q", status.State, StateUnconfigured)
	}
	if status.Daemon != nil {
		t.Error("aucun démon ne doit être décrit tant qu'aucun n'est déclaré")
	}
	if status.IncomingDir != "/incoming" {
		t.Errorf("IncomingDir = %q — il doit être visible, un montage manquant est "+
			"la première cause de « rien n'arrive »", status.IncomingDir)
	}
}

/*
TestStatusDistingueBaseIllisibleDeConfigurationAbsente.

Confondre les deux ferait proposer un formulaire de création à quelqu'un dont la
configuration existe et se lit mal — et l'inviterait donc à la réécrire pour
résoudre un problème qui n'est pas là.
*/
func TestStatusDistingueBaseIllisibleDeConfigurationAbsente(t *testing.T) {
	repo := &fakeRepo{getErr: errors.New("connexion perdue")}
	svc := newTestService(t, repo, enabled())

	status := svc.Status(context.Background())
	if status.State == StateUnconfigured {
		t.Error("une base illisible ne doit pas se présenter comme une absence de configuration")
	}
}

func TestSetDaemonScelleLeMotDePasse(t *testing.T) {
	repo := &fakeRepo{}
	svc := newTestService(t, repo, enabled())

	const password = "un mot de passe EC"

	daemon, err := svc.SetDaemon(context.Background(), SetDaemonParams{
		Host: "amuled", Port: 4712, Password: password, Label: "démon du salon",
	})
	if err != nil {
		t.Fatalf("SetDaemon : %v", err)
	}
	if daemon.Host != "amuled" || daemon.Port != 4712 {
		t.Errorf("démon = %+v, attendu amuled:4712", daemon)
	}

	if repo.stored == nil {
		t.Fatal("rien n'a été persisté")
	}
	if bytes.Contains(repo.stored.PasswordEnc, []byte(password)) {
		t.Error("le mot de passe EC apparaît en clair dans ce qui est persisté")
	}
	if len(repo.stored.PasswordEnc) == 0 {
		t.Error("le mot de passe scellé est vide")
	}
}

/*
TestDaemonNePorteJamaisLeMotDePasse est le test qui compte le plus de ce fichier.

Le type Daemon est ce que l'API renvoie. S'il gagnait un jour un champ portant
le mot de passe — par commodité, pour un formulaire pré-rempli — le secret
partirait vers le navigateur de tout administrateur. Ce test ferme la porte en
inspectant la valeur elle-même.
*/
func TestDaemonNePorteJamaisLeMotDePasse(t *testing.T) {
	repo := &fakeRepo{}
	svc := newTestService(t, repo, enabled())

	const password = "seCr3t-ec-tres-reconnaissable"

	if _, err := svc.SetDaemon(context.Background(), SetDaemonParams{
		Host: "amuled", Port: 4712, Password: password,
	}); err != nil {
		t.Fatalf("SetDaemon : %v", err)
	}

	daemon, err := svc.Daemon(context.Background())
	if err != nil {
		t.Fatalf("Daemon : %v", err)
	}

	if strings.Contains(rendu(t, daemon), password) {
		t.Error("le mot de passe EC est atteignable depuis le type exposé par l'API")
	}
}

func TestSetDaemonRefuseUneSaisieImpossible(t *testing.T) {
	cases := []struct {
		nom   string
		in    SetDaemonParams
		champ string
	}{
		{"hôte vide", SetDaemonParams{Port: 4712, Password: "x"}, "host"},
		{"port nul", SetDaemonParams{Host: "amuled", Password: "x"}, "port"},
		{"port hors bornes", SetDaemonParams{Host: "amuled", Port: 70000, Password: "x"}, "port"},
		{"mot de passe vide", SetDaemonParams{Host: "amuled", Port: 4712}, "password"},
		{
			"métadonnées d'instance",
			SetDaemonParams{Host: "169.254.169.254", Port: 4712, Password: "x"},
			"host",
		},
	}

	for _, c := range cases {
		t.Run(c.nom, func(t *testing.T) {
			svc := newTestService(t, &fakeRepo{}, enabled())

			_, err := svc.SetDaemon(context.Background(), c.in)

			var invalid ValidationError
			if !errors.As(err, &invalid) {
				t.Fatalf("erreur = %v (%T), attendu ValidationError", err, err)
			}
			if _, named := invalid.Fields[c.champ]; !named {
				t.Errorf("le champ %q doit être désigné, obtenu %v", c.champ, invalid.Fields)
			}
		})
	}
}

/*
TestSetDaemonAccepteLesAdressesAutoHebergees est la moitié qui compte.

Un garde-fou SSRF qui refuserait `amuled:4712` ou `192.168.1.10` casserait
l'installation type — c'est-à-dire tout le public du projet.
*/
func TestSetDaemonAccepteLesAdressesAutoHebergees(t *testing.T) {
	for _, host := range []string{"amuled", "192.168.1.10", "127.0.0.1", "nas.local"} {
		t.Run(host, func(t *testing.T) {
			svc := newTestService(t, &fakeRepo{}, enabled())

			if _, err := svc.SetDaemon(context.Background(), SetDaemonParams{
				Host: host, Port: 4712, Password: "x",
			}); err != nil {
				t.Errorf("SetDaemon(%q) refusé : %v", host, err)
			}
		})
	}
}

// TestOperationsRefuseesModuleEteint : le drapeau n'est pas décoratif.
func TestOperationsRefuseesModuleEteint(t *testing.T) {
	svc := newTestService(t, &fakeRepo{}, Options{Enabled: false})
	ctx := context.Background()

	if _, err := svc.Daemon(ctx); !errors.Is(err, ErrDisabled) {
		t.Errorf("Daemon : erreur = %v, attendu ErrDisabled", err)
	}
	if _, err := svc.SetDaemon(ctx, SetDaemonParams{
		Host: "amuled", Port: 4712, Password: "x",
	}); !errors.Is(err, ErrDisabled) {
		t.Errorf("SetDaemon : erreur = %v, attendu ErrDisabled", err)
	}
	if err := svc.ForgetDaemon(ctx); !errors.Is(err, ErrDisabled) {
		t.Errorf("ForgetDaemon : erreur = %v, attendu ErrDisabled", err)
	}
}

func TestForgetDaemonEstIdempotent(t *testing.T) {
	svc := newTestService(t, &fakeRepo{}, enabled())
	ctx := context.Background()

	if err := svc.ForgetDaemon(ctx); err != nil {
		t.Fatalf("oublier un démon absent ne doit pas échouer : %v", err)
	}
	if _, err := svc.Daemon(ctx); !errors.Is(err, ErrNotConfigured) {
		t.Errorf("erreur = %v, attendu ErrNotConfigured", err)
	}
}

// TestValidationErrorEstOrdonnee : ce message finit dans les journaux, et deux
// traces doivent rester comparables.
func TestValidationErrorEstOrdonnee(t *testing.T) {
	err := ValidationError{Fields: map[string]string{
		"port": "hors bornes", "host": "obligatoire", "password": "obligatoire",
	}}

	for range 5 {
		if got := err.Error(); got != err.Error() {
			t.Fatal("le message change d'un appel à l'autre")
		}
	}
	if !strings.Contains(err.Error(), "host : obligatoire") {
		t.Errorf("message = %q, attendu les champs nommés", err.Error())
	}
}

/*
rendu sérialise une valeur des deux façons dont un secret pourrait s'échapper.

En JSON, parce que c'est ce que l'API écrit. Et en `%+v`, parce qu'un champ non
exporté ne partirait pas en JSON mais atterrirait dans un journal — les deux
chemins comptent, et un test qui n'en couvrirait qu'un donnerait une fausse
assurance.
*/
func rendu(t *testing.T, v any) string {
	t.Helper()

	body, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("sérialisation : %v", err)
	}
	return string(body) + " " + fmt.Sprintf("%+v", v)
}

// ─── Pont vers la bibliothèque ───────────────────────────────────────────────
//
// Le dépôt doublé tient les destinations et les publications en mémoire. Cela
// suffit à exercer le pont : ce qu'il faut vérifier est sa LOGIQUE — qui est
// publié, qui est laissé sur disque, et qu'un fichier ne l'est pas deux fois —
// pas la façon dont PostgreSQL range les lignes.

func (f *fakeRepo) ListDestinations(context.Context) ([]Destination, error) {
	out := make([]Destination, 0, len(f.destinations))
	for _, d := range f.destinations {
		out = append(out, d)
	}
	return out, nil
}

func (f *fakeRepo) GetDestination(_ context.Context, category int) (Destination, error) {
	d, ok := f.destinations[category]
	if !ok {
		return Destination{}, ErrNoDestination
	}
	return d, nil
}

func (f *fakeRepo) SaveDestination(_ context.Context, d Destination) (Destination, error) {
	if f.destinations == nil {
		f.destinations = map[int]Destination{}
	}
	f.destinations[d.Category] = d
	return d, nil
}

func (f *fakeRepo) ClaimPublication(_ context.Context, p Publication) (bool, error) {
	if f.publications == nil {
		f.publications = map[string]Publication{}
	}
	if _, exists := f.publications[p.Hash]; exists {
		return false, nil
	}
	p.Status = PublicationPending
	f.publications[p.Hash] = p
	return true, nil
}

func (f *fakeRepo) SetPublicationResult(_ context.Context, hash string, result Publication) error {
	p := f.publications[hash]
	p.Status = result.Status
	p.Detail = result.Detail
	p.LibraryID = result.LibraryID
	p.ComicID = result.ComicID
	f.publications[hash] = p
	return nil
}

func (f *fakeRepo) ListPublications(context.Context, int) ([]Publication, error) {
	out := make([]Publication, 0, len(f.publications))
	for _, p := range f.publications {
		out = append(out, p)
	}
	return out, nil
}
