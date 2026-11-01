package scrum

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/ben-anass/DouzTach/internal/repository"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalidInput     = errors.New("invalid Scrum input")
	ErrInvalidReference = errors.New("invalid Scrum reference")
	ErrNotFound         = errors.New("Scrum item not found")
	ErrActiveChildren   = errors.New("active child records prevent this operation")
)

const (
	defaultPriority = "medium"
	defaultStatus   = "to_do"
)

type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

type CreateEpicInput struct {
	WorkspaceID pgtype.UUID
	ActorUserID pgtype.UUID
	Title       string
	Description string
	Status      string
	Priority    string
	Color       string
	StartDate   pgtype.Date
	EndDate     pgtype.Date
}

type UpdateEpicInput struct {
	EpicID pgtype.UUID
	CreateEpicInput
}

type CreateUserStoryInput struct {
	WorkspaceID        pgtype.UUID
	ActorUserID        pgtype.UUID
	EpicID             pgtype.UUID
	Title              string
	Description        string
	AcceptanceCriteria string
	Status             string
	Priority           string
	AssigneeID         pgtype.UUID
	StoryPoints        pgtype.Int4
}

type UpdateUserStoryInput struct {
	UserStoryID pgtype.UUID
	CreateUserStoryInput
}

type CreateWorkItemInput struct {
	WorkspaceID pgtype.UUID
	ActorUserID pgtype.UUID
	UserStoryID pgtype.UUID
	ParentID    pgtype.UUID
	Title       string
	Description string
	ItemType    string
	Status      string
	Priority    string
	AssigneeID  pgtype.UUID
	SprintID    pgtype.UUID
	DueDate     pgtype.Date
	StoryPoints pgtype.Int4
}

type UpdateWorkItemInput struct {
	WorkItemID pgtype.UUID
	CreateWorkItemInput
}

type CreateLabelInput struct {
	WorkspaceID pgtype.UUID
	ActorUserID pgtype.UUID
	Name        string
	Color       string
}

type EpicProgress struct {
	TotalItems     int64
	CompletedItems int64
	CompletionRate float64
}

func (s *Service) CreateEpic(ctx context.Context, input CreateEpicInput) (repository.Epic, error) {
	input.Title = strings.TrimSpace(input.Title)
	input.Status = defaultIfEmpty(input.Status, "planned")
	input.Priority = defaultIfEmpty(input.Priority, defaultPriority)
	if err := validateEpic(input.WorkspaceID, input.ActorUserID, input.Title, input.Status, input.Priority, input.Color, input.StartDate, input.EndDate); err != nil {
		return repository.Epic{}, err
	}

	return inTransaction(ctx, s.pool, func(q *repository.Queries) (repository.Epic, error) {
		epic, err := q.CreateEpic(ctx, repository.CreateEpicParams{
			WorkspaceID: input.WorkspaceID,
			Title:       input.Title,
			Description: optionalText(input.Description),
			Status:      input.Status,
			Priority:    input.Priority,
			Color:       optionalText(input.Color),
			StartDate:   input.StartDate,
			EndDate:     input.EndDate,
		})
		if err != nil {
			return repository.Epic{}, fmt.Errorf("create epic: %w", err)
		}
		if err := recordActivity(ctx, q, input.ActorUserID, input.WorkspaceID, "created", "epic", epic.ID, nil, epic); err != nil {
			return repository.Epic{}, err
		}
		return epic, nil
	})
}

func (s *Service) ListEpics(ctx context.Context, workspaceID pgtype.UUID) ([]repository.Epic, error) {
	if !workspaceID.Valid {
		return nil, invalid("workspace ID is required")
	}
	items, err := repository.New(s.pool).ListEpicsForWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list epics: %w", err)
	}
	return items, nil
}

func (s *Service) GetEpic(ctx context.Context, workspaceID, epicID pgtype.UUID) (repository.Epic, error) {
	if !workspaceID.Valid || !epicID.Valid {
		return repository.Epic{}, invalid("workspace and epic IDs are required")
	}
	item, err := repository.New(s.pool).GetEpic(ctx, repository.GetEpicParams{ID: epicID, WorkspaceID: workspaceID})
	if err != nil {
		return repository.Epic{}, notFound(err)
	}
	return item, nil
}

func (s *Service) GetEpicProgress(ctx context.Context, workspaceID, epicID pgtype.UUID) (EpicProgress, error) {
	if !workspaceID.Valid || !epicID.Valid {
		return EpicProgress{}, invalid("workspace and epic IDs are required")
	}
	row, err := repository.New(s.pool).GetEpicProgress(ctx, repository.GetEpicProgressParams{
		EpicID:      epicID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return EpicProgress{}, notFound(err)
	}
	progress := EpicProgress{
		TotalItems:     row.TotalItems,
		CompletedItems: row.CompletedItems,
	}
	if progress.TotalItems > 0 {
		progress.CompletionRate = float64(progress.CompletedItems) / float64(progress.TotalItems) * 100
	}
	return progress, nil
}

func (s *Service) UpdateEpic(ctx context.Context, input UpdateEpicInput) (repository.Epic, error) {
	input.Title = strings.TrimSpace(input.Title)
	input.Status = defaultIfEmpty(input.Status, "planned")
	input.Priority = defaultIfEmpty(input.Priority, defaultPriority)
	if !input.EpicID.Valid {
		return repository.Epic{}, invalid("epic ID is required")
	}
	if err := validateEpic(input.WorkspaceID, input.ActorUserID, input.Title, input.Status, input.Priority, input.Color, input.StartDate, input.EndDate); err != nil {
		return repository.Epic{}, err
	}

	return inTransaction(ctx, s.pool, func(q *repository.Queries) (repository.Epic, error) {
		previous, err := q.GetEpic(ctx, repository.GetEpicParams{ID: input.EpicID, WorkspaceID: input.WorkspaceID})
		if err != nil {
			return repository.Epic{}, notFound(err)
		}
		updated, err := q.UpdateEpic(ctx, repository.UpdateEpicParams{
			ID:          input.EpicID,
			WorkspaceID: input.WorkspaceID,
			Title:       input.Title,
			Description: optionalText(input.Description),
			Status:      input.Status,
			Priority:    input.Priority,
			Color:       optionalText(input.Color),
			StartDate:   input.StartDate,
			EndDate:     input.EndDate,
		})
		if err != nil {
			return repository.Epic{}, notFound(err)
		}
		if err := recordActivity(ctx, q, input.ActorUserID, input.WorkspaceID, "updated", "epic", input.EpicID, previous, updated); err != nil {
			return repository.Epic{}, err
		}
		return updated, nil
	})
}

func (s *Service) DeleteEpic(ctx context.Context, workspaceID, actorUserID, epicID pgtype.UUID) error {
	if !workspaceID.Valid || !actorUserID.Valid || !epicID.Valid {
		return invalid("workspace, actor, and epic IDs are required")
	}
	_, err := inTransaction(ctx, s.pool, func(q *repository.Queries) (struct{}, error) {
		previous, err := q.GetEpic(ctx, repository.GetEpicParams{ID: epicID, WorkspaceID: workspaceID})
		if err != nil {
			return struct{}{}, notFound(err)
		}
		if _, err := q.SoftDeleteEpic(ctx, repository.SoftDeleteEpicParams{ID: epicID, WorkspaceID: workspaceID}); err != nil {
			return struct{}{}, notFound(err)
		}
		if err := recordActivity(ctx, q, actorUserID, workspaceID, "deleted", "epic", epicID, previous, map[string]bool{"deleted": true}); err != nil {
			return struct{}{}, err
		}
		return struct{}{}, nil
	})
	return err
}

func (s *Service) CreateUserStory(ctx context.Context, input CreateUserStoryInput) (repository.UserStory, error) {
	input.Title = strings.TrimSpace(input.Title)
	input.Status = defaultIfEmpty(input.Status, defaultStatus)
	input.Priority = defaultIfEmpty(input.Priority, defaultPriority)
	if err := validateStory(input.WorkspaceID, input.ActorUserID, input.Title, input.Status, input.Priority, input.StoryPoints); err != nil {
		return repository.UserStory{}, err
	}

	return inTransaction(ctx, s.pool, func(q *repository.Queries) (repository.UserStory, error) {
		if err := validateStoryReferences(ctx, q, input.WorkspaceID, input.EpicID); err != nil {
			return repository.UserStory{}, err
		}
		if err := validateAssignee(ctx, q, input.WorkspaceID, input.AssigneeID); err != nil {
			return repository.UserStory{}, err
		}
		story, err := q.CreateUserStory(ctx, repository.CreateUserStoryParams{
			WorkspaceID:        input.WorkspaceID,
			EpicID:             input.EpicID,
			Title:              input.Title,
			Description:        optionalText(input.Description),
			AcceptanceCriteria: optionalText(input.AcceptanceCriteria),
			Status:             input.Status,
			Priority:           input.Priority,
			AssigneeID:         input.AssigneeID,
			ReporterID:         input.ActorUserID,
			StoryPoints:        input.StoryPoints,
		})
		if err != nil {
			return repository.UserStory{}, fmt.Errorf("create user story: %w", err)
		}
		if err := recordActivity(ctx, q, input.ActorUserID, input.WorkspaceID, "created", "user_story", story.ID, nil, story); err != nil {
			return repository.UserStory{}, err
		}
		return story, nil
	})
}

func (s *Service) ListUserStories(ctx context.Context, workspaceID pgtype.UUID) ([]repository.UserStory, error) {
	if !workspaceID.Valid {
		return nil, invalid("workspace ID is required")
	}
	items, err := repository.New(s.pool).ListUserStoriesForWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list user stories: %w", err)
	}
	return items, nil
}

func (s *Service) GetUserStory(ctx context.Context, workspaceID, storyID pgtype.UUID) (repository.UserStory, error) {
	if !workspaceID.Valid || !storyID.Valid {
		return repository.UserStory{}, invalid("workspace and user story IDs are required")
	}
	item, err := repository.New(s.pool).GetUserStory(ctx, repository.GetUserStoryParams{ID: storyID, WorkspaceID: workspaceID})
	if err != nil {
		return repository.UserStory{}, notFound(err)
	}
	return item, nil
}

func (s *Service) UpdateUserStory(ctx context.Context, input UpdateUserStoryInput) (repository.UserStory, error) {
	input.Title = strings.TrimSpace(input.Title)
	input.Status = defaultIfEmpty(input.Status, defaultStatus)
	input.Priority = defaultIfEmpty(input.Priority, defaultPriority)
	if !input.UserStoryID.Valid {
		return repository.UserStory{}, invalid("user story ID is required")
	}
	if err := validateStory(input.WorkspaceID, input.ActorUserID, input.Title, input.Status, input.Priority, input.StoryPoints); err != nil {
		return repository.UserStory{}, err
	}

	return inTransaction(ctx, s.pool, func(q *repository.Queries) (repository.UserStory, error) {
		previous, err := q.GetUserStory(ctx, repository.GetUserStoryParams{ID: input.UserStoryID, WorkspaceID: input.WorkspaceID})
		if err != nil {
			return repository.UserStory{}, notFound(err)
		}
		if err := validateStoryReferences(ctx, q, input.WorkspaceID, input.EpicID); err != nil {
			return repository.UserStory{}, err
		}
		if err := validateAssignee(ctx, q, input.WorkspaceID, input.AssigneeID); err != nil {
			return repository.UserStory{}, err
		}
		updated, err := q.UpdateUserStory(ctx, repository.UpdateUserStoryParams{
			ID:                 input.UserStoryID,
			WorkspaceID:        input.WorkspaceID,
			EpicID:             input.EpicID,
			Title:              input.Title,
			Description:        optionalText(input.Description),
			AcceptanceCriteria: optionalText(input.AcceptanceCriteria),
			Status:             input.Status,
			Priority:           input.Priority,
			AssigneeID:         input.AssigneeID,
			StoryPoints:        input.StoryPoints,
		})
		if err != nil {
			return repository.UserStory{}, notFound(err)
		}
		if err := recordActivity(ctx, q, input.ActorUserID, input.WorkspaceID, "updated", "user_story", input.UserStoryID, previous, updated); err != nil {
			return repository.UserStory{}, err
		}
		return updated, nil
	})
}

func (s *Service) DeleteUserStory(ctx context.Context, workspaceID, actorUserID, storyID pgtype.UUID) error {
	if !workspaceID.Valid || !actorUserID.Valid || !storyID.Valid {
		return invalid("workspace, actor, and story IDs are required")
	}
	_, err := inTransaction(ctx, s.pool, func(q *repository.Queries) (struct{}, error) {
		previous, err := q.GetUserStory(ctx, repository.GetUserStoryParams{ID: storyID, WorkspaceID: workspaceID})
		if err != nil {
			return struct{}{}, notFound(err)
		}
		hasWorkItems, err := q.UserStoryHasActiveWorkItems(ctx, repository.UserStoryHasActiveWorkItemsParams{
			UserStoryID: storyID,
			WorkspaceID: workspaceID,
		})
		if err != nil {
			return struct{}{}, fmt.Errorf("check user story work items: %w", err)
		}
		if hasWorkItems {
			return struct{}{}, ErrActiveChildren
		}
		if _, err := q.SoftDeleteUserStory(ctx, repository.SoftDeleteUserStoryParams{ID: storyID, WorkspaceID: workspaceID}); err != nil {
			return struct{}{}, notFound(err)
		}
		if err := recordActivity(ctx, q, actorUserID, workspaceID, "deleted", "user_story", storyID, previous, map[string]bool{"deleted": true}); err != nil {
			return struct{}{}, err
		}
		return struct{}{}, nil
	})
	return err
}

func (s *Service) CreateWorkItem(ctx context.Context, input CreateWorkItemInput) (repository.WorkItem, error) {
	input.Title = strings.TrimSpace(input.Title)
	input.ItemType = strings.TrimSpace(input.ItemType)
	input.Status = defaultIfEmpty(input.Status, defaultStatus)
	input.Priority = defaultIfEmpty(input.Priority, defaultPriority)
	if err := validateWorkItem(input.WorkspaceID, input.ActorUserID, input.Title, input.ItemType, input.Status, input.Priority, input.StoryPoints, input.ParentID); err != nil {
		return repository.WorkItem{}, err
	}

	return inTransaction(ctx, s.pool, func(q *repository.Queries) (repository.WorkItem, error) {
		if err := validateWorkItemReferences(ctx, q, input.WorkspaceID, input.UserStoryID, input.SprintID, input.ParentID); err != nil {
			return repository.WorkItem{}, err
		}
		if err := validateAssignee(ctx, q, input.WorkspaceID, input.AssigneeID); err != nil {
			return repository.WorkItem{}, err
		}
		item, err := q.CreateWorkItem(ctx, repository.CreateWorkItemParams{
			WorkspaceID: input.WorkspaceID,
			UserStoryID: input.UserStoryID,
			ParentID:    input.ParentID,
			Title:       input.Title,
			Description: optionalText(input.Description),
			ItemType:    input.ItemType,
			Status:      input.Status,
			Priority:    input.Priority,
			AssigneeID:  input.AssigneeID,
			ReporterID:  input.ActorUserID,
			SprintID:    input.SprintID,
			DueDate:     input.DueDate,
			StoryPoints: input.StoryPoints,
		})
		if err != nil {
			return repository.WorkItem{}, fmt.Errorf("create work item: %w", err)
		}
		if err := recordActivity(ctx, q, input.ActorUserID, input.WorkspaceID, "created", "work_item", item.ID, nil, item); err != nil {
			return repository.WorkItem{}, err
		}
		return item, nil
	})
}

func (s *Service) ListWorkItems(ctx context.Context, workspaceID pgtype.UUID) ([]repository.WorkItem, error) {
	if !workspaceID.Valid {
		return nil, invalid("workspace ID is required")
	}
	items, err := repository.New(s.pool).ListWorkItemsForWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list work items: %w", err)
	}
	return items, nil
}

func (s *Service) GetWorkItem(ctx context.Context, workspaceID, workItemID pgtype.UUID) (repository.WorkItem, error) {
	if !workspaceID.Valid || !workItemID.Valid {
		return repository.WorkItem{}, invalid("workspace and work item IDs are required")
	}
	item, err := repository.New(s.pool).GetWorkItem(ctx, repository.GetWorkItemParams{ID: workItemID, WorkspaceID: workspaceID})
	if err != nil {
		return repository.WorkItem{}, notFound(err)
	}
	return item, nil
}

func (s *Service) UpdateWorkItem(ctx context.Context, input UpdateWorkItemInput) (repository.WorkItem, error) {
	input.Title = strings.TrimSpace(input.Title)
	input.ItemType = strings.TrimSpace(input.ItemType)
	input.Status = defaultIfEmpty(input.Status, defaultStatus)
	input.Priority = defaultIfEmpty(input.Priority, defaultPriority)
	if !input.WorkItemID.Valid {
		return repository.WorkItem{}, invalid("work item ID is required")
	}
	if err := validateWorkItem(input.WorkspaceID, input.ActorUserID, input.Title, input.ItemType, input.Status, input.Priority, input.StoryPoints, input.ParentID); err != nil {
		return repository.WorkItem{}, err
	}
	if input.ParentID.Valid && input.ParentID == input.WorkItemID {
		return repository.WorkItem{}, invalid("a work item cannot be its own parent")
	}

	return inTransaction(ctx, s.pool, func(q *repository.Queries) (repository.WorkItem, error) {
		previous, err := q.GetWorkItem(ctx, repository.GetWorkItemParams{ID: input.WorkItemID, WorkspaceID: input.WorkspaceID})
		if err != nil {
			return repository.WorkItem{}, notFound(err)
		}
		if err := validateWorkItemReferences(ctx, q, input.WorkspaceID, input.UserStoryID, input.SprintID, input.ParentID); err != nil {
			return repository.WorkItem{}, err
		}
		if err := validateAssignee(ctx, q, input.WorkspaceID, input.AssigneeID); err != nil {
			return repository.WorkItem{}, err
		}
		if input.ItemType == "subtask" && previous.ItemType != "subtask" {
			hasChildren, err := q.WorkItemHasActiveChildren(ctx, repository.WorkItemHasActiveChildrenParams{
				ParentID:    input.WorkItemID,
				WorkspaceID: input.WorkspaceID,
			})
			if err != nil {
				return repository.WorkItem{}, fmt.Errorf("check work item children: %w", err)
			}
			if hasChildren {
				return repository.WorkItem{}, ErrActiveChildren
			}
		}
		updated, err := q.UpdateWorkItem(ctx, repository.UpdateWorkItemParams{
			ID:          input.WorkItemID,
			WorkspaceID: input.WorkspaceID,
			UserStoryID: input.UserStoryID,
			ParentID:    input.ParentID,
			Title:       input.Title,
			Description: optionalText(input.Description),
			ItemType:    input.ItemType,
			Status:      input.Status,
			Priority:    input.Priority,
			AssigneeID:  input.AssigneeID,
			SprintID:    input.SprintID,
			DueDate:     input.DueDate,
			StoryPoints: input.StoryPoints,
		})
		if err != nil {
			return repository.WorkItem{}, notFound(err)
		}
		if err := recordActivity(ctx, q, input.ActorUserID, input.WorkspaceID, "updated", "work_item", input.WorkItemID, previous, updated); err != nil {
			return repository.WorkItem{}, err
		}
		return updated, nil
	})
}

func (s *Service) DeleteWorkItem(ctx context.Context, workspaceID, actorUserID, workItemID pgtype.UUID) error {
	if !workspaceID.Valid || !actorUserID.Valid || !workItemID.Valid {
		return invalid("workspace, actor, and work item IDs are required")
	}
	_, err := inTransaction(ctx, s.pool, func(q *repository.Queries) (struct{}, error) {
		previous, err := q.GetWorkItem(ctx, repository.GetWorkItemParams{ID: workItemID, WorkspaceID: workspaceID})
		if err != nil {
			return struct{}{}, notFound(err)
		}
		hasChildren, err := q.WorkItemHasActiveChildren(ctx, repository.WorkItemHasActiveChildrenParams{
			ParentID:    workItemID,
			WorkspaceID: workspaceID,
		})
		if err != nil {
			return struct{}{}, fmt.Errorf("check work item children: %w", err)
		}
		if hasChildren {
			return struct{}{}, ErrActiveChildren
		}
		if _, err := q.SoftDeleteWorkItem(ctx, repository.SoftDeleteWorkItemParams{ID: workItemID, WorkspaceID: workspaceID}); err != nil {
			return struct{}{}, notFound(err)
		}
		if err := recordActivity(ctx, q, actorUserID, workspaceID, "deleted", "work_item", workItemID, previous, map[string]bool{"deleted": true}); err != nil {
			return struct{}{}, err
		}
		return struct{}{}, nil
	})
	return err
}

func (s *Service) CreateLabel(ctx context.Context, input CreateLabelInput) (repository.Label, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Color = defaultIfEmpty(input.Color, "#7C3AED")
	if !input.WorkspaceID.Valid || !input.ActorUserID.Valid || input.Name == "" || len(input.Name) > 80 {
		return repository.Label{}, invalid("workspace, actor, and a label name of 1-80 characters are required")
	}
	if len(input.Color) > 30 {
		return repository.Label{}, invalid("label color cannot exceed 30 characters")
	}

	return inTransaction(ctx, s.pool, func(q *repository.Queries) (repository.Label, error) {
		label, err := q.CreateLabel(ctx, repository.CreateLabelParams{
			WorkspaceID: input.WorkspaceID,
			Name:        input.Name,
			Color:       input.Color,
		})
		if err != nil {
			return repository.Label{}, fmt.Errorf("create label: %w", err)
		}
		if err := recordActivity(ctx, q, input.ActorUserID, input.WorkspaceID, "created", "label", label.ID, nil, label); err != nil {
			return repository.Label{}, err
		}
		return label, nil
	})
}

func (s *Service) ListWorkItemLabels(ctx context.Context, workspaceID, workItemID pgtype.UUID) ([]repository.Label, error) {
	if !workspaceID.Valid || !workItemID.Valid {
		return nil, invalid("workspace and work item IDs are required")
	}
	items, err := repository.New(s.pool).ListLabelsForWorkItem(ctx, repository.ListLabelsForWorkItemParams{
		WorkspaceID: workspaceID,
		WorkItemID:  workItemID,
	})
	if err != nil {
		return nil, fmt.Errorf("list work item labels: %w", err)
	}
	return items, nil
}

func (s *Service) AddLabelToWorkItem(ctx context.Context, workspaceID, actorUserID, workItemID, labelID pgtype.UUID) error {
	if !workspaceID.Valid || !actorUserID.Valid || !workItemID.Valid || !labelID.Valid {
		return invalid("workspace, actor, work item, and label IDs are required")
	}
	_, err := inTransaction(ctx, s.pool, func(q *repository.Queries) (struct{}, error) {
		refs, err := q.CheckWorkItemLabelReferences(ctx, repository.CheckWorkItemLabelReferencesParams{
			WorkspaceID: workspaceID,
			WorkItemID:  workItemID,
			LabelID:     labelID,
		})
		if err != nil {
			return struct{}{}, fmt.Errorf("validate label references: %w", err)
		}
		if !refs.WorkItemValid || !refs.LabelValid {
			return struct{}{}, ErrInvalidReference
		}
		if _, err := q.AddWorkItemLabel(ctx, repository.AddWorkItemLabelParams{WorkItemID: workItemID, LabelID: labelID}); err != nil {
			return struct{}{}, fmt.Errorf("add label to work item: %w", err)
		}
		if err := recordActivity(ctx, q, actorUserID, workspaceID, "label_added", "work_item", workItemID, nil, map[string]pgtype.UUID{"label_id": labelID}); err != nil {
			return struct{}{}, err
		}
		return struct{}{}, nil
	})
	return err
}

func (s *Service) RemoveLabelFromWorkItem(ctx context.Context, workspaceID, actorUserID, workItemID, labelID pgtype.UUID) error {
	if !workspaceID.Valid || !actorUserID.Valid || !workItemID.Valid || !labelID.Valid {
		return invalid("workspace, actor, work item, and label IDs are required")
	}
	_, err := inTransaction(ctx, s.pool, func(q *repository.Queries) (struct{}, error) {
		link, err := q.RemoveWorkItemLabel(ctx, repository.RemoveWorkItemLabelParams{
			WorkspaceID: workspaceID,
			WorkItemID:  workItemID,
			LabelID:     labelID,
		})
		if err != nil {
			return struct{}{}, notFound(err)
		}
		if err := recordActivity(ctx, q, actorUserID, workspaceID, "label_removed", "work_item", workItemID, link, nil); err != nil {
			return struct{}{}, err
		}
		return struct{}{}, nil
	})
	return err
}

func validateEpic(workspaceID, actorID pgtype.UUID, title, status, priority, color string, start, end pgtype.Date) error {
	if !workspaceID.Valid || !actorID.Valid || title == "" || len(title) > 200 {
		return invalid("workspace, actor, and a title of 1-200 characters are required")
	}
	if !oneOf(status, "planned", "active", "completed", "archived") {
		return invalid("epic status is invalid")
	}
	if err := validatePriority(priority); err != nil {
		return err
	}
	if len(color) > 30 {
		return invalid("epic color cannot exceed 30 characters")
	}
	if start.Valid && end.Valid && start.Time.After(end.Time) {
		return invalid("epic start date must not be after end date")
	}
	return nil
}

func validateStory(workspaceID, actorID pgtype.UUID, title, status, priority string, points pgtype.Int4) error {
	if !workspaceID.Valid || !actorID.Valid || title == "" || len(title) > 200 {
		return invalid("workspace, actor, and a title of 1-200 characters are required")
	}
	if !oneOf(status, "to_do", "in_progress", "in_review", "done", "blocked") {
		return invalid("user story status is invalid")
	}
	if err := validatePriority(priority); err != nil {
		return err
	}
	if points.Valid && points.Int32 < 0 {
		return invalid("story points cannot be negative")
	}
	return nil
}

func validateWorkItem(workspaceID, actorID pgtype.UUID, title, itemType, status, priority string, points pgtype.Int4, parentID pgtype.UUID) error {
	if !workspaceID.Valid || !actorID.Valid || title == "" || len(title) > 200 {
		return invalid("workspace, actor, and a title of 1-200 characters are required")
	}
	if !oneOf(itemType, "task", "bug", "subtask") {
		return invalid("work item type is invalid")
	}
	if !oneOf(status, "to_do", "in_progress", "in_review", "done", "blocked") {
		return invalid("work item status is invalid")
	}
	if err := validatePriority(priority); err != nil {
		return err
	}
	if points.Valid && points.Int32 < 0 {
		return invalid("story points cannot be negative")
	}
	if (itemType == "subtask") != parentID.Valid {
		return invalid("only subtasks must have a task or bug parent")
	}
	return nil
}

func validateStoryReferences(ctx context.Context, q *repository.Queries, workspaceID, epicID pgtype.UUID) error {
	row, err := q.CheckUserStoryReferences(ctx, repository.CheckUserStoryReferencesParams{
		WorkspaceID: workspaceID,
		EpicID:      epicID,
	})
	if err != nil {
		return fmt.Errorf("validate user story references: %w", err)
	}
	if !row.Bool {
		return ErrInvalidReference
	}
	return nil
}

func validateWorkItemReferences(ctx context.Context, q *repository.Queries, workspaceID, storyID, sprintID, parentID pgtype.UUID) error {
	row, err := q.CheckWorkItemReferences(ctx, repository.CheckWorkItemReferencesParams{
		WorkspaceID: workspaceID,
		UserStoryID: storyID,
		SprintID:    sprintID,
		ParentID:    parentID,
	})
	if err != nil {
		return fmt.Errorf("validate work item references: %w", err)
	}
	if !row.StoryValid.Bool || !row.SprintValid.Bool || !row.ParentValid.Bool {
		return ErrInvalidReference
	}
	return nil
}

func validateAssignee(ctx context.Context, q *repository.Queries, workspaceID, assigneeID pgtype.UUID) error {
	if !assigneeID.Valid {
		return nil
	}
	isMember, err := q.IsWorkspaceMember(ctx, repository.IsWorkspaceMemberParams{
		WorkspaceID: workspaceID,
		UserID:      assigneeID,
	})
	if err != nil {
		return fmt.Errorf("validate Scrum assignee: %w", err)
	}
	if !isMember {
		return ErrInvalidReference
	}
	return nil
}

func recordActivity(ctx context.Context, q *repository.Queries, actorID, workspaceID pgtype.UUID, action, entity string, entityID pgtype.UUID, previous, current any) error {
	previousJSON, err := marshalOptional(previous)
	if err != nil {
		return fmt.Errorf("encode previous activity value: %w", err)
	}
	currentJSON, err := marshalOptional(current)
	if err != nil {
		return fmt.Errorf("encode new activity value: %w", err)
	}
	_, err = q.CreateActivityLog(ctx, repository.CreateActivityLogParams{
		ActorUserID:   actorID,
		WorkspaceID:   workspaceID,
		ActionType:    action,
		EntityType:    entity,
		EntityID:      entityID,
		PreviousValue: previousJSON,
		NewValue:      currentJSON,
	})
	if err != nil {
		return fmt.Errorf("record %s activity: %w", action, err)
	}
	return nil
}

func inTransaction[T any](ctx context.Context, pool *pgxpool.Pool, operation func(*repository.Queries) (T, error)) (T, error) {
	var zero T
	tx, err := pool.Begin(ctx)
	if err != nil {
		return zero, fmt.Errorf("begin Scrum transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	result, err := operation(repository.New(tx))
	if err != nil {
		return zero, err
	}
	if err := tx.Commit(ctx); err != nil {
		return zero, fmt.Errorf("commit Scrum transaction: %w", err)
	}
	return result, nil
}

func marshalOptional(value any) ([]byte, error) {
	if value == nil {
		return nil, nil
	}
	return json.Marshal(value)
}

func optionalText(value string) pgtype.Text {
	value = strings.TrimSpace(value)
	return pgtype.Text{String: value, Valid: value != ""}
}

func defaultIfEmpty(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func validatePriority(priority string) error {
	if !oneOf(priority, "low", "medium", "high", "critical") {
		return invalid("priority is invalid")
	}
	return nil
}

func oneOf(value string, choices ...string) bool {
	for _, choice := range choices {
		if value == choice {
			return true
		}
	}
	return false
}

func invalid(message string) error {
	return fmt.Errorf("%w: %s", ErrInvalidInput, message)
}

func notFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return fmt.Errorf("load or update Scrum item: %w", err)
}
