// Package middleware regroupe les intergiciels HTTP du serveur.
package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"

	"github.com/adonko3xBitters/boxincloud/server/internal/platform/logging"
)

// Logger journalise chaque requête et attache un logger enrichi au contexte.
//
// Les handlers peuvent ensuite écrire via logging.FromContext(ctx) sans avoir à
// transporter l'identifiant de requête dans leurs signatures.
func Logger(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Les sondes de santé sont appelées en permanence par les
			// orchestrateurs : les journaliser noierait tout le reste.
			if isHealthProbe(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			reqLog := log.With(
				slog.String("request_id", middleware.GetReqID(r.Context())),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
			)
			ctx := logging.WithLogger(r.Context(), reqLog)

			next.ServeHTTP(ww, r.WithContext(ctx))

			status := ww.Status()
			attrs := []any{
				slog.Int("status", status),
				slog.Int("bytes", ww.BytesWritten()),
				slog.Duration("duration", time.Since(start)),
			}

			switch {
			case status >= 500:
				reqLog.Error("requête", attrs...)
			case status >= 400:
				reqLog.Warn("requête", attrs...)
			default:
				reqLog.Info("requête", attrs...)
			}
		})
	}
}

func isHealthProbe(path string) bool {
	return path == "/healthz" || path == "/readyz"
}
