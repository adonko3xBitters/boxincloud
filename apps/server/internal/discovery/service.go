package discovery

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/google/uuid"

	"github.com/adonko3xBitters/boxincloud/server/internal/platform/crypto"
)

/*
Service de recherche fédérée.

Le principe tient en une phrase : interroger tous les catalogues activés en
parallèle, et rendre ce qui est revenu à temps.

La partie qui demande de la retenue est la gestion des échecs. Une recherche
fédérée réussit PARTIELLEMENT par nature — un catalogue est éteint, un autre
répond en douze secondes, un troisième a changé d'URL. Traiter cela comme une
erreur donnerait une fonctionnalité qui ne marche jamais, puisqu'il suffit d'un
catalogue en panne sur cinq pour tout perdre.

La réponse porte donc deux choses : les résultats, et l'état de chaque
catalogue. L'interface montre les uns et signale les autres, ce qui est aussi
la seule façon pour l'utilisateur de comprendre qu'il manque quelque chose.
*/

// Repository est ce dont le service a besoin de la base.
type Repository interface {
	ListSources(ctx context.Context) ([]Source, error)
	GetSource(ctx context.Context, id uuid.UUID) (Source, error)
	// SourceSecret retourne le mot de passe chiffré, ou nil si le catalogue
	// est public.
	SourceSecret(ctx context.Context, id uuid.UUID) ([]byte, error)
	CreateSource(ctx context.Context, s Source, secret []byte) (Source, error)
	UpdateSource(ctx context.Context, s Source, secret []byte, replaceSecret bool) (Source, error)
	DeleteSource(ctx context.Context, id uuid.UUID) error
	RecordProbe(ctx context.Context, id uuid.UUID, failure string) error

	CreateImport(ctx context.Context, i Import) (Import, error)
	GetImport(ctx context.Context, id uuid.UUID) (Import, error)
	ListImports(ctx context.Context, limit int) ([]Import, error)
	StartImport(ctx context.Context, id uuid.UUID) error
	FinishImport(ctx context.Context, id uuid.UUID, d Deposited) error
	FailImport(ctx context.Context, id uuid.UUID, code, detail string) error
}

/*
LocalCatalog rend les titres que l'instance possède déjà et que CE lecteur a le
droit de voir.

Une fonction plutôt qu'une interface, et fournie à chaque recherche plutôt que
gardée dans le service : elle est nécessairement liée au lecteur courant.

C'est une question de droits, pas de style. Le catalogue local est filtré par
profil — dossiers verrouillés, limite d'âge. Un service qui garderait une vue
non liée marquerait « déjà dans votre bibliothèque » sur un titre qu'un profil
restreint n'a pas le droit de voir, et lui apprendrait ainsi son existence.
Passer la vue en paramètre rend l'oubli impossible.
*/
type LocalCatalog func(ctx context.Context, text string, limit int) ([]string, error)

/*
ImportQueue enfile un import.

Une seule opération, plutôt que le client de jobs entier : ce service n'a aucune
raison de pouvoir enfiler autre chose, et une interface d'une méthode se double
en trois lignes dans un test.
*/
type ImportQueue interface {
	EnqueueImport(ctx context.Context, importID uuid.UUID) error
}

type Service struct {
	repo   Repository
	client Client
	queue  ImportQueue
	sealer *crypto.Sealer
	log    *slog.Logger

	/*
		clients associe un genre de catalogue à son client.

		Vide au départ, et c'est le cas normal : `client` reste le client par
		défaut, celui d'OPDS, et une source qui ne réclame rien d'autre retombe
		sur lui. Le comportement d'origine tient donc tant que rien n'est
		enregistré ici.

		Une table plutôt qu'un `switch` : le service n'a pas à connaître les
		genres qui existent, et un paquet qui en ajoute un — `discovery/scraper`
		— n'a pas à être importé ici, ce qui ferait un cycle.
	*/
	clients map[Kind]Client
	// kinds décrit ces mêmes genres, pour que l'administration puisse les
	// proposer plutôt que les faire saisir.
	kinds map[Kind]KindInfo

	// comics applique les métadonnées enrichies. Nul tant qu'il n'est pas
	// branché, auquel cas l'import n'enrichit rien.
	comics ComicWriter

	// registry porte les bases de métadonnées. Nul tant qu'aucune n'est
	// enregistrée, auquel cas le rapprochement rend simplement une liste vide —
	// une instance sans accès Internet reste utilisable.
	registry *Registry
}

func NewService(
	repo Repository,
	client Client,
	sealer *crypto.Sealer,
	log *slog.Logger,
) *Service {
	return &Service{repo: repo, client: client, sealer: sealer, log: log}
}

/*
SetImportQueue renseigne la file, après coup.

Le service et la file se connaissent mutuellement — la file exécute un worker
qui appelle le service, le service enfile dans la file. La même indirection que
pour l'indexeur casse le cycle au câblage.
*/
func (s *Service) SetImportQueue(queue ImportQueue) { s.queue = queue }

/*
KindInfo décrit un genre de catalogue enregistré.

Existe pour une raison précise : l'écran de configuration doit pouvoir PROPOSER
les genres disponibles, plutôt que demander à l'administrateur de taper
`scraper:comicshelf` de mémoire. Sans cette description, l'interface devrait
connaître la convention de nommage des gabarits — donc dépendre d'un détail du
serveur qui ne la regarde pas.

Les champs au-delà du genre sont facultatifs. Un protocole n'a ni page d'accueil
ni miroirs ; un gabarit a les deux.
*/
type KindInfo struct {
	Kind Kind
	// ID est l'identifiant nu, sans préfixe de genre. Vide pour un protocole.
	ID       string
	Name     string
	Homepage string
	License  string
	Mirrors  []string
}

/*
RegisterClient associe un client à un genre de catalogue.

Appelé au câblage, avant que le service ne serve quoi que ce soit — les tables
ne sont pas protégées par un verrou, et ne doivent donc plus bouger une fois les
requêtes ouvertes. C'est la même discipline que pour le registre des bases de
métadonnées, et pour la même raison : un enregistrement à chaud serait une
donnée partagée écrite pendant qu'on la lit, pour un besoin qui n'existe pas.
*/
func (s *Service) RegisterClient(info KindInfo, client Client) {
	if s.clients == nil {
		s.clients = map[Kind]Client{}
		s.kinds = map[Kind]KindInfo{}
	}
	s.clients[info.Kind] = client
	s.kinds[info.Kind] = info
}

// RegisteredKinds rend les genres enregistrés, dans un ordre stable.
//
// N'inclut pas OPDS : celui-ci est le client par défaut, pas un genre
// enregistré. La distinction n'est pas cosmétique — OPDS n'est pas un choix
// dans une liste, c'est ce qu'on obtient quand on ne choisit rien.
func (s *Service) RegisteredKinds() []KindInfo {
	out := make([]KindInfo, 0, len(s.kinds))
	for _, info := range s.kinds {
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Kind < out[j].Kind })
	return out
}

/*
clientFor choisit le client d'un catalogue.

OPDS est le repli, pas une entrée du registre : c'est ce qu'on obtient quand on
ne choisit rien, et les sources créées avant que d'autres genres n'existent
n'ont donc rien à rattraper.

Un genre EXPLICITE mais absent du registre est une erreur, et il faut résister à
la tentation de le faire retomber sur OPDS. Le cas se produit vraiment : un
opérateur retire un gabarit de son répertoire, et les sources qui s'en servaient
restent en base. Les traiter en OPDS ferait demander un flux Atom à une page
HTML, pour un échec dont le message ne dirait rien de la vraie cause.
*/
func (s *Service) clientFor(source Source) (Client, error) {
	if client, ok := s.clients[source.Kind]; ok {
		return client, nil
	}
	if source.Kind != "" && source.Kind != KindOPDS {
		return nil, fmt.Errorf("%w : genre de catalogue inconnu (%s)",
			ErrUnknownProvider, source.Kind)
	}
	return s.client, nil
}

// ─── Administration des catalogues ───────────────────────────────────────────

func (s *Service) List(ctx context.Context) ([]Source, error) {
	return s.repo.ListSources(ctx)
}

// CreateParams décrit un catalogue à enregistrer.
type CreateParams struct {
	Name string
	URL  string
	// Kind vaut OPDS quand il n'est pas renseigné, ce qui reste le cas de
	// l'écrasante majorité des catalogues ajoutés à la main.
	Kind     Kind
	Enabled  bool
	Username string
	Password string
	/*
		Template porte les règles d'extraction d'une source `web`, en JSON.

		Le service ne les interprète pas — il ne connaît pas les sélecteurs CSS.
		C'est le client du genre qui les compile, et son refus remonte par
		l'essai, avant l'enregistrement. Un formulaire fautif ne crée donc
		jamais de source.
	*/
	Template []byte
}

/*
Create enregistre un catalogue après l'avoir joint.

L'essai n'est pas une politesse. Une URL de catalogue se recopie depuis une
interface tierce, et se trompe souvent d'un segment — `/opds` au lieu de
`/opds/v1.2`. Sans essai, l'erreur ne se voit qu'à la première recherche, où
elle passe pour « aucun résultat ».
*/
func (s *Service) Create(ctx context.Context, p CreateParams) (Source, error) {
	kind := p.Kind
	if kind == "" {
		kind = KindOPDS
	}

	source := Source{
		Name:     strings.TrimSpace(p.Name),
		URL:      strings.TrimSpace(p.URL),
		Kind:     kind,
		Enabled:  p.Enabled,
		Username: strings.TrimSpace(p.Username),
		Template: p.Template,
	}

	// Une source `web` n'existe que par ses règles, et la base le vérifie aussi.
	// Le refuser ici donne un message utile plutôt qu'une violation de
	// contrainte.
	if kind == KindWeb && len(source.Template) == 0 {
		return Source{}, fmt.Errorf(
			"%w : une source de type site web doit décrire où lire ses résultats",
			ErrInvalidSource)
	}
	if kind != KindWeb && len(source.Template) > 0 {
		return Source{}, fmt.Errorf(
			"%w : des règles d'extraction n'ont de sens que pour un site web",
			ErrInvalidSource)
	}

	if source.Name == "" {
		return Source{}, fmt.Errorf("%w : le nom est requis", ErrInvalidSource)
	}
	/*
		Une source lue au gabarit peut n'avoir aucune adresse : celles du gabarit
		suffisent, et c'est même le cas normal — l'administrateur n'en saisit une
		que le jour où un miroir change.

		Un catalogue OPDS, lui, n'est RIEN sans son adresse : il n'y a pas de
		miroir de repli à connaître, et le protocole n'est pas lié à un site
		particulier.
	*/
	if _, templated := kind.ScraperTemplate(); !templated && kind != KindWeb && source.URL == "" {
		return Source{}, fmt.Errorf("%w : l'adresse est requise", ErrInvalidSource)
	}
	if source.URL != "" &&
		!strings.HasPrefix(source.URL, "http://") && !strings.HasPrefix(source.URL, "https://") {
		return Source{}, fmt.Errorf("%w : l'adresse doit commencer par http:// ou https://",
			ErrInvalidSource)
	}

	client, err := s.clientFor(source)
	if err != nil {
		return Source{}, fmt.Errorf("%w : %w", ErrInvalidSource, err)
	}
	if err := client.Probe(ctx, source, p.Password); err != nil {
		return Source{}, fmt.Errorf("%w : %w", ErrInvalidSource, err)
	}

	secret, err := s.seal(p.Password)
	if err != nil {
		return Source{}, err
	}
	return s.repo.CreateSource(ctx, source, secret)
}

// UpdateParams décrit une modification de catalogue.
//
// Password est distingué de son absence par PasswordSet : un formulaire qui
// renvoie un champ vide ne doit pas effacer le mot de passe enregistré, alors
// qu'un administrateur qui veut l'effacer doit pouvoir le faire.
type UpdateParams struct {
	Name        string
	URL         string
	Enabled     bool
	Username    string
	Password    string
	PasswordSet bool
	// Template ne se vide pas : absent, les règles en place sont conservées.
	Template []byte
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, p UpdateParams) (Source, error) {
	current, err := s.repo.GetSource(ctx, id)
	if err != nil {
		return Source{}, err
	}

	current.Name = strings.TrimSpace(p.Name)
	current.URL = strings.TrimSpace(p.URL)
	current.Enabled = p.Enabled
	current.Username = strings.TrimSpace(p.Username)
	if len(p.Template) > 0 {
		current.Template = p.Template
	}

	if current.Name == "" {
		return Source{}, fmt.Errorf("%w : le nom est requis", ErrInvalidSource)
	}
	// Même nuance qu'à la création : une source lue au gabarit peut rendre son
	// adresse pour retomber sur les miroirs déclarés. C'est d'ailleurs la façon
	// d'annuler un changement de miroir qu'on regrette.
	if _, templated := current.Kind.ScraperTemplate(); !templated &&
		current.Kind != KindWeb && current.URL == "" {
		return Source{}, fmt.Errorf("%w : l'adresse est requise", ErrInvalidSource)
	}

	var secret []byte
	if p.PasswordSet {
		if secret, err = s.seal(p.Password); err != nil {
			return Source{}, err
		}
	}
	return s.repo.UpdateSource(ctx, current, secret, p.PasswordSet)
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.DeleteSource(ctx, id)
}

// Probe rejoue l'essai sur un catalogue déjà enregistré et note le résultat.
func (s *Service) Probe(ctx context.Context, id uuid.UUID) error {
	source, password, err := s.credentials(ctx, id)
	if err != nil {
		return err
	}

	failure := ""
	client, err := s.clientFor(source)
	if err != nil {
		failure = err.Error()
	} else if err := client.Probe(ctx, source, password); err != nil {
		failure = err.Error()
	}

	if err := s.repo.RecordProbe(ctx, id, failure); err != nil {
		s.log.Warn("état de catalogue non enregistré",
			slog.String("source", id.String()), slog.Any("err", err))
	}
	if failure != "" {
		return fmt.Errorf("%w : %s", ErrInvalidSource, failure)
	}
	return nil
}

// ─── Recherche ───────────────────────────────────────────────────────────────

/*
Search interroge en parallèle tous les catalogues activés.

Le parallélisme n'est pas une optimisation ici, c'est la fonctionnalité : en
série, cinq catalogues à huit secondes de délai feraient quarante secondes
d'attente pour l'utilisateur le jour où ils sont tous injoignables.
*/
func (s *Service) Search(ctx context.Context, q Query, local LocalCatalog) (SearchResult, error) {
	sources, err := s.repo.ListSources(ctx)
	if err != nil {
		return SearchResult{}, err
	}

	if strings.TrimSpace(q.Text) == "" {
		return SearchResult{Results: []Result{}, Sources: []SourceStatus{}}, nil
	}

	var (
		mu       sync.Mutex
		gathered []Result
		statuses []SourceStatus
		wg       sync.WaitGroup
	)

	for _, source := range sources {
		if !source.Enabled {
			continue
		}

		wg.Add(1)
		go func(source Source) {
			defer wg.Done()

			started := time.Now()
			results, err := s.searchOne(ctx, source, q)
			status := SourceStatus{
				SourceID:  source.ID,
				Name:      source.Name,
				Count:     len(results),
				ElapsedMs: time.Since(started).Milliseconds(),
			}
			if err != nil {
				status.Error = failureCode(err)
				// Au niveau info : un catalogue tiers qui ne répond pas n'est
				// pas un défaut de cette instance, et le journaliser en erreur
				// donnerait un bruit permanent à qui fédère un service public.
				s.log.Info("catalogue sans réponse",
					slog.String("source", source.Name), slog.Any("err", err))
			}

			mu.Lock()
			gathered = append(gathered, results...)
			statuses = append(statuses, status)
			mu.Unlock()
		}(source)
	}

	wg.Wait()

	gathered = dedupe(gathered)
	s.markOwned(ctx, local, q.Text, gathered)

	// Ordre stable : sans lui, deux recherches identiques rendraient les mêmes
	// résultats dans un ordre différent, ce qui donne l'impression que la page
	// bouge toute seule.
	sort.SliceStable(gathered, func(i, j int) bool {
		if gathered[i].SourceName != gathered[j].SourceName {
			return gathered[i].SourceName < gathered[j].SourceName
		}
		return gathered[i].Title < gathered[j].Title
	})
	sort.SliceStable(statuses, func(i, j int) bool {
		return statuses[i].Name < statuses[j].Name
	})

	if gathered == nil {
		gathered = []Result{}
	}
	if statuses == nil {
		statuses = []SourceStatus{}
	}
	return SearchResult{Results: gathered, Sources: statuses}, nil
}

func (s *Service) searchOne(ctx context.Context, source Source, q Query) ([]Result, error) {
	_, password, err := s.credentials(ctx, source.ID)
	if err != nil {
		return nil, err
	}
	client, err := s.clientFor(source)
	if err != nil {
		return nil, err
	}
	return client.Search(ctx, source, password, q)
}

/*
markOwned marque les résultats que l'instance possède déjà.

Un échec ici ne fait pas échouer la recherche : ne pas savoir qu'on possède
déjà un titre est moins grave que de ne rien afficher du tout.
*/
func (s *Service) markOwned(
	ctx context.Context, local LocalCatalog, text string, results []Result,
) {
	if local == nil || len(results) == 0 {
		return
	}

	titles, err := local(ctx, text, 200)
	if err != nil {
		s.log.Warn("comparaison au catalogue local impossible", slog.Any("err", err))
		return
	}

	owned := make(map[string]bool, len(titles))
	for _, title := range titles {
		owned[normalizeTitle(title)] = true
	}
	for i := range results {
		results[i].InLibrary = owned[normalizeTitle(results[i].Title)]
	}
}

func (s *Service) credentials(ctx context.Context, id uuid.UUID) (Source, string, error) {
	source, err := s.repo.GetSource(ctx, id)
	if err != nil {
		return Source{}, "", err
	}
	if source.Username == "" {
		return source, "", nil
	}

	sealed, err := s.repo.SourceSecret(ctx, id)
	if err != nil || len(sealed) == 0 {
		return source, "", err
	}
	plain, err := s.sealer.Open(sealed)
	if err != nil {
		return Source{}, "", fmt.Errorf("identifiants de catalogue illisibles : %w", err)
	}
	return source, string(plain), nil
}

func (s *Service) seal(password string) ([]byte, error) {
	if password == "" {
		return nil, nil
	}
	sealed, err := s.sealer.Seal([]byte(password))
	if err != nil {
		return nil, fmt.Errorf("chiffrement des identifiants : %w", err)
	}
	return sealed, nil
}

// ─── Agrégation ──────────────────────────────────────────────────────────────

/*
dedupe fusionne les entrées que plusieurs catalogues rendent en double.

Le cas est fréquent dès qu'on fédère deux de ses propres instances, ou deux
bibliothèques publiques qui puisent au même fonds. Le rapprochement se fait sur
le titre normalisé et le premier auteur : plus fin serait fragile — les
catalogues n'écrivent pas les identifiants de la même façon — et plus large
fusionnerait des tomes distincts d'une même série.

La provenance est conservée : les liens d'acquisition des doublons rejoignent
l'entrée retenue, ce qui donne à l'utilisateur le choix du catalogue.
*/
func dedupe(results []Result) []Result {
	seen := make(map[string]int, len(results))
	out := make([]Result, 0, len(results))

	for _, result := range results {
		key := normalizeTitle(result.Title)
		if len(result.Authors) > 0 {
			key += "|" + normalizeTitle(result.Authors[0])
		}

		if at, ok := seen[key]; ok {
			out[at].Acquisitions = append(out[at].Acquisitions, result.Acquisitions...)
			// Un catalogue peut avoir la couverture qu'un autre n'a pas.
			if out[at].CoverURL == "" {
				out[at].CoverURL = result.CoverURL
			}
			if out[at].Summary == "" {
				out[at].Summary = result.Summary
			}
			continue
		}

		seen[key] = len(out)
		out = append(out, result)
	}
	return out
}

/*
normalizeTitle ramène un titre à ce qui permet de le comparer.

Casse, accents, ponctuation et espaces multiples sautent : « L'Incal » et
« l'incal » désignent la même œuvre, et deux catalogues ne les écrivent presque
jamais pareil.
*/
func normalizeTitle(title string) string {
	var builder strings.Builder
	builder.Grow(len(title))

	space := false
	for _, r := range strings.ToLower(title) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			if space && builder.Len() > 0 {
				builder.WriteByte(' ')
			}
			space = false
			builder.WriteString(foldRune(r))
		default:
			space = true
		}
	}
	return builder.String()
}

/*
foldRune ramène une lettre à sa forme comparable.

Une table plutôt qu'une dépendance de normalisation Unicode : les alphabets
concernés par les catalogues qu'on fédère tiennent en quelques lignes, et la
table se lit d'un coup d'œil.

Le retour est une CHAÎNE et non une rune, ce qui n'est pas un détail. Les
ligatures s'étendent en deux lettres : « Mœbius » s'écrit « Moebius » partout où
le clavier n'a pas la ligature, et les replier sur une seule lettre donnait
« mobius » — qui ne rejoignait ni l'une ni l'autre.

Le défaut se voyait mal : il ne cassait rien visiblement, il empêchait seulement
deux catalogues de reconnaître le même auteur, et laissait passer des doublons
qu'on croyait dédupliqués.
*/
func foldRune(r rune) string {
	// Ligatures et lettres composées : une lettre en entrée, deux en sortie.
	switch r {
	case 'œ':
		return "oe"
	case 'æ':
		return "ae"
	case 'ß':
		return "ss"
	}

	const (
		accented = "àáâãäåçèéêëìíîïñòóôõöùúûüýÿ"
		plain    = "aaaaaaceeeeiiiinooooouuuuyy"
	)
	if at := strings.IndexRune(accented, r); at >= 0 {
		return string(plain[len([]rune(accented[:at]))])
	}
	return string(r)
}

/*
failureCode traduit un échec en code stable.

L'interface le traduit en français ou en anglais ; le serveur, lui, n'a pas à
deviner la langue du lecteur. Même règle que pour les problèmes RFC 7807.
*/
func failureCode(err error) string {
	switch {
	case errors.Is(err, ErrNoSearch):
		return "no-search"
	case errors.Is(err, ErrUnknownProvider):
		// Distingué d'« invalid » : la source est bien formée, c'est le
		// gabarit ou le protocole qu'elle réclame qui n'est plus chargé. La
		// correction est côté exploitation, pas côté saisie.
		return "unknown-kind"
	case errors.Is(err, ErrInvalidSource):
		return "invalid"
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.Is(err, context.Canceled):
		return "canceled"
	default:
		return "unreachable"
	}
}
