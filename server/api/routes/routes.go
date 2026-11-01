package routes

import (
	"encoding/json"
	"github.com/ben-anass/DouzTach/server/api/handlers"
	"github.com/go-chi/chi/v5"
	"net/http"
)

func NewRouter(authHandler *handlers.AuthHandler) http.Handler {
	r := chi.NewRouter()

	r.Get("/", homeHandler)
	r.Get("/health", healthHandler)

	r.Route("/api/v1", func(r chi.Router) {
		r.Route("/auth", func(r chi.Router) {
			r.Post("/register", authHandler.Register)
		})
	})

	return r
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	response := map[string]string{
		"app":     "DouzTach",
		"message": "Welcome to DouzTach API",
		"version": "0.1.0",
	}

	json.NewEncoder(w).Encode(response)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	response := map[string]string{
		"status": "ok",
		"app":    "DouzTach",
	}

	json.NewEncoder(w).Encode(response)
}
