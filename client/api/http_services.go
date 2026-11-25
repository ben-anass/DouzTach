package api

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type httpAuthService struct {
	client *Client
}

func (s *httpAuthService) Login(ctx context.Context, req LoginRequest) (*AuthResult, error) {
	body := map[string]string{
		"email":    req.Email,
		"password": req.Password,
	}
	var resp struct {
		Message string `json:"message"`
		Token   string `json:"token"`
		User    struct {
			ID               string `json:"id"`
			Username         string `json:"username"`
			Nickname         string `json:"nickname"`
			Email            string `json:"email"`
			FavoriteLanguage string `json:"favorite_language"`
		} `json:"user"`
	}

	if err := s.client.Do(ctx, "POST", "/api/v1/auth/login", nil, body, &resp); err != nil {
		return nil, err
	}

	exp := time.Now().Add(24 * time.Hour)
	return &AuthResult{
		AccessToken: resp.Token,
		TokenType:   "Bearer",
		ExpiresAt:   &exp,
		User: User{
			ID:       resp.User.ID,
			Name:     resp.User.Nickname,
			Username: resp.User.Username,
			Email:    resp.User.Email,
		},
	}, nil
}

func (s *httpAuthService) Register(ctx context.Context, req RegisterRequest) (*AuthResult, error) {
	username := req.Name
	if strings.Contains(req.Name, " ") {
		username = strings.ToLower(strings.ReplaceAll(req.Name, " ", "_"))
	}
	body := map[string]string{
		"username":          username,
		"nickname":          req.Name,
		"email":             req.Email,
		"password":          req.Password,
		"favorite_language": "Go",
	}
	var resp struct {
		Message string `json:"message"`
		Token   string `json:"token"`
		User    struct {
			ID       string `json:"id"`
			Username string `json:"username"`
			Nickname string `json:"nickname"`
			Email    string `json:"email"`
		} `json:"user"`
	}

	if err := s.client.Do(ctx, "POST", "/api/v1/auth/register", nil, body, &resp); err != nil {
		return nil, err
	}

	// Auto login after registration
	return s.Login(ctx, LoginRequest{Email: req.Email, Password: req.Password})
}

func (s *httpAuthService) Logout(ctx context.Context) error {
	return nil
}

func (s *httpAuthService) Profile(ctx context.Context) (*User, error) {
	var resp struct {
		ID       string `json:"id"`
		Username string `json:"username"`
		Nickname string `json:"nickname"`
		Email    string `json:"email"`
	}
	if err := s.client.Do(ctx, "GET", "/api/v1/me", nil, nil, &resp); err != nil {
		return nil, err
	}
	return &User{
		ID:       resp.ID,
		Name:     resp.Nickname,
		Username: resp.Username,
		Email:    resp.Email,
	}, nil
}

// --- Workspaces ---

type httpWorkspaceService struct {
	client *Client
}

func (s *httpWorkspaceService) List(ctx context.Context) ([]Workspace, error) {
	var resp struct {
		Workspaces []struct {
			ID          string    `json:"id"`
			Name        string    `json:"name"`
			Description string    `json:"description"`
			Role        string    `json:"role"`
			MemberCount int       `json:"member_count"`
			CreatedAt   time.Time `json:"created_at"`
		} `json:"workspaces"`
	}
	if err := s.client.Do(ctx, "GET", "/api/v1/workspaces", nil, nil, &resp); err != nil {
		return nil, err
	}
	res := make([]Workspace, 0, len(resp.Workspaces))
	for _, w := range resp.Workspaces {
		res = append(res, Workspace{
			ID:          w.ID,
			Name:        w.Name,
			Description: w.Description,
			Role:        w.Role,
			MemberCount: w.MemberCount,
			CreatedAt:   w.CreatedAt,
		})
	}
	return res, nil
}

func (s *httpWorkspaceService) Create(ctx context.Context, req CreateWorkspaceRequest) (*Workspace, error) {
	body := map[string]string{
		"name":        req.Name,
		"nickname":    strings.ToLower(strings.ReplaceAll(req.Name, " ", "_")),
		"description": req.Description,
		"kind":        "team",
	}
	var resp struct {
		Message   string `json:"message"`
		Workspace struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			Description string `json:"description"`
		} `json:"workspace"`
	}
	if err := s.client.Do(ctx, "POST", "/api/v1/workspaces", nil, body, &resp); err != nil {
		return nil, err
	}
	return &Workspace{
		ID:          resp.Workspace.ID,
		Name:        resp.Workspace.Name,
		Description: resp.Workspace.Description,
		CreatedAt:   time.Now(),
	}, nil
}

func (s *httpWorkspaceService) Members(ctx context.Context, workspaceID string) ([]Member, error) {
	var resp struct {
		Members []struct {
			ID       string    `json:"id"`
			UserID   string    `json:"user_id"`
			Role     string    `json:"role"`
			JoinedAt time.Time `json:"joined_at"`
		} `json:"members"`
	}
	path := fmt.Sprintf("/api/v1/workspaces/%s/members", workspaceID)
	if err := s.client.Do(ctx, "GET", path, nil, nil, &resp); err != nil {
		return nil, err
	}
	members := make([]Member, 0, len(resp.Members))
	for _, m := range resp.Members {
		members = append(members, Member{
			User: User{
				ID:   m.UserID,
				Name: m.UserID,
			},
			Role:     m.Role,
			JoinedAt: m.JoinedAt,
		})
	}
	return members, nil
}

func (s *httpWorkspaceService) CreateInvitation(ctx context.Context, workspaceID, email, role string) (*WorkspaceInvitation, error) {
	var resp struct {
		Invitation WorkspaceInvitation `json:"invitation"`
	}
	path := fmt.Sprintf("/api/v1/workspaces/%s/invitations", workspaceID)
	if err := s.client.Do(ctx, "POST", path, nil, map[string]string{
		"email": email,
		"role":  role,
	}, &resp); err != nil {
		return nil, err
	}
	return &resp.Invitation, nil
}

func (s *httpWorkspaceService) AcceptInvitation(ctx context.Context, token string) error {
	path := fmt.Sprintf("/api/v1/invitations/%s/accept", url.PathEscape(token))
	return s.client.Do(ctx, "POST", path, nil, nil, nil)
}

// --- Tasks ---

type httpTaskService struct {
	client *Client
}

func (s *httpTaskService) List(ctx context.Context, filter TaskFilter) ([]Task, error) {
	if filter.WorkspaceID == "" {
		return nil, errors.New("workspace ID is required to list tasks")
	}
	path := fmt.Sprintf("/api/v1/workspaces/%s/tasks", filter.WorkspaceID)
	var raw []struct {
		ID          string     `json:"id"`
		Title       string     `json:"title"`
		Description string     `json:"description"`
		ItemType    string     `json:"item_type"`
		Status      string     `json:"status"`
		Priority    string     `json:"priority"`
		AssigneeID  *string    `json:"assignee_id"`
		SprintID    *string    `json:"sprint_id"`
		DueDate     *time.Time `json:"due_date"`
		CreatedAt   time.Time  `json:"created_at"`
		UpdatedAt   time.Time  `json:"updated_at"`
	}
	if err := s.client.Do(ctx, "GET", path, nil, nil, &raw); err != nil {
		return nil, err
	}

	tasks := make([]Task, 0, len(raw))
	for _, item := range raw {
		t := Task{
			ID:          item.ID,
			Key:         taskKey(item.ID),
			Title:       item.Title,
			Description: item.Description,
			WorkspaceID: filter.WorkspaceID,
			Type:        TaskTypeTask,
			Status:      TaskStatusTodo,
			Priority:    PriorityMedium,
			DueDate:     item.DueDate,
			CreatedAt:   item.CreatedAt,
			UpdatedAt:   item.UpdatedAt,
		}
		t.Status = parseBackendTaskStatus(item.Status)
		if p, err := ParsePriority(item.Priority); err == nil {
			t.Priority = p
		}
		if item.AssigneeID != nil && *item.AssigneeID != "" {
			t.Assignee = &User{ID: *item.AssigneeID, Name: *item.AssigneeID}
		}
		if item.SprintID != nil {
			t.SprintID = *item.SprintID
		}
		if filter.Matches(t) {
			tasks = append(tasks, t)
		}
	}
	return tasks, nil
}

func (s *httpTaskService) Create(ctx context.Context, req CreateTaskRequest) (*Task, error) {
	path := fmt.Sprintf("/api/v1/workspaces/%s/tasks", req.WorkspaceID)
	body := map[string]interface{}{
		"title":       req.Title,
		"description": req.Description,
		"item_type":   strings.ToLower(string(req.Type)),
		"status":      "to_do",
		"priority":    strings.ToLower(string(req.Priority)),
	}
	if req.AssigneeID != "" {
		body["assignee_id"] = req.AssigneeID
	}
	if req.SprintID != "" {
		body["sprint_id"] = req.SprintID
	}
	if req.DueDate != nil {
		body["due_date"] = req.DueDate.Format("2006-01-02")
	}
	if req.StoryPoints != nil {
		body["story_points"] = *req.StoryPoints
	}

	var created struct {
		ID          string    `json:"id"`
		Title       string    `json:"title"`
		Description string    `json:"description"`
		Status      string    `json:"status"`
		Priority    string    `json:"priority"`
		CreatedAt   time.Time `json:"created_at"`
	}
	if err := s.client.Do(ctx, "POST", path, nil, body, &created); err != nil {
		return nil, err
	}
	return s.Get(ctx, req.WorkspaceID, created.ID)
}

func (s *httpTaskService) Get(ctx context.Context, workspaceID, taskID string) (*Task, error) {
	var raw struct {
		ID          string     `json:"id"`
		WorkspaceID string     `json:"workspace_id"`
		Title       string     `json:"title"`
		Description *string    `json:"description"`
		ItemType    string     `json:"item_type"`
		Status      string     `json:"status"`
		Priority    string     `json:"priority"`
		AssigneeID  *string    `json:"assignee_id"`
		SprintID    *string    `json:"sprint_id"`
		DueDate     *time.Time `json:"due_date"`
		StoryPoints *int32     `json:"story_points"`
		CreatedAt   time.Time  `json:"created_at"`
		UpdatedAt   time.Time  `json:"updated_at"`
	}
	path := fmt.Sprintf("/api/v1/workspaces/%s/tasks/%s", workspaceID, taskID)
	if err := s.client.Do(ctx, "GET", path, nil, nil, &raw); err != nil {
		return nil, err
	}
	task := &Task{
		ID:          raw.ID,
		Key:         taskKey(raw.ID),
		WorkspaceID: workspaceID,
		Title:       raw.Title,
		Type:        TaskType(strings.ToUpper(raw.ItemType)),
		Status:      parseBackendTaskStatus(raw.Status),
		Priority:    Priority(strings.ToUpper(raw.Priority)),
		DueDate:     raw.DueDate,
		CreatedAt:   raw.CreatedAt,
		UpdatedAt:   raw.UpdatedAt,
	}
	if raw.Description != nil {
		task.Description = *raw.Description
	}
	if raw.AssigneeID != nil {
		task.Assignee = &User{ID: *raw.AssigneeID, Name: *raw.AssigneeID}
	}
	if raw.SprintID != nil {
		task.SprintID = *raw.SprintID
	}
	if raw.StoryPoints != nil {
		points := int(*raw.StoryPoints)
		task.StoryPoints = &points
	}
	return task, nil
}

func (s *httpTaskService) Update(ctx context.Context, workspaceID, taskID string, req UpdateTaskRequest) (*Task, error) {
	current, err := s.Get(ctx, workspaceID, taskID)
	if err != nil {
		return nil, err
	}
	body := map[string]interface{}{
		"title":       current.Title,
		"description": current.Description,
		"item_type":   strings.ToLower(string(current.Type)),
		"status":      backendTaskStatus(current.Status),
		"priority":    strings.ToLower(string(current.Priority)),
	}
	if current.Assignee != nil {
		body["assignee_id"] = current.Assignee.ID
	}
	if current.SprintID != "" {
		body["sprint_id"] = current.SprintID
	}
	if current.DueDate != nil {
		body["due_date"] = current.DueDate.Format("2006-01-02")
	}
	if current.StoryPoints != nil {
		body["story_points"] = *current.StoryPoints
	}
	if req.Title != nil {
		body["title"] = *req.Title
	}
	if req.Description != nil {
		body["description"] = *req.Description
	}
	if req.Status != nil {
		body["status"] = backendTaskStatus(*req.Status)
	}
	if req.Priority != nil {
		body["priority"] = strings.ToLower(string(*req.Priority))
	}
	if req.AssigneeID != nil {
		body["assignee_id"] = *req.AssigneeID
	}
	if req.SprintID != nil {
		body["sprint_id"] = *req.SprintID
	}
	if req.DueDate != nil {
		body["due_date"] = req.DueDate.Format("2006-01-02")
	}
	if req.StoryPoints != nil {
		body["story_points"] = *req.StoryPoints
	}
	path := fmt.Sprintf("/api/v1/workspaces/%s/tasks/%s", workspaceID, taskID)
	if err := s.client.Do(ctx, "PUT", path, nil, body, nil); err != nil {
		return nil, err
	}
	return s.Get(ctx, workspaceID, taskID)
}

func (s *httpTaskService) Delete(ctx context.Context, workspaceID, taskID string) error {
	path := fmt.Sprintf("/api/v1/workspaces/%s/tasks/%s", workspaceID, taskID)
	return s.client.Do(ctx, "DELETE", path, nil, nil, nil)
}

func taskKey(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

func backendTaskStatus(status TaskStatus) string {
	switch status {
	case TaskStatusTodo:
		return "to_do"
	case TaskStatusInProgress:
		return "in_progress"
	case TaskStatusReview:
		return "in_review"
	case TaskStatusDone:
		return "done"
	default:
		return strings.ToLower(string(status))
	}
}

func parseBackendTaskStatus(status string) TaskStatus {
	if strings.EqualFold(status, "in_review") {
		return TaskStatusReview
	}
	if parsed, err := ParseTaskStatus(status); err == nil {
		return parsed
	}
	return TaskStatusTodo
}

// --- Epics ---

type httpEpicService struct {
	client *Client
}

type epicResponse struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	Priority    string    `json:"priority"`
	Color       string    `json:"color"`
	StartDate   *string   `json:"start_date"`
	EndDate     *string   `json:"end_date"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func decodeEpic(raw epicResponse, workspaceID string) *Epic {
	epic := &Epic{
		ID:          raw.ID,
		Key:         shortID(raw.ID),
		Title:       raw.Title,
		Description: raw.Description,
		Status:      raw.Status,
		Priority:    raw.Priority,
		Color:       raw.Color,
		WorkspaceID: workspaceID,
		CreatedAt:   raw.CreatedAt,
		UpdatedAt:   raw.UpdatedAt,
	}
	epic.StartDate = parseDateString(raw.StartDate)
	epic.DueDate = parseDateString(raw.EndDate)
	return epic
}

func (s *httpEpicService) Create(ctx context.Context, workspaceID string, req EpicMutationRequest) (*Epic, error) {
	var raw epicResponse
	path := fmt.Sprintf("/api/v1/workspaces/%s/epics", workspaceID)
	if err := s.client.Do(ctx, "POST", path, nil, epicRequest(req), &raw); err != nil {
		return nil, err
	}
	return decodeEpic(raw, workspaceID), nil
}

func (s *httpEpicService) List(ctx context.Context, workspaceID string) ([]Epic, error) {
	path := fmt.Sprintf("/api/v1/workspaces/%s/epics", workspaceID)
	var raw []epicResponse
	if err := s.client.Do(ctx, "GET", path, nil, nil, &raw); err != nil {
		return nil, err
	}
	epics := make([]Epic, 0, len(raw))
	for _, e := range raw {
		epics = append(epics, *decodeEpic(e, workspaceID))
	}
	return epics, nil
}

func (s *httpEpicService) Get(ctx context.Context, workspaceID, epicID string) (*Epic, error) {
	var raw epicResponse
	path := fmt.Sprintf("/api/v1/workspaces/%s/epics/%s", workspaceID, epicID)
	if err := s.client.Do(ctx, "GET", path, nil, nil, &raw); err != nil {
		return nil, err
	}
	return decodeEpic(raw, workspaceID), nil
}

func (s *httpEpicService) Update(ctx context.Context, workspaceID, epicID string, req EpicMutationRequest) (*Epic, error) {
	path := fmt.Sprintf("/api/v1/workspaces/%s/epics/%s", workspaceID, epicID)
	var raw epicResponse
	if err := s.client.Do(ctx, "PUT", path, nil, epicRequest(req), &raw); err != nil {
		return nil, err
	}
	return decodeEpic(raw, workspaceID), nil
}

func (s *httpEpicService) Delete(ctx context.Context, workspaceID, epicID string) error {
	path := fmt.Sprintf("/api/v1/workspaces/%s/epics/%s", workspaceID, epicID)
	return s.client.Do(ctx, "DELETE", path, nil, nil, nil)
}

func epicRequest(req EpicMutationRequest) map[string]string {
	return map[string]string{
		"title":       req.Title,
		"description": req.Description,
		"status":      req.Status,
		"priority":    req.Priority,
		"color":       req.Color,
		"start_date":  req.StartDate,
		"due_date":    req.EndDate,
	}
}

func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

func parseDateString(value *string) *time.Time {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil
	}
	parsed, err := time.Parse("2006-01-02", *value)
	if err != nil {
		return nil
	}
	return &parsed
}

// --- User Stories ---

type httpUserStoryService struct {
	client *Client
}

type userStoryResponse struct {
	ID                 string    `json:"id"`
	WorkspaceID        string    `json:"workspace_id"`
	EpicID             *string   `json:"epic_id"`
	Title              string    `json:"title"`
	Description        *string   `json:"description"`
	AcceptanceCriteria *string   `json:"acceptance_criteria"`
	Status             string    `json:"status"`
	Priority           string    `json:"priority"`
	AssigneeID         *string   `json:"assignee_id"`
	ReporterID         *string   `json:"reporter_id"`
	StoryPoints        *int32    `json:"story_points"`
	BacklogPosition    int32     `json:"backlog_position"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

func storyRequest(req CreateUserStoryRequest) map[string]interface{} {
	body := map[string]interface{}{
		"epic_id":             req.EpicID,
		"title":               req.Title,
		"description":         req.Description,
		"acceptance_criteria": req.AcceptanceCriteria,
		"status":              req.Status,
		"priority":            req.Priority,
		"assignee_id":         req.AssigneeID,
	}
	if req.StoryPoints != nil {
		body["story_points"] = *req.StoryPoints
	}
	return body
}

func decodeUserStories(raw []userStoryResponse, workspaceID string) []UserStory {
	stories := make([]UserStory, 0, len(raw))
	for _, story := range raw {
		item := UserStory{
			ID:              story.ID,
			WorkspaceID:     workspaceID,
			Title:           story.Title,
			Status:          story.Status,
			Priority:        story.Priority,
			BacklogPosition: int(story.BacklogPosition),
			CreatedAt:       story.CreatedAt,
			UpdatedAt:       story.UpdatedAt,
		}
		if story.EpicID != nil {
			item.EpicID = *story.EpicID
		}
		if story.Description != nil {
			item.Description = *story.Description
		}
		if story.AcceptanceCriteria != nil {
			item.AcceptanceCriteria = *story.AcceptanceCriteria
		}
		if story.AssigneeID != nil {
			item.AssigneeID = *story.AssigneeID
		}
		if story.ReporterID != nil {
			item.ReporterID = *story.ReporterID
		}
		if story.StoryPoints != nil {
			points := int(*story.StoryPoints)
			item.StoryPoints = &points
		}
		stories = append(stories, item)
	}
	return stories
}

func (s *httpUserStoryService) List(ctx context.Context, workspaceID string) ([]UserStory, error) {
	var raw []userStoryResponse
	path := fmt.Sprintf("/api/v1/workspaces/%s/stories", workspaceID)
	if err := s.client.Do(ctx, "GET", path, nil, nil, &raw); err != nil {
		return nil, err
	}
	return decodeUserStories(raw, workspaceID), nil
}

func (s *httpUserStoryService) Get(ctx context.Context, workspaceID, storyID string) (*UserStory, error) {
	var raw userStoryResponse
	path := fmt.Sprintf("/api/v1/workspaces/%s/stories/%s", workspaceID, storyID)
	if err := s.client.Do(ctx, "GET", path, nil, nil, &raw); err != nil {
		return nil, err
	}
	items := decodeUserStories([]userStoryResponse{raw}, workspaceID)
	return &items[0], nil
}

func (s *httpUserStoryService) Create(ctx context.Context, workspaceID string, req CreateUserStoryRequest) (*UserStory, error) {
	path := fmt.Sprintf("/api/v1/workspaces/%s/stories", workspaceID)
	var raw userStoryResponse
	if err := s.client.Do(ctx, "POST", path, nil, storyRequest(req), &raw); err != nil {
		return nil, err
	}
	items := decodeUserStories([]userStoryResponse{raw}, workspaceID)
	return &items[0], nil
}

func (s *httpUserStoryService) Update(ctx context.Context, workspaceID, storyID string, req CreateUserStoryRequest) (*UserStory, error) {
	path := fmt.Sprintf("/api/v1/workspaces/%s/stories/%s", workspaceID, storyID)
	var raw userStoryResponse
	if err := s.client.Do(ctx, "PUT", path, nil, storyRequest(req), &raw); err != nil {
		return nil, err
	}
	items := decodeUserStories([]userStoryResponse{raw}, workspaceID)
	return &items[0], nil
}

func (s *httpUserStoryService) Delete(ctx context.Context, workspaceID, storyID string) error {
	path := fmt.Sprintf("/api/v1/workspaces/%s/stories/%s", workspaceID, storyID)
	return s.client.Do(ctx, "DELETE", path, nil, nil, nil)
}

// --- Sprints ---

type httpSprintService struct {
	client *Client
}

type sprintResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Goal      string    `json:"goal"`
	Status    string    `json:"status"`
	StartDate *string   `json:"start_date"`
	EndDate   *string   `json:"end_date"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func decodeSprint(raw sprintResponse, workspaceID string) *Sprint {
	status := SprintStatusPlanned
	if strings.EqualFold(raw.Status, "active") {
		status = SprintStatusActive
	} else if strings.EqualFold(raw.Status, "completed") {
		status = SprintStatusCompleted
	}
	return &Sprint{
		ID:          raw.ID,
		Name:        raw.Name,
		Goal:        raw.Goal,
		Status:      status,
		WorkspaceID: workspaceID,
		StartDate:   parseDateString(raw.StartDate),
		EndDate:     parseDateString(raw.EndDate),
		CreatedAt:   raw.CreatedAt,
		UpdatedAt:   raw.UpdatedAt,
	}
}

func (s *httpSprintService) List(ctx context.Context, workspaceID string) ([]Sprint, error) {
	path := fmt.Sprintf("/api/v1/workspaces/%s/sprints", workspaceID)
	var raw []sprintResponse
	if err := s.client.Do(ctx, "GET", path, nil, nil, &raw); err != nil {
		return nil, err
	}
	sprints := make([]Sprint, 0, len(raw))
	for _, sp := range raw {
		sprints = append(sprints, *decodeSprint(sp, workspaceID))
	}
	return sprints, nil
}

func (s *httpSprintService) Get(ctx context.Context, workspaceID, sprintID string) (*Sprint, error) {
	var raw sprintResponse
	path := fmt.Sprintf("/api/v1/workspaces/%s/sprints/%s", workspaceID, sprintID)
	if err := s.client.Do(ctx, "GET", path, nil, nil, &raw); err != nil {
		return nil, err
	}
	return decodeSprint(raw, workspaceID), nil
}

func (s *httpSprintService) Create(ctx context.Context, workspaceID string, req SprintMutationRequest) (*Sprint, error) {
	path := fmt.Sprintf("/api/v1/workspaces/%s/sprints", workspaceID)
	var raw sprintResponse
	if err := s.client.Do(ctx, "POST", path, nil, sprintRequest(req), &raw); err != nil {
		return nil, err
	}
	return decodeSprint(raw, workspaceID), nil
}

func (s *httpSprintService) Update(ctx context.Context, workspaceID, sprintID string, req SprintMutationRequest) (*Sprint, error) {
	path := fmt.Sprintf("/api/v1/workspaces/%s/sprints/%s", workspaceID, sprintID)
	var raw sprintResponse
	if err := s.client.Do(ctx, "PUT", path, nil, sprintRequest(req), &raw); err != nil {
		return nil, err
	}
	return decodeSprint(raw, workspaceID), nil
}

func (s *httpSprintService) Start(ctx context.Context, workspaceID, sprintID string) (*Sprint, error) {
	return s.transition(ctx, workspaceID, sprintID, "start")
}

func (s *httpSprintService) Complete(ctx context.Context, workspaceID, sprintID string) (*Sprint, error) {
	return s.transition(ctx, workspaceID, sprintID, "complete")
}

func (s *httpSprintService) transition(ctx context.Context, workspaceID, sprintID, action string) (*Sprint, error) {
	path := fmt.Sprintf("/api/v1/workspaces/%s/sprints/%s/%s", workspaceID, sprintID, action)
	var raw sprintResponse
	if err := s.client.Do(ctx, "POST", path, nil, map[string]string{}, &raw); err != nil {
		return nil, err
	}
	return decodeSprint(raw, workspaceID), nil
}

func sprintRequest(req SprintMutationRequest) map[string]string {
	return map[string]string{
		"name":       req.Name,
		"goal":       req.Goal,
		"start_date": req.StartDate,
		"end_date":   req.EndDate,
	}
}

func (s *httpSprintService) Delete(ctx context.Context, workspaceID, sprintID string) error {
	path := fmt.Sprintf("/api/v1/workspaces/%s/sprints/%s", workspaceID, sprintID)
	return s.client.Do(ctx, "DELETE", path, nil, nil, nil)
}

func (s *httpSprintService) ListBacklog(ctx context.Context, workspaceID string) ([]UserStory, error) {
	var raw []userStoryResponse
	path := fmt.Sprintf("/api/v1/workspaces/%s/backlog", workspaceID)
	if err := s.client.Do(ctx, "GET", path, nil, nil, &raw); err != nil {
		return nil, err
	}
	return decodeUserStories(raw, workspaceID), nil
}

func (s *httpSprintService) ListStories(ctx context.Context, workspaceID, sprintID string) ([]UserStory, error) {
	var raw []userStoryResponse
	path := fmt.Sprintf("/api/v1/workspaces/%s/sprints/%s/stories", workspaceID, sprintID)
	if err := s.client.Do(ctx, "GET", path, nil, nil, &raw); err != nil {
		return nil, err
	}
	return decodeUserStories(raw, workspaceID), nil
}

func (s *httpSprintService) AddStory(ctx context.Context, workspaceID, sprintID, storyID string) error {
	path := fmt.Sprintf("/api/v1/workspaces/%s/sprints/%s/stories", workspaceID, sprintID)
	return s.client.Do(ctx, "POST", path, nil, map[string]string{"story_id": storyID}, nil)
}

func (s *httpSprintService) RemoveStory(ctx context.Context, workspaceID, sprintID, storyID string) error {
	path := fmt.Sprintf("/api/v1/workspaces/%s/sprints/%s/stories/%s", workspaceID, sprintID, storyID)
	return s.client.Do(ctx, "DELETE", path, nil, nil, nil)
}

func (s *httpSprintService) ReorderBacklogStory(ctx context.Context, workspaceID, storyID string, position int) error {
	path := fmt.Sprintf("/api/v1/workspaces/%s/backlog/%s/position", workspaceID, storyID)
	return s.client.Do(ctx, "PATCH", path, nil, map[string]int{"position": position}, nil)
}

func (s *httpSprintService) Board(ctx context.Context, workspaceID, sprintID string) ([]UserStory, SprintReport, error) {
	var raw struct {
		Report struct {
			TotalStories         int64   `json:"total_stories"`
			CompletedStories     int64   `json:"completed_stories"`
			CommittedStoryPoints int64   `json:"committed_story_points"`
			Velocity             int64   `json:"velocity"`
			CompletionRate       float64 `json:"completion_rate"`
		} `json:"report"`
		Stories []userStoryResponse `json:"stories"`
	}
	path := fmt.Sprintf("/api/v1/workspaces/%s/sprints/%s/board", workspaceID, sprintID)
	if err := s.client.Do(ctx, "GET", path, nil, nil, &raw); err != nil {
		return nil, SprintReport{}, err
	}
	report := SprintReport{
		TotalStories:         raw.Report.TotalStories,
		CompletedStories:     raw.Report.CompletedStories,
		CommittedStoryPoints: raw.Report.CommittedStoryPoints,
		Velocity:             raw.Report.Velocity,
		CompletionRate:       raw.Report.CompletionRate,
	}
	return decodeUserStories(raw.Stories, workspaceID), report, nil
}

// --- Notifications ---

type httpNotificationService struct {
	client *Client
}

func (s *httpNotificationService) List(ctx context.Context) ([]Notification, error) {
	var raw []struct {
		ID        string     `json:"id"`
		EventType string     `json:"event_type"`
		Message   string     `json:"message"`
		ReadAt    *time.Time `json:"read_at"`
		CreatedAt time.Time  `json:"created_at"`
	}
	if err := s.client.Do(ctx, "GET", "/api/v1/notifications", nil, nil, &raw); err != nil {
		return nil, err
	}
	notifs := make([]Notification, 0, len(raw))
	for _, n := range raw {
		notifs = append(notifs, Notification{
			ID:        n.ID,
			Title:     n.EventType,
			Body:      n.Message,
			Type:      n.EventType,
			Read:      n.ReadAt != nil,
			CreatedAt: n.CreatedAt,
		})
	}
	return notifs, nil
}

func (s *httpNotificationService) MarkRead(ctx context.Context, notificationID string) error {
	path := fmt.Sprintf("/api/v1/notifications/%s/read", notificationID)
	return s.client.Do(ctx, "POST", path, nil, nil, nil)
}

func (s *httpNotificationService) MarkAllRead(ctx context.Context) error {
	return s.client.Do(ctx, "POST", "/api/v1/notifications/read-all", nil, nil, nil)
}

// --- Activity ---

type httpActivityService struct {
	client *Client
}

func (s *httpActivityService) List(ctx context.Context, workspaceID string, limit int) ([]Activity, error) {
	path := fmt.Sprintf("/api/v1/workspaces/%s/activity", workspaceID)
	q := url.Values{}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	var raw []struct {
		ID          string    `json:"id"`
		ActorUserID string    `json:"actor_user_id"`
		ActionType  string    `json:"action_type"`
		EntityType  string    `json:"entity_type"`
		EntityID    string    `json:"entity_id"`
		CreatedAt   time.Time `json:"created_at"`
	}
	if err := s.client.Do(ctx, "GET", path, q, nil, &raw); err != nil {
		return nil, err
	}
	items := make([]Activity, 0, len(raw))
	for _, a := range raw {
		items = append(items, Activity{
			ID:        a.ID,
			ActorID:   a.ActorUserID,
			ActorName: a.ActorUserID[:8],
			Action:    a.ActionType,
			Subject:   a.EntityType,
			SubjectID: a.EntityID,
			CreatedAt: a.CreatedAt,
		})
	}
	return items, nil
}

// --- Chat ---

type httpChatService struct {
	client *Client
}

func (s *httpChatService) Channels(ctx context.Context, workspaceID string) ([]ChatChannel, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return nil, errors.New("workspace ID is required to list chat channels")
	}
	var raw []struct {
		ID        string    `json:"id"`
		Name      string    `json:"name"`
		Kind      string    `json:"kind"`
		CreatedAt time.Time `json:"created_at"`
	}
	path := fmt.Sprintf("/api/v1/workspaces/%s/chat/channels", workspaceID)
	if err := s.client.Do(ctx, "GET", path, nil, nil, &raw); err != nil {
		return nil, err
	}
	channels := make([]ChatChannel, 0, len(raw))
	for _, channel := range raw {
		channels = append(channels, ChatChannel{
			ID:          channel.ID,
			Name:        channel.Name,
			Description: channel.Kind,
		})
	}
	return channels, nil
}

func (s *httpChatService) Messages(ctx context.Context, workspaceID, channelID string, limit int) ([]ChatMessage, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return nil, errors.New("workspace ID is required to list chat messages")
	}
	path := fmt.Sprintf("/api/v1/workspaces/%s/chat/channels/%s/messages", workspaceID, channelID)
	q := url.Values{}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	var raw []struct {
		ID        string    `json:"id"`
		ChannelID string    `json:"channel_id"`
		SenderID  string    `json:"sender_id"`
		Content   string    `json:"content"`
		CreatedAt time.Time `json:"created_at"`
	}
	if err := s.client.Do(ctx, "GET", path, q, nil, &raw); err != nil {
		return nil, err
	}
	messages := make([]ChatMessage, 0, len(raw))
	for _, message := range raw {
		messages = append(messages, ChatMessage{
			ID:        message.ID,
			ChannelID: message.ChannelID,
			SenderID:  message.SenderID,
			Body:      message.Content,
			CreatedAt: message.CreatedAt,
		})
	}
	return messages, nil
}

func (s *httpChatService) Send(ctx context.Context, workspaceID, channelID, body string) (*ChatMessage, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return nil, errors.New("workspace ID is required to send chat messages")
	}
	path := fmt.Sprintf("/api/v1/workspaces/%s/chat/channels/%s/messages", workspaceID, channelID)
	var created struct {
		ID        string    `json:"id"`
		ChannelID string    `json:"channel_id"`
		SenderID  string    `json:"sender_id"`
		Content   string    `json:"content"`
		CreatedAt time.Time `json:"created_at"`
	}
	if err := s.client.Do(ctx, "POST", path, nil, map[string]string{"content": body}, &created); err != nil {
		return nil, err
	}
	return &ChatMessage{
		ID:        created.ID,
		ChannelID: created.ChannelID,
		SenderID:  created.SenderID,
		Body:      created.Content,
		CreatedAt: created.CreatedAt,
	}, nil
}

func newHTTPServices(client *Client) Services {
	status := ServiceStatus{
		Configured:    client != nil,
		ContractReady: true,
		Detail:        "Contract implemented",
	}
	if client == nil {
		status.Detail = ErrNotConfigured.Error()
	}
	return Services{
		Auth:          &httpAuthService{client: client},
		Workspaces:    &httpWorkspaceService{client: client},
		Tasks:         &httpTaskService{client: client},
		Epics:         &httpEpicService{client: client},
		UserStories:   &httpUserStoryService{client: client},
		Sprints:       &httpSprintService{client: client},
		Notifications: &httpNotificationService{client: client},
		Activity:      &httpActivityService{client: client},
		Chat:          &httpChatService{client: client},
		Client:        client,
		Status:        status,
	}
}
