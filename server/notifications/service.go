package notifications

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/ben-anass/DouzTach/internal/repository"
	"github.com/ben-anass/DouzTach/server/activity"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalidInput = errors.New("invalid notification input")
	ErrNotFound     = errors.New("notification not found")
)

const maxPageSize = 100

type Service struct {
	pool *pgxpool.Pool
}

type CreateInput struct {
	RecipientUserID pgtype.UUID
	WorkspaceID     pgtype.UUID
	EventType       string
	RelatedEntityID pgtype.UUID
	Message         string
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

func (s *Service) Create(ctx context.Context, input CreateInput) (repository.Notification, error) {
	input.EventType = strings.TrimSpace(input.EventType)
	input.Message = strings.TrimSpace(input.Message)
	if !input.RecipientUserID.Valid || !input.WorkspaceID.Valid || input.EventType == "" || input.Message == "" {
		return repository.Notification{}, invalid("recipient, workspace, event type, and message are required")
	}

	q := repository.New(s.pool)
	if err := activity.RequireWorkspaceMember(ctx, q, input.WorkspaceID, input.RecipientUserID); err != nil {
		return repository.Notification{}, err
	}
	notification, err := q.CreateNotification(ctx, repository.CreateNotificationParams{
		RecipientUserID: input.RecipientUserID,
		WorkspaceID:     input.WorkspaceID,
		EventType:       input.EventType,
		RelatedEntityID: input.RelatedEntityID,
		Message:         input.Message,
	})
	if err != nil {
		return repository.Notification{}, fmt.Errorf("create notification: %w", err)
	}
	return notification, nil
}

func (s *Service) ListForUser(
	ctx context.Context,
	userID pgtype.UUID,
	pageSize, pageOffset int32,
) ([]repository.Notification, error) {
	if !userID.Valid {
		return nil, invalid("user ID is required")
	}
	if err := validatePage(pageSize, pageOffset); err != nil {
		return nil, err
	}
	notifications, err := repository.New(s.pool).ListNotificationsForUser(ctx, repository.ListNotificationsForUserParams{
		RecipientUserID: userID,
		PageSize:        pageSize,
		PageOffset:      pageOffset,
	})
	if err != nil {
		return nil, fmt.Errorf("list notifications: %w", err)
	}
	return notifications, nil
}

func (s *Service) MarkRead(ctx context.Context, userID, notificationID pgtype.UUID) (repository.Notification, error) {
	if !userID.Valid || !notificationID.Valid {
		return repository.Notification{}, invalid("user and notification IDs are required")
	}
	notification, err := repository.New(s.pool).MarkNotificationRead(ctx, repository.MarkNotificationReadParams{
		ID:              notificationID,
		RecipientUserID: userID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return repository.Notification{}, ErrNotFound
		}
		return repository.Notification{}, fmt.Errorf("mark notification as read: %w", err)
	}
	return notification, nil
}

func validatePage(pageSize, pageOffset int32) error {
	if pageSize < 1 || pageSize > maxPageSize || pageOffset < 0 {
		return invalid("page size must be between 1 and 100 and offset cannot be negative")
	}
	return nil
}

func invalid(message string) error {
	return fmt.Errorf("%w: %s", ErrInvalidInput, message)
}
