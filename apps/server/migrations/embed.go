// Package migrations embarque les migrations SQL dans le binaire.
//
// Elles sont ainsi appliquées au démarrage sans fichier externe : un seul
// artefact à déployer, aucune désynchronisation possible entre le binaire et
// le schéma qu'il attend.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
