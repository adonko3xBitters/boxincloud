package handlers

import (
	"log/slog"
	"net/http"

	"github.com/adonko3xBitters/boxincloud/server/internal/httpapi/problem"
	"github.com/adonko3xBitters/boxincloud/server/internal/platform/logging"
)

// writeInternal répond 500 en journalisant la cause.
//
// Le client ne reçoit qu'un message vague — révéler le détail d'une erreur
// interne renseignerait un attaquant sur la structure du serveur. Mais cette
// cause doit impérativement finir dans les logs, sinon un défaut se manifeste
// comme un 500 muet et devient impossible à diagnostiquer.
//
// Aucun handler ne doit appeler problem.Internal() directement.
func writeInternal(w http.ResponseWriter, r *http.Request, err error) {
	logging.FromContext(r.Context()).Error("erreur interne",
		slog.Any("err", err),
		slog.String("path", r.URL.Path),
	)
	problem.Write(w, r, problem.Internal())
}
