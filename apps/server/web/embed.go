// Package web embarque l'application web compilée dans le binaire du serveur.
//
// Conséquence directe de l'ADR-003 (docs/01-architecture.md) : Next.js est
// compilé en export statique, ce qui permet de livrer un artefact unique. Pas
// de runtime Node à déployer à côté, pas de reverse-proxy à configurer pour
// répartir entre deux services.
//
// `make build-web` remplit dist/. Seul le répertoire est versionné, pas son
// contenu : le serveur compile donc sans build web préalable — un contributeur
// backend n'a pas besoin d'installer Node — et le dépôt ne transporte pas
// d'artefact compilé.
package web

import (
	"embed"
	"errors"
	"io/fs"
)

//go:embed all:dist
var embedded embed.FS

// ErrNotBuilt signale un binaire compilé sans application web.
//
// Le cas est normal en développement backend, et doit le rester : le serveur
// continue de servir son API. Il mérite pourtant d'être nommé, parce que le
// symptôme sans diagnostic — une page blanche — enverrait chercher la panne du
// mauvais côté.
var ErrNotBuilt = errors.New("web : application non compilée (lancer `make build-web`)")

// FS retourne le système de fichiers de l'application web, racine à dist/.
func FS() (fs.FS, error) {
	sub, err := fs.Sub(embedded, "dist")
	if err != nil {
		return nil, err
	}
	if _, err := fs.Stat(sub, "index.html"); err != nil {
		return nil, ErrNotBuilt
	}
	return sub, nil
}
