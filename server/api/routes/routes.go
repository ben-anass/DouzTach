package routes

import (
	"encoding/json"
	"net/http"

	"github.com/ben-anass/DouzTach/internal/auth"
	"github.com/ben-anass/DouzTach/server/api/handlers"
	"github.com/ben-anass/DouzTach/server/users"
	"github.com/go-chi/chi/v5"
)

func NewRouter(
	authHandler *handlers.AuthHandler,
	jwtService *auth.JWTService,
	userService *users.Service,
) http.Handler {
	r := chi.NewRouter()

	r.Get("/", homeHandler)
	r.Get("/health", healthHandler)

	r.Route("/api/v1", func(r chi.Router) {
		// Public routes
		r.Route("/auth", func(r chi.Router) {
			r.Post("/register", authHandler.Register)
			r.Post("/login", authHandler.Login)
		})

		// Protected routes
		r.With(jwtService.Middleware).Get("/me", func(w http.ResponseWriter, r *http.Request) {
			meHandler(userService, w, r)
		})
	})

	return r
}

func meHandler(userService *users.Service, w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	user, err := userService.GetUserByID(r.Context(), userID)
	if err != nil {
		http.Error(w, `{"error":"user not found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	json.NewEncoder(w).Encode(map[string]string{
		"id":    user.ID.String(),
		"name":  user.Name,
		"email": user.Email,
	})
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
