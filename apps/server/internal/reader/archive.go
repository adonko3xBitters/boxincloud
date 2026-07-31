package reader

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

/*
Téléchargement de l'archive entière.

C'est le seul chemin du projet qui rende le fichier complet, et il n'existe que
pour l'OPDS : un lecteur tiers ne sait pas lire page par page comme le fait
l'application. Il télécharge l'album, puis l'ouvre lui-même.

Cela va à l'encontre de tout le reste — l'accès aléatoire par requête Range est
la promesse du projet, et servir cinq cents méga-octets pour lire une page en
serait l'exact contraire. La différence est le CLIENT : celui qui passe par
cette route n'a aucun moyen de faire autrement, et lui refuser l'archive
reviendrait à lui refuser la bibliothèque.

Rien n'est mis en mémoire : le flux traverse le serveur du stockage vers le
client.
*/

// OpenArchive ouvre le fichier complet d'un album.
//
// La visibilité doit avoir été vérifiée par l'appelant : cette méthode ne
// connaît pas le lecteur, seulement l'album.
func (s *Service) OpenArchive(ctx context.Context, comicID uuid.UUID) (PageContent, error) {
	comic, err := s.repo.GetComic(ctx, comicID)
	if err != nil {
		return PageContent{}, err
	}

	/*
		L'objet d'ORIGINE est servi, jamais l'hydraté.

		Un CBR hydraté existe en CBZ dans le cache dérivé, et il serait plus
		commode à servir. Mais l'utilisateur qui télécharge son album attend le
		fichier qu'il a déposé, avec son format et sa taille — pas une
		transformation faite pour les besoins internes du lecteur.
	*/
	lib, err := s.libraries.GetLibrary(ctx, comic.LibraryID)
	if err != nil {
		return PageContent{}, err
	}
	provider, err := s.libraries.ProviderForLibrary(ctx, lib)
	if err != nil {
		return PageContent{}, err
	}

	info, err := provider.Stat(ctx, comic.ObjectKey)
	if err != nil {
		return PageContent{}, fmt.Errorf("archive introuvable : %w", err)
	}

	// Longueur -1 : tout l'objet, depuis le début. La borne de taille n'a pas
	// sa place ici — c'est le fichier de l'utilisateur, entier.
	body, err := provider.ReadRange(ctx, comic.ObjectKey, 0, -1)
	if err != nil {
		return PageContent{}, err
	}

	return PageContent{
		Body: body,
		Size: info.Size,
		ETag: info.ETag,
	}, nil
}
