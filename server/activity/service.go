package activity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ben-anass/DouzTach/internal/repository"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalidInput = errors.New("invalid activity input")
	ErrNotMember    = errors.New("user is not a workspace member")
)

const maxPageSize = 100

type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

func (s *Service) ListForWorkspace(
	ctx context.Context,
	workspaceID, userID pgtype.UUID,
	pageSize, pageOffset int32,
) ([]repository.ActivityLog, error) {
	if !workspaceID.Valid || !userID.Valid {
		return nil, invalid("workspace and user IDs are required")
	}
	if err := validatePage(pageSize, pageOffset); err != nil {
		return nil, err
	}

	q := repository.New(s.pool)
	if err := requireWorkspaceMember(ctx, q, workspaceID, userID); err != nil {
		return nil, err
	}
	items, err := q.ListActivityForWorkspace(ctx, repository.ListActivityForWorkspaceParams{
		WorkspaceID: workspaceID,
		PageSize:    pageSize,
		PageOffset:  pageOffset,
	})
	if err != nil {
		return nil, fmt.Errorf("list workspace activity: %w", err)
	}
	return items, nil
}

func Record(
	ctx context.Context,
	q *repository.Queries,
	actorID, workspaceID, entityID pgtype.UUID,
	action, entity string,
	previous, current any,
) error {
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

func RequireWorkspaceMember(ctx context.Context, q *repository.Queries, workspaceID, userID pgtype.UUID) error {
	return requireWorkspaceMember(ctx, q, workspaceID, userID)
}

func requireWorkspaceMember(ctx context.Context, q *repository.Queries, workspaceID, userID pgtype.UUID) error {
	isMember, err := q.IsWorkspaceMember(ctx, repository.IsWorkspaceMemberParams{
		WorkspaceID: workspaceID,
		UserID:      userID,
	})
	if err != nil {
		return fmt.Errorf("check workspace membership: %w", err)
	}
	if !isMember {
		return ErrNotMember
	}
	return nil
}

func validatePage(pageSize, pageOffset int32) error {
	if pageSize < 1 || pageSize > maxPageSize || pageOffset < 0 {
		return invalid("page size must be between 1 and 100 and offset cannot be negative")
	}
	return nil
}

func marshalOptional(value any) ([]byte, error) {
	if value == nil {
		return nil, nil
	}
	return json.Marshal(value)
}

func invalid(message string) error {
	return fmt.Errorf("%w: %s", ErrInvalidInput, message)
}
