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

type CreateInvitationRequest struct {
	Email string `json:"email"`
	Role  string `json:"role"`
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

func (h *WorkspaceHandler) CreateInvitation(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	workspaceID := chi.URLParam(r, "workspaceID")

	var req CreateInvitationRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	invitation, err := h.workspaceService.CreateInvitation(
		r.Context(),
		workspaceID,
		userID,
		req.Email,
		req.Role,
	)

	if err != nil {
		switch err.Error() {
		case "invitee email is required",
			"invalid invitation role",
			"invalid workspace ID",
			"invalid inviter user ID":
			http.Error(w, `{"error":"invalid invitation data"}`, http.StatusBadRequest)

		case "user is not a workspace member",
			"you do not have permission to invite members":
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)

		case "user is already a workspace member",
			"a pending invitation already exists for this email":
			http.Error(w, `{"error":"invitation conflict"}`, http.StatusConflict)

		default:
			http.Error(w, `{"error":"could not create invitation"}`, http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusCreated)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Invitation created successfully",
		"invitation": map[string]interface{}{
			"id":           invitation.ID.String(),
			"workspace_id": invitation.WorkspaceID.String(),
			"email":        invitation.Email.String,
			"role":         invitation.Role,
			"status":       invitation.Status,
			"token":        invitation.Token.String(),
			"expires_at":   invitation.ExpiresAt.Time,
		},
	})
}

func writeInvitationActionError(w http.ResponseWriter, err error) {
	switch err.Error() {
	case "invalid invitation token", "invalid user ID":
		http.Error(w, `{"error":"invalid invitation data"}`, http.StatusBadRequest)

	case "invitation not found":
		http.Error(w, `{"error":"invitation not found"}`, http.StatusNotFound)

	case "invitation does not belong to this user",
		"invitation belongs to another user":
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)

	case "invitation is not pending",
		"user is already a workspace member":
		http.Error(w, `{"error":"invitation conflict"}`, http.StatusConflict)

	case "invitation has expired":
		http.Error(w, `{"error":"invitation expired"}`, http.StatusGone)

	default:
		http.Error(w, `{"error":"invitation operation failed"}`, http.StatusInternalServerError)
	}
}

func (h *WorkspaceHandler) AcceptInvitation(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	token := chi.URLParam(r, "token")

	invitation, err := h.workspaceService.AcceptInvitation(
		r.Context(),
		token,
		userID,
	)
	if err != nil {
		writeInvitationActionError(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Invitation accepted successfully",
		"invitation": map[string]interface{}{
			"id":           invitation.ID.String(),
			"workspace_id": invitation.WorkspaceID.String(),
			"email":        invitation.Email.String,
			"role":         invitation.Role,
			"status":       invitation.Status,
		},
	})
}

func (h *WorkspaceHandler) RejectInvitation(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	token := chi.URLParam(r, "token")

	invitation, err := h.workspaceService.RejectInvitation(
		r.Context(),
		token,
		userID,
	)
	if err != nil {
		writeInvitationActionError(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Invitation rejected successfully",
		"invitation": map[string]interface{}{
			"id":           invitation.ID.String(),
			"workspace_id": invitation.WorkspaceID.String(),
			"email":        invitation.Email.String,
			"role":         invitation.Role,
			"status":       invitation.Status,
		},
	})
}

func (h *WorkspaceHandler) ListInvitations(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	workspaceID := chi.URLParam(r, "workspaceID")

	invitations, err := h.workspaceService.ListWorkspaceInvitations(
		r.Context(),
		workspaceID,
		userID,
	)

	if err != nil {
		switch err.Error() {
		case "invalid workspace ID", "invalid user ID":
			http.Error(w, `{"error":"invalid ID"}`, http.StatusBadRequest)

		case "user is not a workspace member",
			"you do not have permission to view invitations":
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)

		default:
			http.Error(w, `{"error":"could not list invitations"}`, http.StatusInternalServerError)
		}
		return
	}

	items := make([]map[string]interface{}, 0, len(invitations))

	for _, invitation := range invitations {
		var inviteeID interface{}

		if invitation.InviteeUserID.Valid {
			inviteeID = invitation.InviteeUserID.String()
		}

		items = append(items, map[string]interface{}{
			"id":              invitation.ID.String(),
			"workspace_id":    invitation.WorkspaceID.String(),
			"inviter_user_id": invitation.InviterUserID.String(),
			"invitee_user_id": inviteeID,
			"email":           invitation.Email.String,
			"role":            invitation.Role,
			"status":          invitation.Status,
			"expires_at":      invitation.ExpiresAt.Time,
			"created_at":      invitation.CreatedAt.Time,
		})
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"invitations": items,
	})
}

func (h *WorkspaceHandler) CancelInvitation(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	workspaceID := chi.URLParam(r, "workspaceID")
	invitationID := chi.URLParam(r, "invitationID")

	invitation, err := h.workspaceService.CancelInvitation(
		r.Context(),
		workspaceID,
		invitationID,
		userID,
	)

	if err != nil {
		switch err.Error() {
		case "invalid workspace ID", "invalid invitation ID", "invalid user ID":
			http.Error(w, `{"error":"invalid ID"}`, http.StatusBadRequest)

		case "user is not a workspace member",
			"you do not have permission to cancel invitations":
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)

		case "invitation not found or not pending":
			http.Error(w, `{"error":"invitation conflict"}`, http.StatusConflict)

		default:
			http.Error(w, `{"error":"could not cancel invitation"}`, http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusOK)

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Invitation cancelled successfully",
		"invitation": map[string]interface{}{
			"id":           invitation.ID.String(),
			"workspace_id": invitation.WorkspaceID.String(),
			"email":        invitation.Email.String,
			"role":         invitation.Role,
			"status":       invitation.Status,
		},
	})
}

func (h *WorkspaceHandler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	requesterID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	workspaceID := chi.URLParam(r, "workspaceID")
	targetUserID := chi.URLParam(r, "userID")

	member, err := h.workspaceService.RemoveMember(
		r.Context(),
		workspaceID,
		requesterID,
		targetUserID,
	)

	if err != nil {
		switch err.Error() {
		case "invalid workspace ID",
			"invalid user ID",
			"invalid target user ID":

			http.Error(w, `{"error":"invalid ID"}`, http.StatusBadRequest)

		case "user is not a workspace member",
			"you do not have permission to remove members",
			"admin cannot remove this member":

			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)

		case "target user is not a workspace member":

			http.Error(w, `{"error":"member not found"}`, http.StatusNotFound)

		case "cannot remove workspace owner",
			"cannot remove yourself":

			http.Error(w, `{"error":"member removal conflict"}`, http.StatusConflict)

		default:
			http.Error(w, `{"error":"could not remove member"}`, http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusOK)

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Workspace member removed successfully",
		"member": map[string]interface{}{
			"id":           member.ID.String(),
			"workspace_id": member.WorkspaceID.String(),
			"user_id":      member.UserID.String(),
			"role":         member.Role,
			"joined_at":    member.JoinedAt.Time,
		},
	})
}

func (h *WorkspaceHandler) ChangeMemberRole(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	requesterID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	workspaceID := chi.URLParam(r, "workspaceID")
	targetUserID := chi.URLParam(r, "userID")

	var req struct {
		Role string `json:"role"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	member, err := h.workspaceService.ChangeMemberRole(
		r.Context(),
		workspaceID,
		requesterID,
		targetUserID,
		req.Role,
	)

	if err != nil {
		switch err.Error() {
		case "invalid workspace ID",
			"invalid user ID",
			"invalid target user ID",
			"invalid member role":

			http.Error(w, `{"error":"invalid member data"}`, http.StatusBadRequest)

		case "user is not a workspace member",
			"you do not have permission to change member roles",
			"admin cannot change this member role":

			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)

		case "target user is not a workspace member":

			http.Error(w, `{"error":"member not found"}`, http.StatusNotFound)

		case "cannot change workspace owner role":

			http.Error(w, `{"error":"cannot change owner role"}`, http.StatusConflict)

		default:
			http.Error(w, `{"error":"could not change member role"}`, http.StatusInternalServerError)
		}
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Workspace member role updated successfully",
		"member": map[string]interface{}{
			"id":           member.ID.String(),
			"workspace_id": member.WorkspaceID.String(),
			"user_id":      member.UserID.String(),
			"role":         member.Role,
			"joined_at":    member.JoinedAt.Time,
		},
	})
}

func (h *WorkspaceHandler) LeaveWorkspace(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	workspaceID := chi.URLParam(r, "workspaceID")

	member, err := h.workspaceService.LeaveWorkspace(
		r.Context(),
		workspaceID,
		userID,
	)

	if err != nil {
		switch err.Error() {
		case "invalid workspace ID", "invalid user ID":
			http.Error(w, `{"error":"invalid ID"}`, http.StatusBadRequest)

		case "user is not a workspace member":
			http.Error(w, `{"error":"membership not found"}`, http.StatusNotFound)

		case "workspace owner cannot leave before transferring ownership":
			http.Error(w, `{"error":"transfer ownership before leaving"}`, http.StatusConflict)

		default:
			http.Error(w, `{"error":"could not leave workspace"}`, http.StatusInternalServerError)
		}
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "You left the workspace successfully",
		"membership": map[string]interface{}{
			"id":           member.ID.String(),
			"workspace_id": member.WorkspaceID.String(),
			"user_id":      member.UserID.String(),
			"role":         member.Role,
		},
	})
}
