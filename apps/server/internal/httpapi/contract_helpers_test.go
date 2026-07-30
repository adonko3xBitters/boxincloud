package httpapi_test

import (
	"context"
	"io"
	"log/slog"

	"github.com/google/uuid"

	"github.com/adonko3xBitters/boxincloud/server/internal/app"
	"github.com/adonko3xBitters/boxincloud/server/internal/indexer"
	"github.com/adonko3xBitters/boxincloud/server/internal/platform/sqlc"
)

// newDirectRunner expose l'indexation en direct au test de contrat.
//
// Le contrat porte sur l'API, pas sur l'ordonnancement de la file de jobs :
// on indexe en ligne pour que le jeu de données soit prêt de façon
// déterministe.
func newDirectRunner(core *app.Core) func(context.Context, uuid.UUID) (indexer.ScanStats, error) {
	runner := indexer.NewDirectRunner(indexer.Deps{
		Libraries: core.Libraries,
		Repo:      core.Indexer,
		Cache:     core.Cache,
		Imaging:   core.Imaging,
		Log:       slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	return runner.ScanAndIndex
}

func listParams(libraryID uuid.UUID) sqlc.ListComicsByLibraryParams {
	return sqlc.ListComicsByLibraryParams{LibraryID: libraryID, Limit: 10}
}
