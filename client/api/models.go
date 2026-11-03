package api

import (
	"fmt"
	"strings"
	"time"
)

// TaskStatus is the workflow state of a task.
type TaskStatus string

const (
	TaskStatusTodo       TaskStatus = "TODO"
	TaskStatusInProgress TaskStatus = "IN_PROGRESS"
	TaskStatusReview     TaskStatus = "REVIEW"
	TaskStatusDone       TaskStatus = "DONE"
)

// TaskStatuses lists every supported status in board order.
var TaskStatuses = []TaskStatus{
	TaskStatusTodo,
	TaskStatusInProgress,
	TaskStatusReview,
	TaskStatusDone,
}

// Valid reports whether the status is one of the supported values.
func (s TaskStatus) Valid() bool {
	for _, candidate := range TaskStatuses {
		if s == candidate {
			return true
		}
	}
	return false
}

// Label returns the exact API/display label.
func (s TaskStatus) Label() string { return string(s) }

// Title returns a friendlier label for cards and headings.
func (s TaskStatus) Title() string {
	switch s {
	case TaskStatusTodo:
		return "To Do"
	case TaskStatusInProgress:
		return "In Progress"
	case TaskStatusReview:
		return "Review"
	case TaskStatusDone:
		return "Done"
	default:
		return string(s)
	}
}

// ParseTaskStatus accepts case-insensitive values with spaces or hyphens.
func ParseTaskStatus(value string) (TaskStatus, error) {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	normalized = strings.ReplaceAll(normalized, " ", "_")
	normalized = strings.ReplaceAll(normalized, "-", "_")
	for _, candidate := range TaskStatuses {
		if string(candidate) == normalized {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("invalid status %q (expected TODO, IN_PROGRESS, REVIEW, or DONE)", value)
}

// TaskType is the kind of work item.
type TaskType string

const (
	TaskTypeTask    TaskType = "TASK"
	TaskTypeBug     TaskType = "BUG"
	TaskTypeStory   TaskType = "STORY"
	TaskTypeEpic    TaskType = "EPIC"
	TaskTypeSubtask TaskType = "SUBTASK"
)

// TaskTypes lists every supported task type.
var TaskTypes = []TaskType{
	TaskTypeTask,
	TaskTypeBug,
	TaskTypeStory,
	TaskTypeEpic,
	TaskTypeSubtask,
}

// Valid reports whether the type is supported.
func (t TaskType) Valid() bool {
	for _, candidate := range TaskTypes {
		if t == candidate {
			return true
		}
	}
	return false
}

// Label returns the exact API/display label.
func (t TaskType) Label() string { return string(t) }

// ParseTaskType accepts case-insensitive values.
func ParseTaskType(value string) (TaskType, error) {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	for _, candidate := range TaskTypes {
		if string(candidate) == normalized {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("invalid type %q (expected TASK, BUG, STORY, EPIC, or SUBTASK)", value)
}

// Priority is the urgency of a task.
type Priority string

const (
	PriorityLow    Priority = "LOW"
	PriorityMedium Priority = "MEDIUM"
	PriorityHigh   Priority = "HIGH"
	PriorityUrgent Priority = "URGENT"
)

// Priorities lists every supported priority.
var Priorities = []Priority{
	PriorityLow,
	PriorityMedium,
	PriorityHigh,
	PriorityUrgent,
}

// Valid reports whether the priority is supported.
func (p Priority) Valid() bool {
	for _, candidate := range Priorities {
		if p == candidate {
			return true
		}
	}
	return false
}

// Label returns the exact API/display label.
func (p Priority) Label() string { return string(p) }

// ParsePriority accepts case-insensitive values.
func ParsePriority(value string) (Priority, error) {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	for _, candidate := range Priorities {
		if string(candidate) == normalized {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("invalid priority %q (expected LOW, MEDIUM, HIGH, or URGENT)", value)
}

// SprintStatus is the lifecycle state of a sprint.
type SprintStatus string

const (
	SprintStatusPlanned   SprintStatus = "PLANNED"
	SprintStatusActive    SprintStatus = "ACTIVE"
	SprintStatusCompleted SprintStatus = "COMPLETED"
)

// Valid reports whether the sprint status is supported.
func (s SprintStatus) Valid() bool {
	switch s {
	case SprintStatusPlanned, SprintStatusActive, SprintStatusCompleted:
		return true
	default:
		return false
	}
}

// User is a person known to the client.
type User struct {
	ID        string
	Name      string
	Username  string
	Email     string
	AvatarURL string
}

// DisplayName returns the best human-readable identifier.
func (u User) DisplayName() string {
	switch {
	case strings.TrimSpace(u.Name) != "":
		return u.Name
	case strings.TrimSpace(u.Username) != "":
		return "@" + strings.TrimPrefix(u.Username, "@")
	case strings.TrimSpace(u.Email) != "":
		return u.Email
	case strings.TrimSpace(u.ID) != "":
		return u.ID
	default:
		return "Unassigned"
	}
}

// Workspace is a personal or team workspace.
type Workspace struct {
	ID          string
	Name        string
	Description string
	Role        string
	MemberCount int
	CreatedAt   time.Time
}

// Member is a workspace membership.
type Member struct {
	User     User
	Role     string
	JoinedAt time.Time
}

// Task is a work item.
type Task struct {
	ID          string
	Key         string
	Title       string
	Description string
	WorkspaceID string
	SprintID    string
	EpicID      string
	Type        TaskType
	Status      TaskStatus
	Priority    Priority
	Assignee    *User
	Reporter    *User
	DueDate     *time.Time
	StoryPoints *int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// AssigneeName returns the assignee's display name or "Unassigned".
func (t Task) AssigneeName() string {
	if t.Assignee == nil {
		return "Unassigned"
	}
	return t.Assignee.DisplayName()
}

// IsOverdue reports whether a non-done task is past its due date.
func (t Task) IsOverdue(now time.Time) bool {
	if t.Status == TaskStatusDone || t.DueDate == nil {
		return false
	}
	return t.DueDate.Before(now)
}

// Epic groups related tasks.
type Epic struct {
	ID          string
	Key         string
	Title       string
	Description string
	Status      string
	WorkspaceID string
	Owner       *User
	DueDate     *time.Time
	TaskCount   int
	DoneCount   int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Progress returns a value between 0 and 1.
func (e Epic) Progress() float64 {
	if e.TaskCount <= 0 {
		return 0
	}
	progress := float64(e.DoneCount) / float64(e.TaskCount)
	if progress < 0 {
		return 0
	}
	if progress > 1 {
		return 1
	}
	return progress
}

// Sprint is a time-boxed iteration.
type Sprint struct {
	ID          string
	Name        string
	Goal        string
	Status      SprintStatus
	WorkspaceID string
	StartDate   *time.Time
	EndDate     *time.Time
	TaskCount   int
	DoneCount   int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Progress returns a value between 0 and 1.
func (s Sprint) Progress() float64 {
	if s.TaskCount <= 0 {
		return 0
	}
	progress := float64(s.DoneCount) / float64(s.TaskCount)
	if progress < 0 {
		return 0
	}
	if progress > 1 {
		return 1
	}
	return progress
}

// Notification is an in-app notification.
type Notification struct {
	ID        string
	Title     string
	Body      string
	Type      string
	Read      bool
	Link      string
	CreatedAt time.Time
}

// Activity is an entry in the workspace activity feed.
type Activity struct {
	ID        string
	ActorID   string
	ActorName string
	Action    string
	Subject   string
	SubjectID string
	CreatedAt time.Time
}

// ChatChannel is a conversation available to the user.
type ChatChannel struct {
	ID          string
	Name        string
	Description string
	MemberCount int
}

// ChatMessage is a single chat message.
type ChatMessage struct {
	ID         string
	ChannelID  string
	SenderID   string
	SenderName string
	Body       string
	CreatedAt  time.Time
}

// TaskCounts summarises a task collection for the dashboard.
type TaskCounts struct {
	Total      int
	Todo       int
	InProgress int
	Review     int
	Done       int
	Overdue    int
}

// CountTasks computes dashboard counters. Overdue excludes completed tasks.
func CountTasks(tasks []Task, now time.Time) TaskCounts {
	var counts TaskCounts
	for _, task := range tasks {
		counts.Total++
		switch task.Status {
		case TaskStatusTodo:
			counts.Todo++
		case TaskStatusInProgress:
			counts.InProgress++
		case TaskStatusReview:
			counts.Review++
		case TaskStatusDone:
			counts.Done++
		}
		if task.IsOverdue(now) {
			counts.Overdue++
		}
	}
	return counts
}

// TaskFilter describes the filters accepted by TaskService.List. A zero filter
// means "all tasks in the selected workspace".
type TaskFilter struct {
	WorkspaceID   string
	SprintID      string
	EpicID        string
	AssigneeID    string
	Status        *TaskStatus
	Type          *TaskType
	Priority      *Priority
	Search        string
	BacklogOnly   bool
	Mine          bool
	CurrentUserID string
}

// Matches reports whether a task satisfies the filter. It is used for
// client-side filtering when the backend returns a broader result set.
func (f TaskFilter) Matches(task Task) bool {
	if f.WorkspaceID != "" && task.WorkspaceID != f.WorkspaceID {
		return false
	}
	if f.SprintID != "" && task.SprintID != f.SprintID {
		return false
	}
	if f.EpicID != "" && task.EpicID != f.EpicID {
		return false
	}
	if f.BacklogOnly && task.SprintID != "" {
		return false
	}
	if f.Status != nil && task.Status != *f.Status {
		return false
	}
	if f.Type != nil && task.Type != *f.Type {
		return false
	}
	if f.Priority != nil && task.Priority != *f.Priority {
		return false
	}
	if f.Mine {
		if f.CurrentUserID == "" || task.Assignee == nil || task.Assignee.ID != f.CurrentUserID {
			return false
		}
	}
	if f.AssigneeID != "" {
		if task.Assignee == nil || task.Assignee.ID != f.AssigneeID {
			return false
		}
	}
	if search := strings.ToLower(strings.TrimSpace(f.Search)); search != "" {
		haystack := strings.ToLower(task.Key + " " + task.Title + " " + task.Description)
		if !strings.Contains(haystack, search) {
			return false
		}
	}
	return true
}
