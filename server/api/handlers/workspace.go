package handlers

import (
	"encoding/json"

	"net/http"

	"github.com/ben-anass/DouzTach/internal/auth"
	"github.com/ben-anass/DouzTach/internal/repository"
	"github.com/ben-anass/DouzTach/server/workspaces"
	"github.com/go-chi/chi/v5"
)

type WorkspaceHandler struct {
	workspaceService *workspaces.Service
}

func NewWorkspaceHandler(service *workspaces.Service) *WorkspaceHandler {
	return &WorkspaceHandler{
		workspaceService: service,
	}
}

type CreateWorkspaceRequest struct {
	Name        string `json:"name"`
	Nickname    string `json:"nickname"`
	Description string `json:"description"`
	Kind        string `json:"kind"`
}

func (h *WorkspaceHandler) Create(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req CreateWorkspaceRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	ownerID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	workspace, err := h.workspaceService.CreateWorkspace(
		r.Context(),
		ownerID,
		req.Name,
		req.Nickname,
		req.Description,
		req.Kind,
	)
	if err != nil {
		status := http.StatusInternalServerError

		if err.Error() == "workspace name is required" ||
			err.Error() == "invalid workspace kind" ||
			err.Error() == "invalid owner user ID" {
			status = http.StatusBadRequest
		}

		http.Error(w, `{"error":"could not create workspace"}`, status)
		return
	}

	w.WriteHeader(http.StatusCreated)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Workspace created successfully",
		"workspace": map[string]string{
			"id":           workspace.ID.String(),
			"name":         workspace.Name,
			"nickname":     workspace.Nickname.String,
			"description":  workspace.Description.String,
			"kind":         workspace.Kind,
			"theme":        workspace.Theme.String,
			"accent_color": workspace.AccentColor.String,
		},
	})
}
func (h *WorkspaceHandler) List(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	workspaces, err := h.workspaceService.ListWorkspacesForUser(r.Context(), userID)
	if err != nil {
		http.Error(w, `{"error":"could not list workspaces"}`, http.StatusInternalServerError)
		return
	}

	if workspaces == nil {
		workspaces = []repository.Workspace{}
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"workspaces": workspaces,
	})
}

func (h *WorkspaceHandler) ListMembers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	workspaceID := chi.URLParam(r, "workspaceID")

	members, err := h.workspaceService.ListWorkspaceMembers(
		r.Context(),
		workspaceID,
		userID,
	)
	if err != nil {
		if err.Error() == "invalid workspace ID" ||
			err.Error() == "invalid user ID" {
			http.Error(w, `{"error":"invalid ID"}`, http.StatusBadRequest)
			return
		}

		if err.Error() == "user is not a workspace member" {
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
			return
		}

		http.Error(w, `{"error":"could not list workspace members"}`, http.StatusInternalServerError)
		return
	}

	if members == nil {
		members = []repository.WorkspaceMember{}
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"members": members,
	})
}
