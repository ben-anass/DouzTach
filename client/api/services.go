package api

import (
	"context"
	"fmt"
	"time"
)

// AuthResult is the client-side representation of a successful login or
// registration. The token is stored by the config layer and never printed.
type AuthResult struct {
	AccessToken  string
	RefreshToken string
	TokenType    string
	ExpiresAt    *time.Time
	User         User
}

// LoginRequest contains credentials. It is never logged.
type LoginRequest struct {
	Email    string
	Password string
}

// RegisterRequest contains the fields required to create an account.
type RegisterRequest struct {
	Name     string
	Email    string
	Password string
}

// CreateWorkspaceRequest is the client-side workspace creation payload.
type CreateWorkspaceRequest struct {
	Name        string
	Description string
}

// CreateUserStoryRequest is the JSON contract for user-story mutations.
type CreateUserStoryRequest struct {
	EpicID             string
	Title              string
	Description        string
	AcceptanceCriteria string
	Status             string
	Priority           string
	AssigneeID         string
	StoryPoints        *int
}

type EpicMutationRequest struct {
	Title       string
	Description string
	Status      string
	Priority    string
	Color       string
	StartDate   string
	EndDate     string
}

type SprintMutationRequest struct {
	Name      string
	Goal      string
	StartDate string
	EndDate   string
}

// CreateTaskRequest is the client-side task creation payload.
type CreateTaskRequest struct {
	WorkspaceID string
	Title       string
	Description string
	Type        TaskType
	Priority    Priority
	AssigneeID  string
	SprintID    string
	UserStoryID string
	DueDate     *time.Time
	StoryPoints *int
}

// UpdateTaskRequest uses pointers so callers can distinguish "leave unchanged"
// from an explicit zero value.
type UpdateTaskRequest struct {
	Title       *string
	Description *string
	Status      *TaskStatus
	Priority    *Priority
	AssigneeID  *string
	SprintID    *string
	UserStoryID *string
	DueDate     *time.Time
	StoryPoints *int
}

// AuthService covers account and session operations.
type AuthService interface {
	Login(ctx context.Context, req LoginRequest) (*AuthResult, error)
	Register(ctx context.Context, req RegisterRequest) (*AuthResult, error)
	Logout(ctx context.Context) error
	Profile(ctx context.Context) (*User, error)
}

// WorkspaceService covers workspace discovery and creation.
type WorkspaceService interface {
	List(ctx context.Context) ([]Workspace, error)
	Create(ctx context.Context, req CreateWorkspaceRequest) (*Workspace, error)
	Members(ctx context.Context, workspaceID string) ([]Member, error)
	CreateInvitation(ctx context.Context, workspaceID, email, role string) (*WorkspaceInvitation, error)
	AcceptInvitation(ctx context.Context, token string) error
}

// TaskService covers task CRUD.
type TaskService interface {
	List(ctx context.Context, filter TaskFilter) ([]Task, error)
	Get(ctx context.Context, workspaceID, taskID string) (*Task, error)
	Create(ctx context.Context, req CreateTaskRequest) (*Task, error)
	Update(ctx context.Context, workspaceID, taskID string, req UpdateTaskRequest) (*Task, error)
	Delete(ctx context.Context, workspaceID, taskID string) error
}

// EpicService covers workspace-scoped epic operations.
type EpicService interface {
	List(ctx context.Context, workspaceID string) ([]Epic, error)
	Get(ctx context.Context, workspaceID, epicID string) (*Epic, error)
	Create(ctx context.Context, workspaceID string, req EpicMutationRequest) (*Epic, error)
	Update(ctx context.Context, workspaceID, epicID string, req EpicMutationRequest) (*Epic, error)
	Delete(ctx context.Context, workspaceID, epicID string) error
}

// UserStoryService covers workspace-scoped user-story CRUD.
type UserStoryService interface {
	List(ctx context.Context, workspaceID string) ([]UserStory, error)
	Get(ctx context.Context, workspaceID, storyID string) (*UserStory, error)
	Create(ctx context.Context, workspaceID string, req CreateUserStoryRequest) (*UserStory, error)
	Update(ctx context.Context, workspaceID, storyID string, req CreateUserStoryRequest) (*UserStory, error)
	Delete(ctx context.Context, workspaceID, storyID string) error
}

// SprintService covers sprint discovery.
type SprintService interface {
	List(ctx context.Context, workspaceID string) ([]Sprint, error)
	Get(ctx context.Context, workspaceID, sprintID string) (*Sprint, error)
	Delete(ctx context.Context, workspaceID, sprintID string) error
	ListBacklog(ctx context.Context, workspaceID string) ([]UserStory, error)
	ListStories(ctx context.Context, workspaceID, sprintID string) ([]UserStory, error)
	AddStory(ctx context.Context, workspaceID, sprintID, storyID string) error
	RemoveStory(ctx context.Context, workspaceID, sprintID, storyID string) error
	ReorderBacklogStory(ctx context.Context, workspaceID, storyID string, position int) error
	Board(ctx context.Context, workspaceID, sprintID string) ([]UserStory, SprintReport, error)
	Create(ctx context.Context, workspaceID string, req SprintMutationRequest) (*Sprint, error)
	Update(ctx context.Context, workspaceID, sprintID string, req SprintMutationRequest) (*Sprint, error)
	Start(ctx context.Context, workspaceID, sprintID string) (*Sprint, error)
	Complete(ctx context.Context, workspaceID, sprintID string) (*Sprint, error)
	Reopen(ctx context.Context, workspaceID, sprintID string) (*Sprint, error)
}

// NotificationService covers the notification inbox.
type NotificationService interface {
	List(ctx context.Context) ([]Notification, error)
	MarkRead(ctx context.Context, notificationID string) error
	MarkAllRead(ctx context.Context) error
}

// ActivityService covers the workspace activity feed.
type ActivityService interface {
	List(ctx context.Context, workspaceID string, limit int) ([]Activity, error)
}

// ChatService covers the authenticated HTTP channel and message operations.
// Real-time delivery is handled by the backend WebSocket endpoint.
type ChatService interface {
	Channels(ctx context.Context, workspaceID string) ([]ChatChannel, error)
	Messages(ctx context.Context, workspaceID, channelID string, limit int) ([]ChatMessage, error)
	Send(ctx context.Context, workspaceID, channelID, body string) (*ChatMessage, error)
}

// ServiceStatus describes what the client knows about the backend connection.
// ContractReady is false until a real service implementation is registered.
type ServiceStatus struct {
	Configured    bool
	Reachable     bool
	ContractReady bool
	StatusCode    int
	Latency       time.Duration
	CheckedAt     time.Time
	Detail        string
}

// Summary returns a short human-readable state for headers and doctor output.
func (s ServiceStatus) Summary() string {
	switch {
	case !s.Configured:
		return "Not configured"
	case s.ContractReady && s.Reachable:
		return "Connected"
	case s.ContractReady:
		return "Contract ready, API unreachable"
	case s.Reachable:
		return "API reachable, contract pending"
	default:
		return "API unreachable"
	}
}

// Services bundles the client-side service interfaces. Commands and the TUI
// depend on these interfaces rather than on concrete HTTP implementations.
type Services struct {
	Auth          AuthService
	Workspaces    WorkspaceService
	Tasks         TaskService
	Epics         EpicService
	UserStories   UserStoryService
	Sprints       SprintService
	Notifications NotificationService
	Activity      ActivityService
	Chat          ChatService

	Client *Client
	Status ServiceStatus
}

// Ready reports whether a real backend contract has been implemented.
func (s Services) Ready() bool { return s.Status.ContractReady }

// RequireReady returns ErrContractPending when the backend contract is not yet
// available. Commands use this to fail fast before collecting secrets.
func (s Services) RequireReady() error {
	if s.Ready() {
		return nil
	}
	if s.Client == nil {
		return fmt.Errorf("%w; configure an API URL with 'douztech settings set api-url <url>'", ErrContractPending)
	}
	return ErrContractPending
}

// NewServices builds real HTTP adapters when an API URL is configured.
// Without a URL, pending services fail explicitly instead of fabricating data.
func NewServices(baseURL string, timeout time.Duration, tokenProvider func() string) (Services, error) {
	if baseURL == "" {
		return newPendingServices(nil), nil
	}
	client, err := NewClient(baseURL, WithTimeout(timeout), WithTokenProvider(tokenProvider))
	if err != nil {
		return Services{}, err
	}
	services := newHTTPServices(client)
	services.Status.Detail = "Contract ready"
	return services, nil
}

func newPendingServices(client *Client) Services {
	status := ServiceStatus{
		Configured:    client != nil,
		ContractReady: false,
		Detail:        ErrContractPending.Error(),
	}
	if client == nil {
		status.Detail = ErrNotConfigured.Error()
	}
	return Services{
		Auth:          pendingAuth{},
		Workspaces:    pendingWorkspaces{},
		Tasks:         pendingTasks{},
		Epics:         pendingEpics{},
		UserStories:   pendingUserStories{},
		Sprints:       pendingSprints{},
		Notifications: pendingNotifications{},
		Activity:      pendingActivity{},
		Chat:          pendingChat{},
		Client:        client,
		Status:        status,
	}
}

// RefreshStatus pings the configured API URL and updates the service status.
// It never marks the contract as ready.
func (s *Services) RefreshStatus(ctx context.Context) ServiceStatus {
	if s == nil {
		return ServiceStatus{Detail: "services are not initialized"}
	}
	if s.Client == nil {
		s.Status = ServiceStatus{
			Configured:    false,
			Reachable:     false,
			ContractReady: false,
			CheckedAt:     time.Now(),
			Detail:        ErrNotConfigured.Error(),
		}
		return s.Status
	}
	ping, err := s.Client.Ping(ctx)
	status := ServiceStatus{
		Configured:    true,
		ContractReady: s.Status.ContractReady,
		CheckedAt:     time.Now(),
	}
	if err != nil {
		status.Detail = err.Error()
	} else {
		status.Reachable = true
		status.StatusCode = ping.StatusCode
		status.Latency = ping.Duration
		if status.ContractReady {
			status.Detail = "API reachable"
		} else {
			status.Detail = "API reachable; endpoint contract pending"
		}
	}
	s.Status = status
	return status
}

type pendingAuth struct{}

func (pendingAuth) Login(context.Context, LoginRequest) (*AuthResult, error) {
	return nil, pendingError("login")
}

func (pendingAuth) Register(context.Context, RegisterRequest) (*AuthResult, error) {
	return nil, pendingError("register")
}

func (pendingAuth) Logout(context.Context) error {
	return pendingError("logout")
}

func (pendingAuth) Profile(context.Context) (*User, error) {
	return nil, pendingError("profile")
}

type pendingWorkspaces struct{}

func (pendingWorkspaces) List(context.Context) ([]Workspace, error) {
	return nil, pendingError("list workspaces")
}

func (pendingWorkspaces) Create(context.Context, CreateWorkspaceRequest) (*Workspace, error) {
	return nil, pendingError("create workspace")
}

func (pendingWorkspaces) Members(context.Context, string) ([]Member, error) {
	return nil, pendingError("list workspace members")
}

func (pendingWorkspaces) CreateInvitation(context.Context, string, string, string) (*WorkspaceInvitation, error) {
	return nil, pendingError("create workspace invitation")
}

func (pendingWorkspaces) AcceptInvitation(context.Context, string) error {
	return pendingError("accept workspace invitation")
}

type pendingTasks struct{}

func (pendingTasks) List(context.Context, TaskFilter) ([]Task, error) {
	return nil, pendingError("list tasks")
}

func (pendingTasks) Get(context.Context, string, string) (*Task, error) {
	return nil, pendingError("get task")
}

func (pendingTasks) Create(context.Context, CreateTaskRequest) (*Task, error) {
	return nil, pendingError("create task")
}

func (pendingTasks) Update(context.Context, string, string, UpdateTaskRequest) (*Task, error) {
	return nil, pendingError("update task")
}

func (pendingTasks) Delete(context.Context, string, string) error {
	return pendingError("delete task")
}

type pendingEpics struct{}

func (pendingEpics) List(context.Context, string) ([]Epic, error) {
	return nil, pendingError("list epics")
}

func (pendingEpics) Get(context.Context, string, string) (*Epic, error) {
	return nil, pendingError("get epic")
}

func (pendingEpics) Create(context.Context, string, EpicMutationRequest) (*Epic, error) {
	return nil, pendingError("create epic")
}

func (pendingEpics) Update(context.Context, string, string, EpicMutationRequest) (*Epic, error) {
	return nil, pendingError("update epic")
}

func (pendingEpics) Delete(context.Context, string, string) error {
	return pendingError("delete epic")
}

type pendingUserStories struct{}

func (pendingUserStories) List(context.Context, string) ([]UserStory, error) {
	return nil, pendingError("list user stories")
}

func (pendingUserStories) Get(context.Context, string, string) (*UserStory, error) {
	return nil, pendingError("get user story")
}

func (pendingUserStories) Create(context.Context, string, CreateUserStoryRequest) (*UserStory, error) {
	return nil, pendingError("create user story")
}

func (pendingUserStories) Update(context.Context, string, string, CreateUserStoryRequest) (*UserStory, error) {
	return nil, pendingError("update user story")
}

func (pendingUserStories) Delete(context.Context, string, string) error {
	return pendingError("delete user story")
}

type pendingSprints struct{}

func (pendingSprints) List(context.Context, string) ([]Sprint, error) {
	return nil, pendingError("list sprints")
}

func (pendingSprints) Get(context.Context, string, string) (*Sprint, error) {
	return nil, pendingError("get sprint")
}

func (pendingSprints) Delete(context.Context, string, string) error {
	return pendingError("delete sprint")
}

func (pendingSprints) ListBacklog(context.Context, string) ([]UserStory, error) {
	return nil, pendingError("list sprint backlog")
}

func (pendingSprints) ListStories(context.Context, string, string) ([]UserStory, error) {
	return nil, pendingError("list sprint stories")
}

func (pendingSprints) AddStory(context.Context, string, string, string) error {
	return pendingError("add story to sprint")
}

func (pendingSprints) RemoveStory(context.Context, string, string, string) error {
	return pendingError("remove story from sprint")
}

func (pendingSprints) ReorderBacklogStory(context.Context, string, string, int) error {
	return pendingError("reorder sprint backlog")
}

func (pendingSprints) Board(context.Context, string, string) ([]UserStory, SprintReport, error) {
	return nil, SprintReport{}, pendingError("get sprint board")
}

func (pendingSprints) Create(context.Context, string, SprintMutationRequest) (*Sprint, error) {
	return nil, pendingError("create sprint")
}

func (pendingSprints) Update(context.Context, string, string, SprintMutationRequest) (*Sprint, error) {
	return nil, pendingError("update sprint")
}

func (pendingSprints) Start(context.Context, string, string) (*Sprint, error) {
	return nil, pendingError("start sprint")
}

func (pendingSprints) Complete(context.Context, string, string) (*Sprint, error) {
	return nil, pendingError("complete sprint")
}

func (pendingSprints) Reopen(context.Context, string, string) (*Sprint, error) {
	return nil, pendingError("reopen sprint")
}

type pendingNotifications struct{}

func (pendingNotifications) List(context.Context) ([]Notification, error) {
	return nil, pendingError("list notifications")
}

func (pendingNotifications) MarkRead(context.Context, string) error {
	return pendingError("mark notification read")
}

func (pendingNotifications) MarkAllRead(context.Context) error {
	return pendingError("mark all notifications read")
}

type pendingActivity struct{}

func (pendingActivity) List(context.Context, string, int) ([]Activity, error) {
	return nil, pendingError("list activity")
}

type pendingChat struct{}

func (pendingChat) Channels(context.Context, string) ([]ChatChannel, error) {
	return nil, pendingError("list chat channels")
}

func (pendingChat) Messages(context.Context, string, string, int) ([]ChatMessage, error) {
	return nil, pendingError("list chat messages")
}

func (pendingChat) Send(context.Context, string, string, string) (*ChatMessage, error) {
	return nil, pendingError("send chat message")
}

func pendingError(operation string) error {
	return fmt.Errorf("%s: %w", operation, ErrContractPending)
}

// Compile-time checks that the pending implementations satisfy every service
// interface.
var (
	_ AuthService         = pendingAuth{}
	_ WorkspaceService    = pendingWorkspaces{}
	_ TaskService         = pendingTasks{}
	_ EpicService         = pendingEpics{}
	_ UserStoryService    = pendingUserStories{}
	_ SprintService       = pendingSprints{}
	_ NotificationService = pendingNotifications{}
	_ ActivityService     = pendingActivity{}
	_ ChatService         = pendingChat{}
)
