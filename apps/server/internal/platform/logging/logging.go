// Package logging configure le logger structuré de l'application.
package logging

import (
	"context"
	"log/slog"
	"os"
)

// New construit un logger structuré.
//
// Format "text" en développement (lisible dans un terminal), "json" en
// production (exploitable par un collecteur).
func New(level slog.Level, format string) *slog.Logger {
	opts := &slog.HandlerOptions{Level: level}

	var handler slog.Handler
	if format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}
	return slog.New(handler)
}

type ctxKey struct{}

// WithLogger attache un logger au contexte.
func WithLogger(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, l)
}

// FromContext récupère le logger attaché au contexte, ou le logger par défaut.
//
// Permet aux handlers de logger avec les attributs de la requête (identifiant,
// utilisateur) sans avoir à faire circuler le logger dans chaque signature.
func FromContext(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(ctxKey{}).(*slog.Logger); ok {
		return l
	}
	return slog.Default()
}
