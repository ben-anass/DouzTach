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
	workspaceHandler *handlers.WorkspaceHandler,
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

		r.With(jwtService.Middleware).Post("/workspaces", workspaceHandler.Create)
		r.With(jwtService.Middleware).Get("/workspaces", workspaceHandler.List)

		r.With(jwtService.Middleware).Get(
			"/workspaces/{workspaceID}/members",
			workspaceHandler.ListMembers,
		)

		// Remove workspace member
		r.With(jwtService.Middleware).Delete(
			"/workspaces/{workspaceID}/members/{userID}",
			workspaceHandler.RemoveMember,
		)
		// Leave workspace
		r.With(jwtService.Middleware).Delete(
			"/workspaces/{workspaceID}/leave",
			workspaceHandler.LeaveWorkspace,
		)
		// Change workspace member role
		r.With(jwtService.Middleware).Patch(
			"/workspaces/{workspaceID}/members/{userID}/role",
			workspaceHandler.ChangeMemberRole,
		)
		r.With(jwtService.Middleware).Post(
			"/workspaces/{workspaceID}/invitations",
			workspaceHandler.CreateInvitation,
		)
		// Cancel workspace invitation
		r.With(jwtService.Middleware).Delete(
			"/workspaces/{workspaceID}/invitations/{invitationID}",
			workspaceHandler.CancelInvitation,
		)

		// List workspace invitations
		r.With(jwtService.Middleware).Get(
			"/workspaces/{workspaceID}/invitations",
			workspaceHandler.ListInvitations,
		)
		// Accept workspace invitation
		r.With(jwtService.Middleware).Post(
			"/invitations/{token}/accept",
			workspaceHandler.AcceptInvitation,
		)

		// Reject workspace invitation
		r.With(jwtService.Middleware).Post(
			"/invitations/{token}/reject",
			workspaceHandler.RejectInvitation,
		)
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
		"id":       user.ID.String(),
		"username": user.Username,
		"nickname": user.Nickname,
		"email":    user.Email,
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
