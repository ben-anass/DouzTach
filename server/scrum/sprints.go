package scrum

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/ben-anass/DouzTach/internal/repository"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrActiveSprint        = errors.New("workspace already has an active sprint")
	ErrInvalidTransition   = errors.New("invalid sprint status transition")
	ErrStoryAlreadyPlanned = errors.New("user story is already assigned to a sprint")
)

type CreateSprintInput struct {
	WorkspaceID pgtype.UUID
	ActorUserID pgtype.UUID
	Name        string
	Goal        string
	StartDate   pgtype.Date
	EndDate     pgtype.Date
}

type UpdateSprintInput struct {
	SprintID pgtype.UUID
	CreateSprintInput
}

type SprintReport struct {
	TotalStories         int64
	CompletedStories     int64
	CommittedStoryPoints int64
	Velocity             int64
	CompletionRate       float64
}

func (s *Service) CreateSprint(ctx context.Context, input CreateSprintInput) (repository.Sprint, error) {
	input.Name = strings.TrimSpace(input.Name)
	if err := validateSprint(input.WorkspaceID, input.ActorUserID, input.Name, input.StartDate, input.EndDate); err != nil {
		return repository.Sprint{}, err
	}

	return inTransaction(ctx, s.pool, func(q *repository.Queries) (repository.Sprint, error) {
		sprint, err := q.CreateSprint(ctx, repository.CreateSprintParams{
			WorkspaceID: input.WorkspaceID,
			Name:        input.Name,
			Goal:        optionalText(input.Goal),
			Status:      "planned",
			StartDate:   input.StartDate,
			EndDate:     input.EndDate,
		})
		if err != nil {
			return repository.Sprint{}, fmt.Errorf("create sprint: %w", err)
		}
		if err := recordActivity(ctx, q, input.ActorUserID, input.WorkspaceID, "created", "sprint", sprint.ID, nil, sprint); err != nil {
			return repository.Sprint{}, err
		}
		return sprint, nil
	})
}

func (s *Service) ListSprints(ctx context.Context, workspaceID pgtype.UUID) ([]repository.Sprint, error) {
	if !workspaceID.Valid {
		return nil, invalid("workspace ID is required")
	}
	sprints, err := repository.New(s.pool).ListSprintsForWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list sprints: %w", err)
	}
	return sprints, nil
}

func (s *Service) GetSprint(ctx context.Context, workspaceID, sprintID pgtype.UUID) (repository.Sprint, error) {
	if !workspaceID.Valid || !sprintID.Valid {
		return repository.Sprint{}, invalid("workspace and sprint IDs are required")
	}
	sprint, err := repository.New(s.pool).GetSprint(ctx, repository.GetSprintParams{
		ID:          sprintID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return repository.Sprint{}, notFound(err)
	}
	return sprint, nil
}

func (s *Service) UpdateSprint(ctx context.Context, input UpdateSprintInput) (repository.Sprint, error) {
	input.Name = strings.TrimSpace(input.Name)
	if !input.SprintID.Valid {
		return repository.Sprint{}, invalid("sprint ID is required")
	}
	if err := validateSprint(input.WorkspaceID, input.ActorUserID, input.Name, input.StartDate, input.EndDate); err != nil {
		return repository.Sprint{}, err
	}

	return inTransaction(ctx, s.pool, func(q *repository.Queries) (repository.Sprint, error) {
		previous, err := q.GetSprintForUpdate(ctx, repository.GetSprintForUpdateParams{
			ID:          input.SprintID,
			WorkspaceID: input.WorkspaceID,
		})
		if err != nil {
			return repository.Sprint{}, notFound(err)
		}
		if previous.Status != "planned" {
			return repository.Sprint{}, ErrInvalidTransition
		}
		updated, err := q.UpdatePlannedSprint(ctx, repository.UpdatePlannedSprintParams{
			ID:          input.SprintID,
			WorkspaceID: input.WorkspaceID,
			Name:        input.Name,
			Goal:        optionalText(input.Goal),
			StartDate:   input.StartDate,
			EndDate:     input.EndDate,
		})
		if err != nil {
			return repository.Sprint{}, fmt.Errorf("update sprint: %w", err)
		}
		if err := recordActivity(ctx, q, input.ActorUserID, input.WorkspaceID, "updated", "sprint", input.SprintID, previous, updated); err != nil {
			return repository.Sprint{}, err
		}
		return updated, nil
	})
}

func (s *Service) StartSprint(ctx context.Context, workspaceID, actorUserID, sprintID pgtype.UUID) (repository.Sprint, error) {
	return s.transitionSprint(ctx, workspaceID, actorUserID, sprintID, "planned", "started", func(q *repository.Queries) (repository.Sprint, error) {
		return q.StartSprint(ctx, repository.StartSprintParams{ID: sprintID, WorkspaceID: workspaceID})
	})
}

func (s *Service) CompleteSprint(ctx context.Context, workspaceID, actorUserID, sprintID pgtype.UUID) (repository.Sprint, error) {
	return s.transitionSprint(ctx, workspaceID, actorUserID, sprintID, "active", "completed", func(q *repository.Queries) (repository.Sprint, error) {
		return q.CompleteSprint(ctx, repository.CompleteSprintParams{ID: sprintID, WorkspaceID: workspaceID})
	})
}

func (s *Service) ReopenSprint(ctx context.Context, workspaceID, actorUserID, sprintID pgtype.UUID) (repository.Sprint, error) {
	return s.transitionSprint(ctx, workspaceID, actorUserID, sprintID, "completed", "reopened", func(q *repository.Queries) (repository.Sprint, error) {
		return q.ReopenSprint(ctx, repository.ReopenSprintParams{ID: sprintID, WorkspaceID: workspaceID})
	})
}

func (s *Service) transitionSprint(
	ctx context.Context,
	workspaceID, actorUserID, sprintID pgtype.UUID,
	expectedStatus, action string,
	transition func(*repository.Queries) (repository.Sprint, error),
) (repository.Sprint, error) {
	if !workspaceID.Valid || !actorUserID.Valid || !sprintID.Valid {
		return repository.Sprint{}, invalid("workspace, actor, and sprint IDs are required")
	}

	sprint, err := inTransaction(ctx, s.pool, func(q *repository.Queries) (repository.Sprint, error) {
		previous, err := q.GetSprintForUpdate(ctx, repository.GetSprintForUpdateParams{
			ID:          sprintID,
			WorkspaceID: workspaceID,
		})
		if err != nil {
			return repository.Sprint{}, notFound(err)
		}
		if previous.Status != expectedStatus {
			return repository.Sprint{}, ErrInvalidTransition
		}
		if action == "started" || action == "reopened" {
			if !previous.StartDate.Valid || !previous.EndDate.Valid {
				return repository.Sprint{}, invalid("sprint start and end dates are required before activation")
			}
		}
		updated, err := transition(q)
		if err != nil {
			if isActiveSprintConflict(err) {
				return repository.Sprint{}, ErrActiveSprint
			}
			return repository.Sprint{}, fmt.Errorf("%s sprint: %w", action, err)
		}
		if err := recordActivity(ctx, q, actorUserID, workspaceID, action, "sprint", sprintID, previous, updated); err != nil {
			return repository.Sprint{}, err
		}
		return updated, nil
	})
	return sprint, err
}

func (s *Service) ListBacklog(ctx context.Context, workspaceID pgtype.UUID) ([]repository.ListBacklogStoriesRow, error) {
	if !workspaceID.Valid {
		return nil, invalid("workspace ID is required")
	}
	stories, err := repository.New(s.pool).ListBacklogStories(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list backlog: %w", err)
	}
	return stories, nil
}

func (s *Service) SetBacklogStoryPosition(ctx context.Context, workspaceID, actorUserID, storyID pgtype.UUID, position int32) error {
	if !workspaceID.Valid || !actorUserID.Valid || !storyID.Valid {
		return invalid("workspace, actor, and story IDs are required")
	}
	if position < 0 {
		return invalid("backlog position cannot be negative")
	}

	_, err := inTransaction(ctx, s.pool, func(q *repository.Queries) (struct{}, error) {
		previous, err := q.GetUserStory(ctx, repository.GetUserStoryParams{ID: storyID, WorkspaceID: workspaceID})
		if err != nil {
			return struct{}{}, notFound(err)
		}
		updated, err := q.SetBacklogStoryPosition(ctx, repository.SetBacklogStoryPositionParams{
			ID:              storyID,
			WorkspaceID:     workspaceID,
			BacklogPosition: position,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return struct{}{}, ErrInvalidReference
			}
			return struct{}{}, fmt.Errorf("set backlog position: %w", err)
		}
		if err := recordActivity(ctx, q, actorUserID, workspaceID, "backlog_reordered", "user_story", storyID, map[string]int32{"position": previous.BacklogPosition}, map[string]int32{"position": updated.BacklogPosition}); err != nil {
			return struct{}{}, err
		}
		return struct{}{}, nil
	})
	return err
}

func (s *Service) AddStoryToSprint(ctx context.Context, workspaceID, actorUserID, sprintID, storyID pgtype.UUID) error {
	if !workspaceID.Valid || !actorUserID.Valid || !sprintID.Valid || !storyID.Valid {
		return invalid("workspace, actor, sprint, and story IDs are required")
	}

	_, err := inTransaction(ctx, s.pool, func(q *repository.Queries) (struct{}, error) {
		sprint, err := q.GetSprintForUpdate(ctx, repository.GetSprintForUpdateParams{ID: sprintID, WorkspaceID: workspaceID})
		if err != nil {
			return struct{}{}, notFound(err)
		}
		if sprint.Status != "planned" {
			return struct{}{}, ErrInvalidTransition
		}
		check, err := q.CanAddStoryToSprint(ctx, repository.CanAddStoryToSprintParams{
			StoryID:     storyID,
			WorkspaceID: workspaceID,
			SprintID:    sprintID,
		})
		if err != nil {
			return struct{}{}, fmt.Errorf("validate sprint story: %w", err)
		}
		if !check.StoryValid || !check.SprintValid {
			return struct{}{}, ErrInvalidReference
		}
		if !check.StoryUnassigned {
			return struct{}{}, ErrStoryAlreadyPlanned
		}
		position, err := q.NextSprintStoryPosition(ctx, sprintID)
		if err != nil {
			return struct{}{}, fmt.Errorf("find next sprint story position: %w", err)
		}
		_, err = q.AddStoryToSprint(ctx, repository.AddStoryToSprintParams{
			SprintID:    sprintID,
			StoryID:     storyID,
			WorkspaceID: workspaceID,
			Position:    position,
		})
		if err != nil {
			if isSprintStoryConflict(err) {
				return struct{}{}, ErrStoryAlreadyPlanned
			}
			return struct{}{}, fmt.Errorf("add story to sprint: %w", err)
		}
		if err := recordActivity(ctx, q, actorUserID, workspaceID, "story_added_to_sprint", "user_story", storyID, nil, map[string]any{
			"sprint_id": sprintID,
			"position":  position,
		}); err != nil {
			return struct{}{}, err
		}
		return struct{}{}, nil
	})
	return err
}

func (s *Service) RemoveStoryFromSprint(ctx context.Context, workspaceID, actorUserID, sprintID, storyID pgtype.UUID) error {
	if !workspaceID.Valid || !actorUserID.Valid || !sprintID.Valid || !storyID.Valid {
		return invalid("workspace, actor, sprint, and story IDs are required")
	}
	_, err := inTransaction(ctx, s.pool, func(q *repository.Queries) (struct{}, error) {
		sprint, err := q.GetSprintForUpdate(ctx, repository.GetSprintForUpdateParams{ID: sprintID, WorkspaceID: workspaceID})
		if err != nil {
			return struct{}{}, notFound(err)
		}
		if sprint.Status != "planned" {
			return struct{}{}, ErrInvalidTransition
		}
		removed, err := q.RemoveStoryFromSprint(ctx, repository.RemoveStoryFromSprintParams{
			SprintID:    sprintID,
			StoryID:     storyID,
			WorkspaceID: workspaceID,
		})
		if err != nil {
			return struct{}{}, notFound(err)
		}
		if err := recordActivity(ctx, q, actorUserID, workspaceID, "story_removed_from_sprint", "user_story", storyID, map[string]any{
			"sprint_id": removed.SprintID,
			"position":  removed.Position,
		}, nil); err != nil {
			return struct{}{}, err
		}
		return struct{}{}, nil
	})
	return err
}

func (s *Service) ListStoriesForSprint(ctx context.Context, workspaceID, sprintID pgtype.UUID) ([]repository.ListStoriesForSprintRow, error) {
	if !workspaceID.Valid || !sprintID.Valid {
		return nil, invalid("workspace and sprint IDs are required")
	}
	stories, err := repository.New(s.pool).ListStoriesForSprint(ctx, repository.ListStoriesForSprintParams{
		ID:          sprintID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return nil, fmt.Errorf("list sprint stories: %w", err)
	}
	return stories, nil
}

func (s *Service) GetSprintReport(ctx context.Context, workspaceID, sprintID pgtype.UUID) (SprintReport, error) {
	if !workspaceID.Valid || !sprintID.Valid {
		return SprintReport{}, invalid("workspace and sprint IDs are required")
	}
	row, err := repository.New(s.pool).GetSprintReport(ctx, repository.GetSprintReportParams{
		ID:          sprintID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SprintReport{}, ErrNotFound
		}
		return SprintReport{}, fmt.Errorf("get sprint report: %w", err)
	}
	report := SprintReport{
		TotalStories:         row.TotalStories,
		CompletedStories:     row.CompletedStories,
		CommittedStoryPoints: row.CommittedStoryPoints,
		Velocity:             row.CompletedStoryPoints,
	}
	if report.TotalStories > 0 {
		report.CompletionRate = float64(report.CompletedStories) / float64(report.TotalStories) * 100
	}
	return report, nil
}

func validateSprint(workspaceID, actorID pgtype.UUID, name string, startDate, endDate pgtype.Date) error {
	if !workspaceID.Valid || !actorID.Valid || name == "" || len(name) > 200 {
		return invalid("workspace, actor, and a sprint name of 1-200 characters are required")
	}
	if startDate.Valid && endDate.Valid && startDate.Time.After(endDate.Time) {
		return invalid("sprint start date must not be after end date")
	}
	return nil
}

func isActiveSprintConflict(err error) bool {
	var postgresError *pgconn.PgError
	return errors.As(err, &postgresError) &&
		postgresError.Code == "23505" &&
		postgresError.ConstraintName == "idx_sprints_one_active_per_workspace"
}

func isSprintStoryConflict(err error) bool {
	var postgresError *pgconn.PgError
	return errors.As(err, &postgresError) &&
		postgresError.Code == "23505" &&
		postgresError.ConstraintName == "idx_sprint_stories_story_id"
}
