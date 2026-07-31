package discovery

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/adonko3xBitters/boxincloud/server/internal/platform/netguard"
)

/*
Import d'un résultat vers une bibliothèque.

Une recherche qui trouve sans permettre de rapatrier n'est qu'une liste de
liens. L'import ferme la boucle : le serveur télécharge chez le catalogue et
dépose dans le backend de stockage choisi, sans que le fichier transite par le
navigateur de l'utilisateur — ce qui compte quand l'instance a une bien
meilleure liaison que le téléphone qui la pilote.

# La contrainte qui gouverne tout le reste

Le client envoie l'adresse à télécharger. C'est donc, tel quel, un serveur qui
va chercher une URL fournie par un tiers : la définition d'une SSRF, et pire
encore d'un relais anonyme — n'importe qui pourrait faire télécharger n'importe
quoi par l'instance, depuis l'intérieur de son réseau.

D'où la règle : **l'adresse doit appartenir à un catalogue déclaré.** Même
schéma, même hôte, même port qu'une source enregistrée par un administrateur.
Ce n'est pas une vérification de forme, c'est ce qui confine le téléchargement
au périmètre que l'administration a ouvert.

Le reste des garde-fous est hérité de l'ingestion, et c'est délibéré : borne de
taille appliquée au flux, signature du contenu vérifiée avant écriture, refus
d'écraser un objet existant, contrôle d'écriture sur le dossier de destination.
Réécrire ces règles ici les aurait fait diverger.

# Pourquoi une tâche de fond, et où passe la frontière

Un import dure le temps d'un téléchargement : quelques secondes pour un album,
plusieurs minutes pour une intégrale servie par un catalogue lointain. Le tenir
dans une requête HTTP obligeait le navigateur à garder une connexion ouverte
tout du long, et faisait perdre l'import à qui fermait son onglet.

La demande est donc enregistrée puis enfilée, et la requête rend 202.

Ce découpage impose de trancher ce qui est vérifié tout de suite et ce qui ne
peut l'être qu'au moment du téléchargement. La règle suivie ici : **tout ce qui
ne demande pas le réseau est vérifié avant de rendre 202.** Catalogue inconnu,
adresse étrangère, bibliothèque inaccessible — ces refus-là sont immédiats et
portent un code d'erreur HTTP, parce qu'ils sont dus à la demande elle-même.

Ce qui dépend du catalogue distant — il ne répond pas, il sert un format qu'on
refuse, le fichier existe déjà — ne peut être découvert qu'en essayant. Ces
échecs-là atterrissent dans la ligne de suivi, sous forme de code stable que
l'interface traduit.

La distinction n'est pas théorique : elle décide de ce que l'utilisateur voit
au moment où il clique. Une adresse fautive doit se dire tout de suite ; un
catalogue en panne ne peut se dire qu'après.
*/

// ErrForeignHost signale une adresse qui n'appartient à aucun catalogue déclaré.
var ErrForeignHost = errors.New(
	"discovery : cette adresse n'appartient à aucun catalogue déclaré")

// ImportParams décrit un import.
type ImportParams struct {
	// SourceID est le catalogue d'où vient le lien. C'est lui qui autorise
	// l'adresse, et qui fournit les identifiants s'il en faut.
	SourceID uuid.UUID
	// Href est le lien d'acquisition, tel que rendu par la recherche.
	Href string

	LibraryID uuid.UUID
	// Folder est le dossier de destination, relatif au préfixe de la
	// bibliothèque. Vide pour la racine.
	Folder string

	// Title sert à nommer le fichier quand le catalogue ne donne rien de mieux.
	Title string

	// RequestedBy est le compte à l'origine de la demande. L'import écrit dans
	// une bibliothèque : savoir de qui vient l'écriture est le minimum quand
	// plusieurs comptes se la partagent.
	RequestedBy *uuid.UUID
}

// ImportStatus est l'état d'un import.
type ImportStatus string

const (
	ImportQueued  ImportStatus = "queued"
	ImportRunning ImportStatus = "running"
	ImportDone    ImportStatus = "done"
	ImportFailed  ImportStatus = "failed"
)

// Import est la trace persistée d'une demande d'import.
//
// Elle existe parce que l'import est asynchrone : sans elle, l'utilisateur
// lancerait une action qui disparaît de sa vue, sans moyen de savoir si elle a
// abouti ni pourquoi elle a échoué.
type Import struct {
	ID         uuid.UUID
	SourceID   *uuid.UUID
	SourceName string
	Href       string
	LibraryID  uuid.UUID
	Folder     string
	Title      string

	Status ImportStatus
	// ErrorCode est un code stable, traduit par l'interface.
	ErrorCode string
	// ErrorDetail est le diagnostic brut, souvent celui du catalogue distant.
	ErrorDetail string

	ComicID   *uuid.UUID
	ObjectKey string
	FileSize  int64

	RequestedBy *uuid.UUID
	CreatedAt   time.Time
	StartedAt   *time.Time
	FinishedAt  *time.Time
}

/*
Deposit dépose un flux dans une bibliothèque.

Déclarée au point d'usage plutôt qu'empruntée au paquet d'ingestion : ce module
n'a pas à connaître les types de l'autre, et cette signature étroite est aussi
ce qui permet de tester l'import sans backend de stockage.
*/
type Deposit func(ctx context.Context, p DepositParams) (Deposited, error)

type DepositParams struct {
	LibraryID uuid.UUID
	Folder    string
	Filename  string
	// Size vaut -1 quand le catalogue n'annonce pas de longueur, ce qui arrive
	// dès qu'il répond en flux fragmenté.
	Size    int64
	Content io.Reader
}

type Deposited struct {
	ComicID   uuid.UUID
	ObjectKey string
	Title     string
	Format    string
	Size      int64
}

/*
RequestImport enregistre une demande d'import et l'enfile.

Ce qui est vérifié ici l'est parce que ce sont des propriétés de la DEMANDE, et
qu'un refus immédiat vaut mieux qu'une ligne de suivi en échec deux secondes
plus tard : le catalogue doit exister, et l'adresse lui appartenir.

L'enregistrement précède l'enfilement — un job dont la ligne de suivi
n'existerait pas travaillerait à l'aveugle. Les deux ne sont pas dans une même
transaction, et l'écart est traité plutôt que nié : si l'enfilement échoue, la
ligne est marquée en échec sur-le-champ. Un import « en attente » qui ne démarre
jamais serait le pire des deux mondes, indiscernable d'un simple retard.
*/
func (s *Service) RequestImport(ctx context.Context, p ImportParams) (Import, error) {
	source, _, err := s.credentials(ctx, p.SourceID)
	if err != nil {
		return Import{}, err
	}

	if err := sameOrigin(source.URL, p.Href); err != nil {
		return Import{}, err
	}
	if err := netguard.Check(p.Href); err != nil {
		return Import{}, fmt.Errorf("%w : %w", ErrInvalidSource, err)
	}

	if s.queue == nil {
		return Import{}, errors.New("discovery : aucune file d'import configurée")
	}

	record, err := s.repo.CreateImport(ctx, Import{
		SourceID:    &source.ID,
		SourceName:  source.Name,
		Href:        p.Href,
		LibraryID:   p.LibraryID,
		Folder:      p.Folder,
		Title:       p.Title,
		Status:      ImportQueued,
		RequestedBy: p.RequestedBy,
	})
	if err != nil {
		return Import{}, err
	}

	if err := s.queue.EnqueueImport(ctx, record.ID); err != nil {
		// La ligne existe mais rien ne la fera avancer. La marquer en échec
		// tout de suite plutôt que de la laisser « en attente » indéfiniment :
		// une file en panne doit se voir, pas se déguiser en lenteur.
		if failErr := s.repo.FailImport(ctx, record.ID, "queue", err.Error()); failErr != nil {
			s.log.Error("import ni enfilé ni marqué en échec",
				slog.String("import", record.ID.String()), slog.Any("err", failErr))
		}
		return Import{}, fmt.Errorf("discovery : import non enfilé : %w", err)
	}

	record.Status = ImportQueued
	return record, nil
}

/*
RunImport exécute un import enfilé.

Appelé par le worker, jamais par un gestionnaire HTTP.

Le corps de la réponse distante est passé DIRECTEMENT au dépôt, sans fichier
temporaire ni tampon en mémoire : une intégrale de cinq cents méga-octets
traverse le serveur sans jamais y tenir en entier. C'est le même choix que pour
le téléversement, et pour la même raison.
*/
func (s *Service) RunImport(ctx context.Context, importID uuid.UUID, deposit Deposit) error {
	record, err := s.repo.GetImport(ctx, importID)
	if err != nil {
		return err
	}

	// Un import déjà abouti ne se rejoue pas. River peut relancer un job dont
	// la réussite n'a pas été enregistrée à temps ; sans ce garde, la seconde
	// tentative se heurterait au refus d'écraser et marquerait en échec un
	// import qui a parfaitement fonctionné.
	if record.Status == ImportDone {
		return nil
	}

	if err := s.repo.StartImport(ctx, importID); err != nil {
		return err
	}

	deposited, err := s.runImport(ctx, record, deposit)
	if err != nil {
		code, detail := importFailure(err)
		if failErr := s.repo.FailImport(ctx, importID, code, detail); failErr != nil {
			s.log.Error("échec d'import non enregistré",
				slog.String("import", importID.String()), slog.Any("err", failErr))
		}
		// L'erreur n'est PAS remontée à River. Elle est déjà consignée, et la
		// rendre ferait retenter un import qu'un catalogue éteint ou un format
		// refusé feront échouer à l'identique — trois fois, à intervalles
		// croissants, pour le même résultat.
		s.log.Info("import en échec",
			slog.String("import", importID.String()),
			slog.String("code", code),
			slog.Any("err", err))
		return nil
	}

	if err := s.repo.FinishImport(ctx, importID, deposited); err != nil {
		return err
	}

	/*
		L'enrichissement vient APRÈS la clôture, et son échec n'est pas remonté.

		L'ordre compte : un import est terminé dès que le fichier est dans la
		bibliothèque et inscrit au catalogue. Le faire dépendre d'une requête
		vers un service tiers laisserait un import « en cours » à cause d'une
		base publique en panne, alors que l'album est là et lisible.
	*/
	s.enrichImported(ctx, deposited.ComicID, deposited.Title)
	return nil
}

func (s *Service) runImport(
	ctx context.Context, record Import, deposit Deposit,
) (Deposited, error) {
	if record.SourceID == nil {
		return Deposited{}, ErrSourceNotFound
	}

	source, password, err := s.credentials(ctx, *record.SourceID)
	if err != nil {
		return Deposited{}, err
	}

	// Revérifié au moment de partir, et pas seulement à la demande : entre les
	// deux, un administrateur a pu changer l'adresse du catalogue, et un job en
	// attente ne doit pas garder un droit d'accès qu'on lui a retiré.
	if err := sameOrigin(source.URL, record.Href); err != nil {
		return Deposited{}, err
	}
	if err := netguard.Check(record.Href); err != nil {
		return Deposited{}, fmt.Errorf("%w : %w", ErrInvalidSource, err)
	}

	fetched, err := s.client.Open(ctx, source, password, record.Href)
	if err != nil {
		return Deposited{}, err
	}
	defer func() { _ = fetched.Body.Close() }()

	return deposit(ctx, DepositParams{
		LibraryID: record.LibraryID,
		Folder:    record.Folder,
		Filename:  filenameFor(fetched, record.Href, record.Title),
		Size:      fetched.Size,
		Content:   fetched.Body,
	})
}

// ListImports rend les imports récents, du plus neuf au plus ancien.
func (s *Service) ListImports(ctx context.Context, limit int) ([]Import, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return s.repo.ListImports(ctx, limit)
}

/*
importFailure traduit un échec en code stable et en diagnostic.

Le code est destiné à l'interface, qui le traduit ; le diagnostic est destiné à
un humain qui cherche pourquoi, et vient souvent du catalogue distant. Les
confondre donnerait soit un message intraduisible, soit un code qui ne dit rien
du vrai problème.

Les erreurs d'ingestion ne sont pas nommées ici : ce paquet ne connaît pas
l'autre. C'est l'adaptateur qui les enveloppe dans une ErrDeposit porteuse d'un
code — le seul endroit où les deux vocabulaires se rencontrent.
*/
func importFailure(err error) (string, string) {
	var deposit ErrDeposit
	switch {
	case errors.As(err, &deposit):
		return deposit.Code, deposit.Detail
	case errors.Is(err, ErrForeignHost):
		return "foreign-host", err.Error()
	case errors.Is(err, ErrSourceNotFound):
		return "source-gone", err.Error()
	case errors.Is(err, ErrInvalidSource):
		return "invalid", err.Error()
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		return "timeout", err.Error()
	default:
		return "unreachable", err.Error()
	}
}

/*
ErrDeposit porte l'échec d'un dépôt, traduit en code par l'adaptateur.

Sans elle, ce paquet devrait connaître les erreurs de l'ingestion pour les
nommer, ce qui reviendrait à en dépendre par la bande. L'adaptateur, lui, connaît
déjà les deux.
*/
type ErrDeposit struct {
	Code   string
	Detail string
}

func (e ErrDeposit) Error() string { return e.Detail }

/*
sameOrigin vérifie que l'adresse appartient bien au catalogue annoncé.

La comparaison porte sur le schéma, l'hôte et le port, pas sur le chemin : les
catalogues servent leurs fichiers depuis des chemins sans rapport avec celui du
flux — `/opds/v2` pour la recherche, `/api/v1/books/42/file` pour le contenu —
et exiger un préfixe commun casserait la plupart des implémentations.

L'hôte, en revanche, ne bouge pas. C'est lui qui délimite ce qu'un
administrateur a ouvert.
*/
func sameOrigin(sourceURL, href string) error {
	base, err := url.Parse(sourceURL)
	if err != nil {
		return fmt.Errorf("%w : %w", ErrInvalidSource, err)
	}
	target, err := url.Parse(href)
	if err != nil {
		return fmt.Errorf("%w : adresse illisible", ErrForeignHost)
	}

	if !strings.EqualFold(base.Scheme, target.Scheme) ||
		!strings.EqualFold(base.Host, target.Host) {
		return fmt.Errorf("%w : %s n'est pas %s", ErrForeignHost, target.Host, base.Host)
	}
	return nil
}

/*
filenameFor trouve un nom de fichier utilisable.

Ce n'est pas un détail cosmétique : le nom devient la clé de l'objet, et c'est
lui que l'indexation analyse pour en tirer série, tome et titre. Un import nommé
`file` donne un album nommé « file ».

Trois pistes, de la plus fiable à la moins mauvaise :

 1. l'en-tête `Content-Disposition`, quand le catalogue le pose — c'est le seul
    endroit où il déclare vraiment un nom ;
 2. le dernier segment de l'adresse, s'il porte une extension reconnue. Beaucoup
    de catalogues servent `/download/le-titre.cbz` ;
 3. le titre du résultat, complété par l'extension déduite du type de contenu.
    C'est le cas de Komga et Kavita, dont les liens ressemblent à
    `/api/v1/books/42/file` et ne disent rien.

Sans extension à l'arrivée, l'ingestion refusera le fichier — et c'est le bon
comportement : mieux vaut un refus lisible qu'un objet déposé sous un format
que personne ne saura ouvrir.
*/
func filenameFor(fetched Fetched, href, title string) string {
	if fetched.Filename != "" {
		return path.Base(fetched.Filename)
	}

	if parsed, err := url.Parse(href); err == nil {
		base := path.Base(parsed.Path)
		if extensions[strings.ToLower(path.Ext(base))] {
			return base
		}
	}

	extension := extensionForContentType(fetched.ContentType)
	name := strings.TrimSpace(title)
	if name == "" {
		name = "import"
	}
	return name + extension
}

// extensions reconnues dans une adresse.
//
// Volontairement la même liste que celle de l'ingestion : un nom qu'on retient
// ici et qu'elle refuserait ensuite ne rendrait service à personne.
var extensions = map[string]bool{
	".cbz": true, ".zip": true,
	".cbr": true, ".rar": true,
	".cb7": true, ".7z": true,
	".pdf": true, ".epub": true,
}

/*
extensionForContentType déduit une extension du type annoncé.

Le `.cbz` par défaut n'est pas un pari : c'est ce que sert l'écrasante majorité
des catalogues de bande dessinée, et l'ingestion vérifie de toute façon la
signature du contenu avant d'écrire. Se tromper ici donne un refus propre, pas
un fichier corrompu dans le bucket.
*/
func extensionForContentType(contentType string) string {
	media, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		media = strings.ToLower(strings.TrimSpace(contentType))
	}

	switch media {
	case "application/pdf":
		return ".pdf"
	case "application/epub+zip":
		return ".epub"
	case "application/vnd.comicbook-rar", "application/x-cbr", "application/vnd.rar":
		return ".cbr"
	case "application/x-7z-compressed", "application/vnd.comicbook-7z":
		return ".cb7"
	default:
		return ".cbz"
	}
}
