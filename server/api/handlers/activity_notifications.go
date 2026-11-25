package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/ben-anass/DouzTach/internal/auth"
	"github.com/ben-anass/DouzTach/server/activity"
	"github.com/ben-anass/DouzTach/server/notifications"
	"github.com/go-chi/chi/v5"
)

type ActivityHandler struct {
	activityService *activity.Service
}

func NewActivityHandler(service *activity.Service) *ActivityHandler {
	return &ActivityHandler{activityService: service}
}

func (h *ActivityHandler) ListWorkspaceActivity(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	userIDStr, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	workspaceID := parseUUID(chi.URLParam(r, "workspaceID"))
	userID := parseUUID(userIDStr)

	limit := int32(30)
	if l := r.URL.Query().Get("limit"); l != "" {
		val, err := strconv.Atoi(l)
		if err != nil || val < 1 || val > 100 {
			http.Error(w, `{"error":"limit must be between 1 and 100"}`, http.StatusBadRequest)
			return
		}
		limit = int32(val)
	}

	items, err := h.activityService.ListForWorkspace(r.Context(), workspaceID, userID, limit, 0)
	if err != nil {
		if errors.Is(err, activity.ErrNotMember) {
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
			return
		}
		if errors.Is(err, activity.ErrInvalidInput) {
			http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
			return
		}
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(items)
}

type NotificationHandler struct {
	notificationService *notifications.Service
}

func NewNotificationHandler(service *notifications.Service) *NotificationHandler {
	return &NotificationHandler{notificationService: service}
}

func (h *NotificationHandler) ListUserNotifications(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	userIDStr, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	userID := parseUUID(userIDStr)

	limit := int32(50)
	if l := r.URL.Query().Get("limit"); l != "" {
		val, err := strconv.Atoi(l)
		if err != nil || val < 1 || val > 100 {
			http.Error(w, `{"error":"limit must be between 1 and 100"}`, http.StatusBadRequest)
			return
		}
		limit = int32(val)
	}

	items, err := h.notificationService.ListForUser(r.Context(), userID, limit, 0)
	if err != nil {
		if errors.Is(err, notifications.ErrInvalidInput) {
			http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
			return
		}
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(items)
}

func (h *NotificationHandler) MarkRead(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	userIDStr, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	userID := parseUUID(userIDStr)
	notifID := parseUUID(chi.URLParam(r, "notificationID"))

	n, err := h.notificationService.MarkRead(r.Context(), userID, notifID)
	if err != nil {
		if errors.Is(err, notifications.ErrNotFound) {
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			return
		}
		if errors.Is(err, notifications.ErrInvalidInput) {
			http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
			return
		}
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(n)
}

func (h *NotificationHandler) MarkAllRead(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	userIDStr, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	userID := parseUUID(userIDStr)
	if err := h.notificationService.MarkAllRead(r.Context(), userID); err != nil {
		if errors.Is(err, notifications.ErrInvalidInput) {
			http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
			return
		}
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
