package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/adonko3xBitters/boxincloud/server/internal/amule"
	"github.com/adonko3xBitters/boxincloud/server/internal/amule/ec"
	"github.com/adonko3xBitters/boxincloud/server/internal/app"
	"github.com/adonko3xBitters/boxincloud/server/internal/archive"
	"github.com/adonko3xBitters/boxincloud/server/internal/config"
	"github.com/adonko3xBitters/boxincloud/server/internal/indexer"
	"github.com/adonko3xBitters/boxincloud/server/internal/library"
	"github.com/adonko3xBitters/boxincloud/server/internal/platform/db"
	"github.com/adonko3xBitters/boxincloud/server/internal/platform/jobs"
	"github.com/adonko3xBitters/boxincloud/server/internal/storage"
)

type commands struct {
	core *app.Core
	pool *db.Pool
	cfg  *config.Config
	log  *slog.Logger
}

// ─── Diagnostic ──────────────────────────────────────────────────────────────

func (c *commands) pingJob(ctx context.Context, args []string) error {
	message := "ping"
	if len(args) > 0 {
		message = strings.Join(args, " ")
	}

	if err := c.core.Jobs.Insert(ctx, jobs.PingArgs{Message: message}); err != nil {
		return fmt.Errorf("insertion du job : %w", err)
	}

	fmt.Printf("Job 'ping' enfilé (%q).\n", message)
	fmt.Println("Vérifiez les logs du serveur : il doit apparaître sous une seconde.")
	return nil
}

// ─── Stockage ────────────────────────────────────────────────────────────────

func (c *commands) storage(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("usage : boxincloudctl storage <add|list|test> …")
	}

	switch args[0] {
	case "add":
		return c.storageAdd(ctx, args[1:])
	case "list":
		return c.storageList(ctx)
	case "test":
		return c.storageTest(ctx, args[1:])
	default:
		return fmt.Errorf("sous-commande inconnue : storage %s", args[0])
	}
}

func (c *commands) storageAdd(ctx context.Context, args []string) error {
	if len(args) < 2 {
		return errors.New("usage : boxincloudctl storage add <nom> <s3|local> [clé=valeur …]")
	}

	name := args[0]
	kind := storage.Kind(args[1])

	opts, err := parseKeyValues(args[2:])
	if err != nil {
		return err
	}

	// Les identifiants sont séparés du reste : ils seront chiffrés en base et
	// n'en ressortiront jamais.
	config, secrets := map[string]string{}, map[string]string{}
	for k, v := range opts {
		switch k {
		case "access_key", "secret_key", "password":
			secrets[k] = v
		default:
			config[k] = v
		}
	}

	backend, err := c.core.Libraries.CreateBackend(ctx, library.CreateBackendParams{
		Name:      name,
		Kind:      kind,
		Config:    config,
		Secrets:   secrets,
		IsDefault: parseBoolOr(opts["default"], false),
		ReadOnly:  parseBoolOr(opts["read_only"], false),
	})
	if err != nil {
		return err
	}

	fmt.Printf("Backend %q créé (%s) — testé et joignable.\n", backend.Name, backend.Kind)
	fmt.Printf("  id : %s\n", backend.ID)
	return nil
}

func (c *commands) storageList(ctx context.Context) error {
	backends, err := c.core.Libraries.ListBackends(ctx)
	if err != nil {
		return err
	}
	if len(backends) == 0 {
		fmt.Println("Aucun backend enregistré. Voir : boxincloudctl storage add --help")
		return nil
	}

	fmt.Printf("%-22s %-8s %-10s %s\n", "NOM", "TYPE", "ÉTAT", "EMPLACEMENT")
	for _, b := range backends {
		location := b.Config["bucket"]
		if location == "" {
			location = b.Config["root"]
		}
		if endpoint := b.Config["endpoint"]; endpoint != "" {
			location = endpoint + "/" + location
		}

		mark := ""
		if b.IsDefault {
			mark = " (défaut)"
		}
		fmt.Printf("%-22s %-8s %-10s %s%s\n", b.Name, b.Kind, b.Status, location, mark)
	}
	return nil
}

func (c *commands) storageTest(ctx context.Context, args []string) error {
	if len(args) < 1 {
		return errors.New("usage : boxincloudctl storage test <nom>")
	}

	backend, err := c.core.Libraries.GetBackendByName(ctx, args[0])
	if err != nil {
		return err
	}
	if err := c.core.Libraries.TestBackend(ctx, backend.ID); err != nil {
		return fmt.Errorf("le backend %q ne répond pas : %w", backend.Name, err)
	}

	fmt.Printf("Backend %q : ok\n", backend.Name)
	return nil
}

// ─── Bibliothèques ───────────────────────────────────────────────────────────

func (c *commands) library(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("usage : boxincloudctl library <add|list> …")
	}

	switch args[0] {
	case "add":
		return c.libraryAdd(ctx, args[1:])
	case "list":
		return c.libraryList(ctx)
	default:
		return fmt.Errorf("sous-commande inconnue : library %s", args[0])
	}
}

func (c *commands) libraryAdd(ctx context.Context, args []string) error {
	if len(args) < 2 {
		return errors.New("usage : boxincloudctl library add <nom> <backend> [préfixe]")
	}

	backend, err := c.core.Libraries.GetBackendByName(ctx, args[1])
	if err != nil {
		return err
	}

	prefix := ""
	if len(args) > 2 {
		prefix = args[2]
	}

	lib, err := c.core.Libraries.CreateLibrary(ctx, library.CreateLibraryParams{
		Name:       args[0],
		BackendID:  backend.ID,
		RootPrefix: prefix,
	})
	if err != nil {
		return err
	}

	fmt.Printf("Bibliothèque %q créée sur le backend %q.\n", lib.Name, backend.Name)
	fmt.Printf("  id : %s\n", lib.ID)
	fmt.Printf("Lancez le scan : boxincloudctl scan-now %s\n", lib.Name)
	return nil
}

func (c *commands) libraryList(ctx context.Context) error {
	libs, err := c.core.Libraries.ListLibraries(ctx)
	if err != nil {
		return err
	}
	if len(libs) == 0 {
		fmt.Println("Aucune bibliothèque. Voir : boxincloudctl library add")
		return nil
	}

	fmt.Printf("%-22s %-10s %-8s %s\n", "NOM", "TYPE", "ALBUMS", "PRÉFIXE")
	for _, l := range libs {
		fmt.Printf("%-22s %-10s %-8d %s\n", l.Name, l.Kind, l.ComicCount, l.RootPrefix)
	}
	return nil
}

// ─── Scan ────────────────────────────────────────────────────────────────────

func (c *commands) scan(ctx context.Context, args []string) error {
	if len(args) < 1 {
		return errors.New("usage : boxincloudctl scan <bibliothèque>")
	}

	lib, err := c.core.Libraries.GetLibraryByName(ctx, args[0])
	if err != nil {
		return err
	}
	if err := c.core.ScanLibrary(ctx, lib.ID); err != nil {
		return err
	}

	fmt.Printf("Scan de %q enfilé. Le serveur doit tourner pour le traiter.\n", lib.Name)
	fmt.Println("Sans serveur, utilisez : boxincloudctl scan-now")
	return nil
}

// scanNow exécute le pipeline en direct, sans passer par la file.
//
// Rend le scan observable depuis un terminal : on voit le parcours, puis
// l'indexation de chaque album. C'est l'outil de démonstration et de
// diagnostic de M1.
func (c *commands) scanNow(ctx context.Context, args []string) error {
	if len(args) < 1 {
		return errors.New("usage : boxincloudctl scan-now <bibliothèque>")
	}

	lib, err := c.core.Libraries.GetLibraryByName(ctx, args[0])
	if err != nil {
		return err
	}

	start := time.Now()
	fmt.Printf("Scan de %q…\n\n", lib.Name)

	runner := indexer.NewDirectRunner(indexer.Deps{
		Libraries: c.core.Libraries,
		Repo:      c.core.Indexer,
		Cache:     c.core.Cache,
		Imaging:   c.core.Imaging,
		Log:       c.log,
	})

	stats, err := runner.ScanAndIndex(ctx, lib.ID)
	if err != nil {
		return err
	}

	fmt.Printf("\nTerminé en %s\n", time.Since(start).Round(time.Millisecond))
	fmt.Printf("  objets vus  : %d\n", stats.ObjectsSeen)
	fmt.Printf("  ajoutés     : %d\n", stats.Added)
	fmt.Printf("  modifiés    : %d\n", stats.Updated)
	fmt.Printf("  disparus    : %d\n", stats.Removed)
	fmt.Printf("  erreurs     : %d\n", stats.Errors)
	return nil
}

// ─── Lecture d'une page ──────────────────────────────────────────────────────

// page extrait une page et mesure ce que cela a coûté.
//
// C'est la démonstration directe de la promesse du projet : le nombre de
// requêtes Range et d'octets transférés est affiché, à comparer à la taille de
// l'archive.
func (c *commands) page(ctx context.Context, args []string) error {
	if len(args) < 3 {
		return errors.New("usage : boxincloudctl page <bibliothèque> <clé> <n> [fichier de sortie]")
	}

	lib, err := c.core.Libraries.GetLibraryByName(ctx, args[0])
	if err != nil {
		return err
	}

	key := args[1]
	index, err := strconv.Atoi(args[2])
	if err != nil {
		return fmt.Errorf("numéro de page invalide : %q", args[2])
	}

	provider, err := c.core.Libraries.ProviderForLibrary(ctx, lib)
	if err != nil {
		return err
	}

	info, err := provider.Stat(ctx, key)
	if err != nil {
		return err
	}

	counter := newRangeCounter(provider)

	idx, err := archive.ReadZipIndex(ctx, counter, key, info.Size)
	if err != nil {
		return err
	}
	indexCalls, indexBytes := counter.calls, counter.bytes

	if index < 0 || index >= len(idx.Pages) {
		return fmt.Errorf("page %d hors limites : l'archive en compte %d", index, len(idx.Pages))
	}

	counter.reset()

	r, err := archive.OpenEntry(ctx, counter, key, idx.Pages[index])
	if err != nil {
		return err
	}
	defer func() { _ = r.Close() }()

	out := io.Discard
	dest := "(non enregistrée)"
	if len(args) > 3 {
		// Chemin de sortie fourni en argument par l'opérateur qui lance la
		// commande : il écrit déjà où il veut avec son propre shell.
		// #nosec G304 G703
		f, err := os.Create(filepath.Clean(args[3]))
		if err != nil {
			return err
		}
		defer func() { _ = f.Close() }()
		out = f
		dest = args[3]
	}

	written, err := io.Copy(out, r)
	if err != nil {
		return err
	}

	pct := func(n int64) float64 { return float64(n) / float64(info.Size) * 100 }

	fmt.Printf("Archive      : %s (%s, %d pages)\n", key, humanBytes(info.Size), len(idx.Pages))
	fmt.Printf("Entrée       : %s\n", idx.Pages[index].Name)
	fmt.Printf("Page écrite  : %s (%s)\n\n", dest, humanBytes(written))
	fmt.Printf("Indexation   : %d requêtes Range, %s transférés (%.2f %% de l'archive)\n",
		indexCalls, humanBytes(indexBytes), pct(indexBytes))
	fmt.Printf("Lecture page : %d requête Range, %s transférés (%.2f %% de l'archive)\n",
		counter.calls, humanBytes(counter.bytes), pct(counter.bytes))
	return nil
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// rangeCounter instrumente un provider pour mesurer le coût réel d'une lecture.
type rangeCounter struct {
	storage.Provider
	calls int
	bytes int64
}

func newRangeCounter(p storage.Provider) *rangeCounter {
	return &rangeCounter{Provider: p}
}

func (c *rangeCounter) reset() {
	c.calls = 0
	c.bytes = 0
}

func (c *rangeCounter) ReadRange(ctx context.Context, key string, offset, length int64) (io.ReadCloser, error) {
	r, err := c.Provider.ReadRange(ctx, key, offset, length)
	if err != nil {
		return nil, err
	}
	c.calls++
	return &countingReader{ReadCloser: r, counter: c}, nil
}

type countingReader struct {
	io.ReadCloser
	counter *rangeCounter
}

func (r *countingReader) Read(p []byte) (int, error) {
	n, err := r.ReadCloser.Read(p)
	r.counter.bytes += int64(n)
	return n, err
}

func parseKeyValues(args []string) (map[string]string, error) {
	out := make(map[string]string, len(args))
	for _, arg := range args {
		k, v, ok := strings.Cut(arg, "=")
		if !ok {
			return nil, fmt.Errorf("argument attendu sous la forme clé=valeur, reçu %q", arg)
		}
		out[strings.TrimSpace(k)] = v
	}
	return out, nil
}

func parseBoolOr(s string, def bool) bool {
	if s == "" {
		return def
	}
	b, err := strconv.ParseBool(s)
	if err != nil {
		return def
	}
	return b
}

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d o", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %co", float64(n)/float64(div), "kMGTP"[exp])
}

// ─── Comptes ─────────────────────────────────────────────────────────────────

/*
Gestion des comptes en ligne de commande.

# Pourquoi elle existe

Sans elle, un administrateur qui oublie son mot de passe est enfermé dehors
DÉFINITIVEMENT. Il n'y a ni courriel de récupération — une instance
auto-hébergée n'a pas forcément de serveur de messagerie — ni second
administrateur garanti, puisque l'assistant d'installation n'en crée qu'un.

Le seul recours était d'écrire un hachage argon2id à la main dans PostgreSQL.
Personne ne fait ça correctement du premier coup, et s'y tromper verrouille le
compte pour de bon.

# Pourquoi le mot de passe ne s'écrit pas dans la commande

Il est lu sur l'entrée standard, jamais passé en argument. Un argument atterrit
dans l'historique du shell, dans la sortie de `ps` pendant l'exécution, et dans
les journaux de tout ce qui enregistre les commandes. Trois fuites pour un
confort de frappe.
*/
func (c *commands) user(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("usage : boxincloudctl user <list|set-password>")
	}

	switch args[0] {
	case "list":
		return c.userList(ctx)
	case "set-password":
		return c.userSetPassword(ctx, args[1:])
	default:
		return fmt.Errorf("sous-commande inconnue : %s", args[0])
	}
}

func (c *commands) userList(ctx context.Context) error {
	users, err := c.core.Queries.ListUsers(ctx)
	if err != nil {
		return fmt.Errorf("lecture des comptes : %w", err)
	}
	if len(users) == 0 {
		fmt.Println("Aucun compte. Ouvrez l'interface web : l'assistant crée le premier.")
		return nil
	}

	fmt.Printf("%-24s %-8s %s\n", "COMPTE", "RÔLE", "DERNIÈRE CONNEXION")
	for _, u := range users {
		last := "jamais"
		if u.LastLoginAt.Valid {
			last = u.LastLoginAt.Time.Format("2006-01-02 15:04")
		}
		fmt.Printf("%-24s %-8s %s\n", u.Username, u.Role, last)
	}
	return nil
}

/*
userSetPassword change le mot de passe d'un compte.

Les sessions ouvertes sont révoquées. C'est le point : on ne change pas un mot
de passe pour le plaisir, mais parce qu'on l'a perdu ou qu'il a fuité. Dans le
second cas, laisser vivre les jetons existants rendrait l'opération inutile —
celui qui avait l'accès le garderait.
*/
func (c *commands) userSetPassword(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("usage : boxincloudctl user set-password <compte>")
	}
	username := args[0]

	account, err := c.core.Queries.GetUserByUsername(ctx, username)
	if err != nil {
		return fmt.Errorf("compte introuvable : %s", username)
	}

	fmt.Fprintf(os.Stderr, "Nouveau mot de passe pour %s (saisie sur l'entrée standard) : ", username)

	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		return fmt.Errorf("lecture du mot de passe : %w", err)
	}
	password := strings.TrimRight(line, "\r\n")

	if err := c.core.Accounts.ResetPassword(ctx, account.ID, password); err != nil {
		return err
	}

	revoked, err := c.core.Queries.RevokeAllUserSessions(ctx, account.ID)
	if err != nil {
		return fmt.Errorf("mot de passe changé, mais sessions non révoquées : %w", err)
	}
	_ = revoked

	fmt.Printf("→ mot de passe de %s changé, sessions révoquées\n", username)
	return nil
}

// ─── Module eD2k/Kad ─────────────────────────────────────────────────────────

func (c *commands) ed2k(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("usage : boxincloudctl ed2k <ping>")
	}

	switch args[0] {
	case "ping":
		return c.ed2kPing(ctx)
	default:
		return errors.New("sous-commande inconnue : " + args[0])
	}
}

/*
ed2kPing joint le démon aMule et mesure l'aller-retour.

C'est l'outil de diagnostic du module, et il existe pour une raison précise :
quand l'interface affiche « démon injoignable », il faut pouvoir savoir si le
problème est l'adresse, le mot de passe, la version du protocole ou le réseau —
et le savoir depuis le serveur, là où la connexion part réellement, plutôt que
depuis un navigateur qui n'en voit que le résultat.
*/
func (c *commands) ed2kPing(ctx context.Context) error {
	result, err := c.core.Ed2k.Ping(ctx)

	// L'adresse est affichée même en cas d'échec : la moitié des problèmes de
	// connexion sont une adresse qui n'est pas celle qu'on croyait.
	if result.Address != "" {
		fmt.Printf("démon        %s\n", result.Address)
	}

	if err != nil {
		// L'indice s'imprime, il n'entre pas dans l'erreur.
		//
		// Une erreur est une phrase que d'autres enveloppent et journalisent ;
		// y coller trois lignes d'explication les rend illisibles partout
		// ailleurs. L'aide, elle, ne concerne que la personne devant ce
		// terminal.
		switch {
		case errors.Is(err, amule.ErrDisabled):
			fmt.Println("\nPassez BOXINCLOUD_ED2K_ENABLED à true, puis relancez.")

		case errors.Is(err, amule.ErrNotConfigured):
			fmt.Println("\nDéclarez le démon depuis l'interface, page eD2k / Kad.")

		case errors.Is(err, ec.ErrProtocolVersion):
			fmt.Printf("\nCe client parle la version 0x%04X du protocole External Connections.\n"+
				"amuled exige une correspondance EXACTE, pas une compatibilité ascendante :\n"+
				"une version différente des deux côtés ne se contourne pas, elle se met à jour.\n",
				ec.ProtocolVersion)

		case errors.Is(err, ec.ErrAuthFailed):
			fmt.Println("\nRappel : dans amule.conf, ECPassword n'est PAS le mot de passe en\n" +
				"clair, c'est son empreinte MD5. Le mot de passe déclaré ici est celui\n" +
				"en clair.")
		}
		return err
	}

	fmt.Printf("version      %s\n", result.ServerVersion)
	fmt.Printf("protocole    0x%04X\n", result.ProtocolVersion)
	fmt.Printf("poignée      %s\n", result.Handshake.Round(time.Millisecond))
	fmt.Printf("aller-retour %s\n", result.RoundTrip.Round(time.Microsecond))

	return nil
}
