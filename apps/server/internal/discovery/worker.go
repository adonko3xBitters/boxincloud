package discovery

import (
	"context"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
)

/*
Worker d'import.

L'import est la seule opération du module qui écrive, et la seule qui dure : le
temps d'un téléchargement, quelques secondes pour un album, plusieurs minutes
pour une intégrale servie par un catalogue lointain.

Le tenir dans une requête HTTP obligeait le navigateur à garder une connexion
ouverte tout du long, et faisait perdre l'import à qui fermait son onglet. Le
job règle les deux : la demande est enregistrée puis enfilée, et le
téléchargement survit à celui qui l'a demandé.

# Une seule tentative

`MaxAttempts` vaut 1, ce qui mérite d'être justifié parce que c'est l'inverse
du réglage habituel.

Les échecs d'un import sont, dans leur immense majorité, déterministes :
un catalogue éteint, une adresse qui a changé, un format refusé, un fichier déjà
présent. Les retenter trois fois à intervalles croissants ne change rien au
résultat, mais fait patienter l'utilisateur devant un import « en cours » qui ne
peut pas aboutir, et martèle un serveur tiers.

Les échecs sont donc consignés dans la ligne de suivi plutôt que rendus à River,
et l'utilisateur relance lui-même s'il pense que la cause a disparu. C'est une
décision qu'il est mieux placé que nous pour prendre : lui sait s'il vient de
rallumer son Komga.
*/

// ImportArgs déclenche un import enregistré.
//
// Seul l'identifiant voyage. Tout le reste — adresse, bibliothèque, dossier —
// est relu en base au moment d'exécuter : un job porte une intention, pas une
// copie de l'état, qui aurait vieilli entre-temps.
type ImportArgs struct {
	ImportID uuid.UUID `json:"import_id"`
}

func (ImportArgs) Kind() string { return "discovery_import" }

func (ImportArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: 1}
}

// ImportWorker exécute les imports.
type ImportWorker struct {
	river.WorkerDefaults[ImportArgs]

	svc *Service
	// deposit écrit dans la bibliothèque. Fourni par le câblage, pour que ce
	// paquet n'ait pas à connaître l'ingestion.
	deposit Deposit
}

func NewImportWorker(svc *Service, deposit Deposit) *ImportWorker {
	return &ImportWorker{svc: svc, deposit: deposit}
}

func (w *ImportWorker) Work(ctx context.Context, job *river.Job[ImportArgs]) error {
	return w.svc.RunImport(ctx, job.Args.ImportID, w.deposit)
}

// Register déclare le worker auprès de la file.
//
// Même forme que pour l'indexeur : le câblage se fait dans `app`, et le paquet
// d'infrastructure ne connaît aucun module métier.
func Register(workers *river.Workers, svc *Service, deposit Deposit) {
	river.AddWorker(workers, NewImportWorker(svc, deposit))
}
