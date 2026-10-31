package routes

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func NewRouter() http.Handler {
	r := chi.NewRouter()

	r.Get("/", homeHandler)
	r.Get("/health", healthHandler)

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
