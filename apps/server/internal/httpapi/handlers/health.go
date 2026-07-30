// Package handlers contient les handlers HTTP, un fichier par ressource.
package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/adonko3xBitters/boxincloud/server/internal/httpapi/problem"
)

// Pinger est ce dont Health a besoin d'une base de données — rien de plus.
//
// Déclarer l'interface au point d'usage plutôt que d'importer le package db
// garde les handlers testables sans PostgreSQL.
type Pinger interface {
	Ping(ctx context.Context) error
}

// BuildInfo décrit la version du binaire en cours d'exécution.
type BuildInfo struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	GoVersion string `json:"goVersion"`
}

// Health expose les sondes de disponibilité et la version.
type Health struct {
	db    Pinger
	build BuildInfo
}

func NewHealth(db Pinger, build BuildInfo) *Health {
	return &Health{db: db, build: build}
}

// Live répond dès lors que le process est vivant.
//
// Ne teste aucune dépendance : un orchestrateur ne doit pas redémarrer le
// serveur parce que PostgreSQL est momentanément indisponible — le
// redémarrage n'y changerait rien.
func (h *Health) Live(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Ready indique si le serveur peut traiter des requêtes.
//
// Teste les dépendances : un 503 ici retire l'instance du load balancer sans
// la tuer.
func (h *Health) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	if err := h.db.Ping(ctx); err != nil {
		problem.Write(w, r, problem.ServiceUnavailable("database is unreachable"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"database": "ok",
	})
}

// Version retourne les informations de build.
func (h *Health) Version(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.build)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
