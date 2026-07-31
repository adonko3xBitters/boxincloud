package discovery

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/url"
	"path"
	"strings"

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
Import télécharge un résultat et le dépose dans une bibliothèque.

Le corps de la réponse est passé DIRECTEMENT au dépôt, sans passer par un
fichier temporaire ni par la mémoire. Une intégrale de cinq cents méga-octets
traverse donc le serveur sans jamais y tenir en entier ; c'est le même choix que
pour le téléversement, et pour la même raison.
*/
func (s *Service) Import(ctx context.Context, p ImportParams, deposit Deposit) (Deposited, error) {
	source, password, err := s.credentials(ctx, p.SourceID)
	if err != nil {
		return Deposited{}, err
	}

	if err := sameOrigin(source.URL, p.Href); err != nil {
		return Deposited{}, err
	}
	if err := netguard.Check(p.Href); err != nil {
		return Deposited{}, fmt.Errorf("%w : %w", ErrInvalidSource, err)
	}

	fetched, err := s.client.Open(ctx, source, password, p.Href)
	if err != nil {
		return Deposited{}, err
	}
	defer func() { _ = fetched.Body.Close() }()

	return deposit(ctx, DepositParams{
		LibraryID: p.LibraryID,
		Folder:    p.Folder,
		Filename:  filenameFor(fetched, p.Href, p.Title),
		Size:      fetched.Size,
		Content:   fetched.Body,
	})
}

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
