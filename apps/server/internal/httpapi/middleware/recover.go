package middleware

import (
	"errors"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/adonko3xBitters/boxincloud/server/internal/httpapi/problem"
	"github.com/adonko3xBitters/boxincloud/server/internal/platform/logging"
)

// Recover intercepte les paniques d'un handler et répond 500.
//
// Une panique dans une requête ne doit jamais faire tomber le serveur : les
// autres lectures en cours n'ont pas à en pâtir. La trace complète part dans
// les logs, jamais dans la réponse — elle révélerait la structure interne.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}
			// Déconnexion cliente : ce n'est pas une erreur du serveur.
			if err, ok := rec.(error); ok && errors.Is(err, http.ErrAbortHandler) {
				panic(rec)
			}

			logging.FromContext(r.Context()).Error("panique dans un handler",
				slog.Any("panic", rec),
				slog.String("stack", string(debug.Stack())),
			)

			problem.Write(w, r, problem.Internal())
		}()

		next.ServeHTTP(w, r)
	})
}
