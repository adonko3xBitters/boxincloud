// Package web embarque l'application web compilée dans le binaire du serveur.
//
// Conséquence directe de l'ADR-003 (docs/01-architecture.md) : Next.js est
// compilé en export statique, ce qui permet de livrer un artefact unique. Pas
// de runtime Node à déployer à côté, pas de reverse-proxy à configurer pour
// répartir entre deux services.
//
// `make build-web` remplit dist/. Un placeholder y est versionné pour que le
// serveur compile sans avoir à builder le web au préalable — un contributeur
// backend n'a pas besoin d'installer Node.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var embedded embed.FS

// FS retourne le système de fichiers de l'application web, racine à dist/.
func FS() (fs.FS, error) {
	return fs.Sub(embedded, "dist")
}
