package auth

import (
	"context"
	"fmt"
)

/*
Vérification d'identifiants sans émission de jeton.

Elle existe pour l'OPDS. Les lecteurs de bande dessinée tiers — Chunky, Panels,
KyBook, Moon+ Reader — n'implémentent pas d'échange de jetons : ils envoient un
couple identifiant/mot de passe en Basic sur CHAQUE requête, et c'est la seule
authentification que la spécification OPDS mentionne.

`Login` ne convient pas pour cela. Il crée un appareil, émet une paire de jetons
et horodate la connexion — trois écritures par requête, alors qu'un lecteur en
fait une vingtaine pour afficher une page de couvertures.

D'où cette voie plus courte : vérifier le mot de passe, rendre le compte, et
rien d'autre.
*/

// VerifyCredentials valide un couple identifiant/mot de passe.
//
// Ne crée aucun appareil, n'émet aucun jeton et n'horodate rien : c'est une
// vérification, pas une connexion.
func (s *Service) VerifyCredentials(
	ctx context.Context, username, password string,
) (User, error) {
	user, hash, err := s.repo.GetUserByUsername(ctx, username)
	if err != nil {
		// Un hachage est effectué malgré tout, pour que la réponse prenne le
		// même temps qu'avec un compte réel. Sans cela, la durée permettrait
		// d'énumérer les comptes.
		DummyVerify(password)
		return User{}, ErrInvalidCredentials
	}

	if err := VerifyPassword(password, hash); err != nil {
		return User{}, ErrInvalidCredentials
	}

	// Le rôle de la base fait foi, comme pour un jeton : un compte désactivé
	// entre deux requêtes doit cesser d'être servi immédiatement.
	role, err := s.AccountState(ctx, user.ID)
	if err != nil {
		return User{}, fmt.Errorf("état du compte : %w", err)
	}
	user.Role = role

	return user, nil
}
