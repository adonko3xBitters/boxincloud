package indexer

import (
	"context"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
)

// DirectRunner exécute le pipeline sans passer par la file de jobs.
//
// Deux usages :
//
//   - `boxincloudctl scan-now`, qui rend le scan observable depuis un terminal
//     sans qu'un serveur tourne ;
//   - les tests d'intégration, qui vérifient le pipeline de bout en bout sans
//     dépendre de l'ordonnancement de River.
//
// C'est exactement le même code métier qui s'exécute : les workers River ne
// font que l'appeler. Un pipeline validé ici est validé en production.
type DirectRunner struct {
	scan  *ScanLibraryWorker
	index *IndexComicWorker
}

func NewDirectRunner(deps Deps) *DirectRunner {
	runner := &DirectRunner{}

	// L'indexation est exécutée en ligne plutôt qu'enfilée : le scan ne rend
	// la main qu'une fois tous les albums indexés, ce qui rend le résultat
	// immédiatement observable et les tests déterministes.
	interceptor := &inlineIndexer{runner: runner}
	deps.Repo = &enqueueInterceptor{Repository: deps.Repo, inline: interceptor}

	runner.scan = &ScanLibraryWorker{deps: deps}
	runner.index = &IndexComicWorker{deps: deps}
	return runner
}

// ScanAndIndex parcourt la bibliothèque puis indexe tout ce qui en a besoin.
func (r *DirectRunner) ScanAndIndex(ctx context.Context, libraryID uuid.UUID) (ScanStats, error) {
	return r.scan.scan(ctx, libraryID)
}

// IndexComic indexe un album précis.
func (r *DirectRunner) IndexComic(ctx context.Context, comicID uuid.UUID) error {
	return r.index.Work(ctx, &river.Job[IndexComicArgs]{Args: IndexComicArgs{ComicID: comicID}})
}

// inlineIndexer référence le runner une fois celui-ci construit — les deux
// workers et l'intercepteur se référencent mutuellement.
type inlineIndexer struct {
	runner *DirectRunner
}

// enqueueInterceptor détourne EnqueueIndexComic vers une exécution immédiate,
// en laissant passer tout le reste vers le repository réel.
type enqueueInterceptor struct {
	Repository
	inline *inlineIndexer
}

func (i *enqueueInterceptor) EnqueueIndexComic(ctx context.Context, comicID uuid.UUID) error {
	return i.inline.runner.IndexComic(ctx, comicID)
}
