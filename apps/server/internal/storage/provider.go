// Package storage définit l'abstraction d'accès aux octets.
//
// C'est la pièce centrale de boxincloud. Aucun autre module n'ouvre de fichier,
// n'appelle un SDK cloud, ni ne suppose l'existence d'un chemin sur disque :
// tout passe par Provider. C'est cette contrainte — et rien d'autre — qui rend
// possible le support simultané de MinIO, S3, Backblaze B2, Cloudflare R2,
// Wasabi et d'un disque local.
//
// ReadRange est la méthode la plus importante de l'interface. Elle permet de
// lire la page 12 d'une archive de 200 Mo sans la télécharger : on lit l'index
// ZIP en fin de fichier, on le persiste en base, puis chaque page se sert par
// une unique requête HTTP Range.
//
// Les implémentations arrivent avec M1.
package storage

import (
	"context"
	"errors"
	"io"
	"time"
)

// Kind identifie le type d'un backend.
type Kind string

const (
	KindS3     Kind = "s3"     // MinIO, AWS S3, Backblaze B2, Cloudflare R2, Wasabi, Garage
	KindLocal  Kind = "local"  // système de fichiers
	KindWebDAV Kind = "webdav" // Nextcloud, Synology
)

// Erreurs communes à toutes les implémentations. Les providers doivent
// traduire les erreurs de leur SDK vers celles-ci : les appelants ne doivent
// jamais avoir à distinguer une 404 S3 d'un ENOENT.
var (
	ErrNotFound         = errors.New("storage : objet introuvable")
	ErrNotSupported     = errors.New("storage : opération non supportée par ce backend")
	ErrReadOnly         = errors.New("storage : backend en lecture seule")
	ErrInvalidRange     = errors.New("storage : plage demandée invalide")
	ErrPermissionDenied = errors.New("storage : accès refusé")
)

// ObjectInfo décrit un objet sans en lire le contenu.
type ObjectInfo struct {
	Key     string
	Size    int64
	ModTime time.Time

	// ETag sert à détecter une modification sans relire l'objet. Sa forme
	// varie selon les fournisseurs (MD5, somme composite pour les envois
	// multipart) : on le traite comme une chaîne opaque, jamais comme un hash.
	ETag string

	ContentType string
}

// Provider est l'unique porte d'accès aux octets.
//
// Une implémentation doit être sûre d'emploi concurrent : le serveur sert
// plusieurs lectures en parallèle sur le même provider.
type Provider interface {
	// Kind indique le type de backend.
	Kind() Kind

	// Ping vérifie que le backend est joignable et les identifiants valides.
	// Appelé à l'ajout d'un backend et périodiquement pour l'indicateur de santé.
	Ping(ctx context.Context) error

	// List énumère les objets sous un préfixe, en appelant fn pour chacun.
	//
	// Le parcours est en flux plutôt qu'en tranche : une bibliothèque peut
	// contenir des dizaines de milliers d'objets, et les charger tous en
	// mémoire avant de commencer à traiter serait inutilement coûteux.
	// Une erreur retournée par fn interrompt le parcours et remonte.
	List(ctx context.Context, prefix string, fn func(ObjectInfo) error) error

	// Stat retourne les métadonnées d'un objet.
	// Retourne ErrNotFound s'il n'existe pas.
	Stat(ctx context.Context, key string) (ObjectInfo, error)

	// Open ouvre un objet en lecture intégrale, depuis le début.
	// L'appelant doit fermer le lecteur retourné.
	Open(ctx context.Context, key string) (io.ReadCloser, error)

	// ReadRange lit length octets à partir de offset.
	//
	// ★ Méthode centrale du projet : c'est elle qui permet de servir une page
	// sans télécharger l'archive entière.
	//
	// Un length négatif signifie « jusqu'à la fin de l'objet ». Un offset
	// au-delà de la taille retourne ErrInvalidRange.
	// L'appelant doit fermer le lecteur retourné.
	ReadRange(ctx context.Context, key string, offset, length int64) (io.ReadCloser, error)

	// Write écrit un objet. size vaut -1 si la taille est inconnue à l'avance ;
	// les backends qui l'exigent devront alors bufferiser.
	Write(ctx context.Context, key string, r io.Reader, size int64, contentType string) error

	// Delete supprime un objet. Supprimer un objet absent n'est pas une erreur.
	Delete(ctx context.Context, key string) error
}

// Presigner est implémenté par les backends sachant produire une URL d'accès
// direct temporaire.
//
// Quand elle est disponible, le serveur y redirige le client plutôt que de
// relayer les octets : la bande passante ne transite pas par le serveur, ce qui
// change tout sur une bibliothèque servie à plusieurs personnes.
//
// Interface optionnelle : tester avec une assertion de type.
type Presigner interface {
	PresignedURL(ctx context.Context, key string, ttl time.Duration) (string, error)
}

// Config décrit un backend tel que stocké en base.
//
// Les identifiants sont séparés du reste : Options est renvoyé par l'API,
// Secrets est chiffré en base et n'en sort jamais.
type Config struct {
	Kind    Kind
	Options map[string]string // endpoint, bucket, region, use_ssl, path_style…
	Secrets map[string]string // access_key, secret_key…
}
