package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/ben-anass/DouzTach/internal/auth"
	"github.com/ben-anass/DouzTach/server/activity"
	"github.com/ben-anass/DouzTach/server/scrum"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type ScrumHandler struct {
	scrumService *scrum.Service
}

func NewScrumHandler(service *scrum.Service) *ScrumHandler {
	return &ScrumHandler{scrumService: service}
}

func (h *ScrumHandler) RequireWorkspaceMember(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userIDStr, ok := auth.UserIDFromContext(r.Context())
		if !ok {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		workspaceID := parseUUID(chi.URLParam(r, "workspaceID"))
		userID := parseUUID(userIDStr)
		member, err := h.scrumService.IsWorkspaceMember(r.Context(), workspaceID, userID)
		if err != nil {
			writeScrumError(w, err)
			return
		}
		if !member {
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func parseUUID(s string) pgtype.UUID {
	var u pgtype.UUID
	_ = u.Scan(s)
	return u
}

func int4FromPointer(value *int32) pgtype.Int4 {
	if value == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: *value, Valid: true}
}

func parseDate(s string) pgtype.Date {
	var d pgtype.Date
	if strings.TrimSpace(s) == "" {
		return d
	}
	t, err := time.Parse("2006-01-02", strings.TrimSpace(s))
	if err == nil {
		_ = d.Scan(t)
	}
	return d
}

func validOptionalDate(value string) bool {
	if strings.TrimSpace(value) == "" {
		return true
	}
	_, err := time.Parse("2006-01-02", strings.TrimSpace(value))
	return err == nil
}

func validOptionalUUID(value string) bool {
	if strings.TrimSpace(value) == "" {
		return true
	}
	var id pgtype.UUID
	return id.Scan(strings.TrimSpace(value)) == nil && id.Valid
}

// --- Epics ---

type CreateEpicRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Priority    string `json:"priority"`
	Color       string `json:"color"`
	StartDate   string `json:"start_date"`
	DueDate     string `json:"due_date"`
}

func (h *ScrumHandler) CreateEpic(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	actorIDStr, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	workspaceID := parseUUID(chi.URLParam(r, "workspaceID"))
	actorID := parseUUID(actorIDStr)

	var req CreateEpicRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if !validOptionalDate(req.StartDate) || !validOptionalDate(req.DueDate) {
		http.Error(w, `{"error":"invalid date; use YYYY-MM-DD"}`, http.StatusBadRequest)
		return
	}

	epic, err := h.scrumService.CreateEpic(r.Context(), scrum.CreateEpicInput{
		WorkspaceID: workspaceID,
		ActorUserID: actorID,
		Title:       req.Title,
		Description: req.Description,
		Status:      req.Status,
		Priority:    req.Priority,
		Color:       req.Color,
		StartDate:   parseDate(req.StartDate),
		EndDate:     parseDate(req.DueDate),
	})
	if err != nil {
		writeScrumError(w, err)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(epic)
}

func (h *ScrumHandler) ListEpics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	workspaceID := parseUUID(chi.URLParam(r, "workspaceID"))
	epics, err := h.scrumService.ListEpics(r.Context(), workspaceID)
	if err != nil {
		writeScrumError(w, err)
		return
	}
	json.NewEncoder(w).Encode(epics)
}

func (h *ScrumHandler) GetEpic(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	workspaceID := parseUUID(chi.URLParam(r, "workspaceID"))
	epicID := parseUUID(chi.URLParam(r, "epicID"))

	epic, err := h.scrumService.GetEpic(r.Context(), workspaceID, epicID)
	if err != nil {
		writeScrumError(w, err)
		return
	}
	json.NewEncoder(w).Encode(epic)
}

func (h *ScrumHandler) UpdateEpic(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	actorIDStr, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	workspaceID := parseUUID(chi.URLParam(r, "workspaceID"))
	epicID := parseUUID(chi.URLParam(r, "epicID"))
	actorID := parseUUID(actorIDStr)

	var req CreateEpicRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if !validOptionalDate(req.StartDate) || !validOptionalDate(req.DueDate) {
		http.Error(w, `{"error":"invalid date; use YYYY-MM-DD"}`, http.StatusBadRequest)
		return
	}

	epic, err := h.scrumService.UpdateEpic(r.Context(), scrum.UpdateEpicInput{
		EpicID: epicID,
		CreateEpicInput: scrum.CreateEpicInput{
			WorkspaceID: workspaceID,
			ActorUserID: actorID,
			Title:       req.Title,
			Description: req.Description,
			Status:      req.Status,
			Priority:    req.Priority,
			Color:       req.Color,
			StartDate:   parseDate(req.StartDate),
			EndDate:     parseDate(req.DueDate),
		},
	})
	if err != nil {
		writeScrumError(w, err)
		return
	}
	json.NewEncoder(w).Encode(epic)
}

func (h *ScrumHandler) DeleteEpic(w http.ResponseWriter, r *http.Request) {
	actorIDStr, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	workspaceID := parseUUID(chi.URLParam(r, "workspaceID"))
	epicID := parseUUID(chi.URLParam(r, "epicID"))
	actorID := parseUUID(actorIDStr)

	if err := h.scrumService.DeleteEpic(r.Context(), workspaceID, actorID, epicID); err != nil {
		writeScrumError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- User Stories ---

type CreateUserStoryRequest struct {
	EpicID             string `json:"epic_id"`
	Title              string `json:"title"`
	Description        string `json:"description"`
	AcceptanceCriteria string `json:"acceptance_criteria"`
	Status             string `json:"status"`
	Priority           string `json:"priority"`
	AssigneeID         string `json:"assignee_id"`
	StoryPoints        *int32 `json:"story_points"`
}

func (h *ScrumHandler) CreateUserStory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	actorIDStr, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	workspaceID := parseUUID(chi.URLParam(r, "workspaceID"))
	actorID := parseUUID(actorIDStr)

	var req CreateUserStoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if !validOptionalUUID(req.EpicID) || !validOptionalUUID(req.AssigneeID) {
		http.Error(w, `{"error":"invalid epic or assignee ID"}`, http.StatusBadRequest)
		return
	}
	points := int4FromPointer(req.StoryPoints)
	story, err := h.scrumService.CreateUserStory(r.Context(), scrum.CreateUserStoryInput{
		WorkspaceID:        workspaceID,
		ActorUserID:        actorID,
		EpicID:             parseUUID(req.EpicID),
		Title:              req.Title,
		Description:        req.Description,
		AcceptanceCriteria: req.AcceptanceCriteria,
		Status:             req.Status,
		Priority:           req.Priority,
		AssigneeID:         parseUUID(req.AssigneeID),
		StoryPoints:        points,
	})
	if err != nil {
		writeScrumError(w, err)
		return
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(story)
}

func (h *ScrumHandler) ListUserStories(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	workspaceID := parseUUID(chi.URLParam(r, "workspaceID"))
	stories, err := h.scrumService.ListUserStories(r.Context(), workspaceID)
	if err != nil {
		writeScrumError(w, err)
		return
	}
	json.NewEncoder(w).Encode(stories)
}

func (h *ScrumHandler) GetUserStory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	workspaceID := parseUUID(chi.URLParam(r, "workspaceID"))
	storyID := parseUUID(chi.URLParam(r, "storyID"))
	story, err := h.scrumService.GetUserStory(r.Context(), workspaceID, storyID)
	if err != nil {
		writeScrumError(w, err)
		return
	}
	json.NewEncoder(w).Encode(story)
}

func (h *ScrumHandler) UpdateUserStory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	actorIDStr, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	workspaceID := parseUUID(chi.URLParam(r, "workspaceID"))
	storyID := parseUUID(chi.URLParam(r, "storyID"))
	actorID := parseUUID(actorIDStr)
	var req CreateUserStoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if !validOptionalUUID(req.EpicID) || !validOptionalUUID(req.AssigneeID) {
		http.Error(w, `{"error":"invalid epic or assignee ID"}`, http.StatusBadRequest)
		return
	}
	points := int4FromPointer(req.StoryPoints)
	story, err := h.scrumService.UpdateUserStory(r.Context(), scrum.UpdateUserStoryInput{
		UserStoryID: storyID,
		CreateUserStoryInput: scrum.CreateUserStoryInput{
			WorkspaceID:        workspaceID,
			ActorUserID:        actorID,
			EpicID:             parseUUID(req.EpicID),
			Title:              req.Title,
			Description:        req.Description,
			AcceptanceCriteria: req.AcceptanceCriteria,
			Status:             req.Status,
			Priority:           req.Priority,
			AssigneeID:         parseUUID(req.AssigneeID),
			StoryPoints:        points,
		},
	})
	if err != nil {
		writeScrumError(w, err)
		return
	}
	json.NewEncoder(w).Encode(story)
}

func (h *ScrumHandler) DeleteUserStory(w http.ResponseWriter, r *http.Request) {
	actorIDStr, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	workspaceID := parseUUID(chi.URLParam(r, "workspaceID"))
	storyID := parseUUID(chi.URLParam(r, "storyID"))
	actorID := parseUUID(actorIDStr)
	if err := h.scrumService.DeleteUserStory(r.Context(), workspaceID, actorID, storyID); err != nil {
		writeScrumError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Tasks / Work Items ---

type CreateWorkItemRequest struct {
	UserStoryID string `json:"user_story_id"`
	ParentID    string `json:"parent_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	ItemType    string `json:"item_type"`
	Status      string `json:"status"`
	Priority    string `json:"priority"`
	AssigneeID  string `json:"assignee_id"`
	SprintID    string `json:"sprint_id"`
	DueDate     string `json:"due_date"`
	StoryPoints *int32 `json:"story_points"`
}

func (h *ScrumHandler) CreateWorkItem(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	actorIDStr, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	workspaceID := parseUUID(chi.URLParam(r, "workspaceID"))
	actorID := parseUUID(actorIDStr)

	var req CreateWorkItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if !validOptionalUUID(req.UserStoryID) || !validOptionalUUID(req.ParentID) ||
		!validOptionalUUID(req.AssigneeID) || !validOptionalUUID(req.SprintID) ||
		!validOptionalDate(req.DueDate) {
		http.Error(w, `{"error":"invalid work-item reference or due date"}`, http.StatusBadRequest)
		return
	}

	sp := int4FromPointer(req.StoryPoints)

	item, err := h.scrumService.CreateWorkItem(r.Context(), scrum.CreateWorkItemInput{
		WorkspaceID: workspaceID,
		ActorUserID: actorID,
		UserStoryID: parseUUID(req.UserStoryID),
		ParentID:    parseUUID(req.ParentID),
		Title:       req.Title,
		Description: req.Description,
		ItemType:    req.ItemType,
		Status:      req.Status,
		Priority:    req.Priority,
		AssigneeID:  parseUUID(req.AssigneeID),
		SprintID:    parseUUID(req.SprintID),
		DueDate:     parseDate(req.DueDate),
		StoryPoints: sp,
	})
	if err != nil {
		writeScrumError(w, err)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(item)
}

func (h *ScrumHandler) ListWorkItems(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	workspaceID := parseUUID(chi.URLParam(r, "workspaceID"))
	items, err := h.scrumService.ListWorkItems(r.Context(), workspaceID)
	if err != nil {
		writeScrumError(w, err)
		return
	}
	json.NewEncoder(w).Encode(items)
}

func (h *ScrumHandler) GetWorkItem(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	workspaceID := parseUUID(chi.URLParam(r, "workspaceID"))
	itemID := parseUUID(chi.URLParam(r, "itemID"))

	item, err := h.scrumService.GetWorkItem(r.Context(), workspaceID, itemID)
	if err != nil {
		writeScrumError(w, err)
		return
	}
	json.NewEncoder(w).Encode(item)
}

func (h *ScrumHandler) UpdateWorkItem(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	actorIDStr, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	workspaceID := parseUUID(chi.URLParam(r, "workspaceID"))
	itemID := parseUUID(chi.URLParam(r, "itemID"))
	actorID := parseUUID(actorIDStr)

	var req CreateWorkItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if !validOptionalUUID(req.UserStoryID) || !validOptionalUUID(req.ParentID) ||
		!validOptionalUUID(req.AssigneeID) || !validOptionalUUID(req.SprintID) ||
		!validOptionalDate(req.DueDate) {
		http.Error(w, `{"error":"invalid work-item reference or due date"}`, http.StatusBadRequest)
		return
	}

	sp := int4FromPointer(req.StoryPoints)

	item, err := h.scrumService.UpdateWorkItem(r.Context(), scrum.UpdateWorkItemInput{
		WorkItemID: itemID,
		CreateWorkItemInput: scrum.CreateWorkItemInput{
			WorkspaceID: workspaceID,
			ActorUserID: actorID,
			UserStoryID: parseUUID(req.UserStoryID),
			ParentID:    parseUUID(req.ParentID),
			Title:       req.Title,
			Description: req.Description,
			ItemType:    req.ItemType,
			Status:      req.Status,
			Priority:    req.Priority,
			AssigneeID:  parseUUID(req.AssigneeID),
			SprintID:    parseUUID(req.SprintID),
			DueDate:     parseDate(req.DueDate),
			StoryPoints: sp,
		},
	})
	if err != nil {
		writeScrumError(w, err)
		return
	}
	json.NewEncoder(w).Encode(item)
}

func (h *ScrumHandler) DeleteWorkItem(w http.ResponseWriter, r *http.Request) {
	actorIDStr, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	workspaceID := parseUUID(chi.URLParam(r, "workspaceID"))
	itemID := parseUUID(chi.URLParam(r, "itemID"))
	actorID := parseUUID(actorIDStr)

	if err := h.scrumService.DeleteWorkItem(r.Context(), workspaceID, actorID, itemID); err != nil {
		writeScrumError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Sprints ---

type CreateSprintRequest struct {
	Name      string `json:"name"`
	Goal      string `json:"goal"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
}

func (h *ScrumHandler) CreateSprint(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	actorIDStr, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	workspaceID := parseUUID(chi.URLParam(r, "workspaceID"))
	actorID := parseUUID(actorIDStr)

	var req CreateSprintRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if !validOptionalDate(req.StartDate) || !validOptionalDate(req.EndDate) {
		http.Error(w, `{"error":"invalid sprint date; use YYYY-MM-DD"}`, http.StatusBadRequest)
		return
	}

	sprint, err := h.scrumService.CreateSprint(r.Context(), scrum.CreateSprintInput{
		WorkspaceID: workspaceID,
		ActorUserID: actorID,
		Name:        req.Name,
		Goal:        req.Goal,
		StartDate:   parseDate(req.StartDate),
		EndDate:     parseDate(req.EndDate),
	})
	if err != nil {
		writeScrumError(w, err)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(sprint)
}

func (h *ScrumHandler) ListSprints(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	workspaceID := parseUUID(chi.URLParam(r, "workspaceID"))
	sprints, err := h.scrumService.ListSprints(r.Context(), workspaceID)
	if err != nil {
		writeScrumError(w, err)
		return
	}
	json.NewEncoder(w).Encode(sprints)
}

func (h *ScrumHandler) GetSprint(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	workspaceID := parseUUID(chi.URLParam(r, "workspaceID"))
	sprintID := parseUUID(chi.URLParam(r, "sprintID"))

	sprint, err := h.scrumService.GetSprint(r.Context(), workspaceID, sprintID)
	if err != nil {
		writeScrumError(w, err)
		return
	}
	json.NewEncoder(w).Encode(sprint)
}

func (h *ScrumHandler) UpdateSprint(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	actorIDStr, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	workspaceID := parseUUID(chi.URLParam(r, "workspaceID"))
	sprintID := parseUUID(chi.URLParam(r, "sprintID"))
	actorID := parseUUID(actorIDStr)

	var req CreateSprintRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if !validOptionalDate(req.StartDate) || !validOptionalDate(req.EndDate) {
		http.Error(w, `{"error":"invalid sprint date; use YYYY-MM-DD"}`, http.StatusBadRequest)
		return
	}

	sprint, err := h.scrumService.UpdateSprint(r.Context(), scrum.UpdateSprintInput{
		SprintID: sprintID,
		CreateSprintInput: scrum.CreateSprintInput{
			WorkspaceID: workspaceID,
			ActorUserID: actorID,
			Name:        req.Name,
			Goal:        req.Goal,
			StartDate:   parseDate(req.StartDate),
			EndDate:     parseDate(req.EndDate),
		},
	})
	if err != nil {
		writeScrumError(w, err)
		return
	}
	json.NewEncoder(w).Encode(sprint)
}

func (h *ScrumHandler) StartSprint(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	actorIDStr, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	workspaceID := parseUUID(chi.URLParam(r, "workspaceID"))
	sprintID := parseUUID(chi.URLParam(r, "sprintID"))
	actorID := parseUUID(actorIDStr)

	sprint, err := h.scrumService.StartSprint(r.Context(), workspaceID, actorID, sprintID)
	if err != nil {
		writeScrumError(w, err)
		return
	}
	json.NewEncoder(w).Encode(sprint)
}

func (h *ScrumHandler) CompleteSprint(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	actorIDStr, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	workspaceID := parseUUID(chi.URLParam(r, "workspaceID"))
	sprintID := parseUUID(chi.URLParam(r, "sprintID"))
	actorID := parseUUID(actorIDStr)

	sprint, err := h.scrumService.CompleteSprint(r.Context(), workspaceID, actorID, sprintID)
	if err != nil {
		writeScrumError(w, err)
		return
	}
	json.NewEncoder(w).Encode(sprint)
}

func (h *ScrumHandler) ReopenSprint(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	actorIDStr, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	workspaceID := parseUUID(chi.URLParam(r, "workspaceID"))
	sprintID := parseUUID(chi.URLParam(r, "sprintID"))
	actorID := parseUUID(actorIDStr)

	sprint, err := h.scrumService.ReopenSprint(r.Context(), workspaceID, actorID, sprintID)
	if err != nil {
		writeScrumError(w, err)
		return
	}
	json.NewEncoder(w).Encode(sprint)
}

func (h *ScrumHandler) DeleteSprint(w http.ResponseWriter, r *http.Request) {
	actorIDStr, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	workspaceID := parseUUID(chi.URLParam(r, "workspaceID"))
	sprintID := parseUUID(chi.URLParam(r, "sprintID"))
	actorID := parseUUID(actorIDStr)
	if err := h.scrumService.DeleteSprint(r.Context(), workspaceID, actorID, sprintID); err != nil {
		writeScrumError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ScrumHandler) ListBacklog(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	workspaceID := parseUUID(chi.URLParam(r, "workspaceID"))
	stories, err := h.scrumService.ListBacklog(r.Context(), workspaceID)
	if err != nil {
		writeScrumError(w, err)
		return
	}
	json.NewEncoder(w).Encode(stories)
}

func (h *ScrumHandler) SetBacklogStoryPosition(w http.ResponseWriter, r *http.Request) {
	actorIDStr, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	var req struct {
		Position int32 `json:"position"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	workspaceID := parseUUID(chi.URLParam(r, "workspaceID"))
	storyID := parseUUID(chi.URLParam(r, "storyID"))
	actorID := parseUUID(actorIDStr)
	if err := h.scrumService.SetBacklogStoryPosition(r.Context(), workspaceID, actorID, storyID, req.Position); err != nil {
		writeScrumError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type SprintStoryRequest struct {
	StoryID  string `json:"story_id"`
	Position *int32 `json:"position"`
}

func (h *ScrumHandler) AddStoryToSprint(w http.ResponseWriter, r *http.Request) {
	actorIDStr, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	var req SprintStoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	workspaceID := parseUUID(chi.URLParam(r, "workspaceID"))
	sprintID := parseUUID(chi.URLParam(r, "sprintID"))
	actorID := parseUUID(actorIDStr)
	storyID := parseUUID(req.StoryID)
	if err := h.scrumService.AddStoryToSprint(r.Context(), workspaceID, actorID, sprintID, storyID); err != nil {
		writeScrumError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ScrumHandler) RemoveStoryFromSprint(w http.ResponseWriter, r *http.Request) {
	actorIDStr, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	storyIDStr := chi.URLParam(r, "storyID")
	if storyIDStr == "" {
		var req SprintStoryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
			return
		}
		storyIDStr = req.StoryID
	}
	workspaceID := parseUUID(chi.URLParam(r, "workspaceID"))
	sprintID := parseUUID(chi.URLParam(r, "sprintID"))
	actorID := parseUUID(actorIDStr)
	if err := h.scrumService.RemoveStoryFromSprint(r.Context(), workspaceID, actorID, sprintID, parseUUID(storyIDStr)); err != nil {
		writeScrumError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ScrumHandler) ListSprintStories(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	workspaceID := parseUUID(chi.URLParam(r, "workspaceID"))
	sprintID := parseUUID(chi.URLParam(r, "sprintID"))
	stories, err := h.scrumService.ListStoriesForSprint(r.Context(), workspaceID, sprintID)
	if err != nil {
		writeScrumError(w, err)
		return
	}
	json.NewEncoder(w).Encode(stories)
}

func (h *ScrumHandler) SprintBoard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	workspaceID := parseUUID(chi.URLParam(r, "workspaceID"))
	sprintID := parseUUID(chi.URLParam(r, "sprintID"))
	report, err := h.scrumService.GetSprintReport(r.Context(), workspaceID, sprintID)
	if err != nil {
		writeScrumError(w, err)
		return
	}
	stories, err := h.scrumService.ListStoriesForSprint(r.Context(), workspaceID, sprintID)
	if err != nil {
		writeScrumError(w, err)
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"report":  report,
		"stories": stories,
	})
}

func writeScrumError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	message := err.Error()
	switch {
	case errors.Is(err, activity.ErrNotMember), errors.Is(err, activity.ErrReadOnly):
		status = http.StatusForbidden
		message = "forbidden"
	case errors.Is(err, scrum.ErrNotFound):
		status = http.StatusNotFound
		message = "not found"
	case errors.Is(err, scrum.ErrInvalidInput), errors.Is(err, scrum.ErrInvalidReference):
		status = http.StatusBadRequest
	case errors.Is(err, scrum.ErrActiveSprint), errors.Is(err, scrum.ErrInvalidTransition),
		errors.Is(err, scrum.ErrStoryAlreadyPlanned), errors.Is(err, scrum.ErrActiveChildren):
		status = http.StatusConflict
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
