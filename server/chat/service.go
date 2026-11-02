package chat

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/ben-anass/DouzTach/internal/repository"
	"github.com/ben-anass/DouzTach/server/activity"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalidInput = errors.New("invalid chat input")
	ErrNotFound     = errors.New("chat resource not found")
)

const maxPageSize = 100

type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

func (s *Service) CreateChannel(
	ctx context.Context,
	workspaceID, actorID pgtype.UUID,
	name, kind string,
) (repository.ChatChannel, error) {
	name = strings.TrimSpace(name)
	kind = strings.TrimSpace(kind)
	if kind == "" {
		kind = "custom"
	}
	if !workspaceID.Valid || !actorID.Valid || name == "" || utf8.RuneCountInString(name) > 100 {
		return repository.ChatChannel{}, invalid("workspace, actor, and a channel name of 1 to 100 characters are required")
	}
	if !oneOf(kind, "general", "announcements", "development", "custom") {
		return repository.ChatChannel{}, invalid("channel kind is invalid")
	}

	channel, err := inTransaction(ctx, s.pool, func(q *repository.Queries) (repository.ChatChannel, error) {
		if err := activity.RequireWorkspaceMember(ctx, q, workspaceID, actorID); err != nil {
			return repository.ChatChannel{}, err
		}
		created, err := q.CreateChatChannel(ctx, repository.CreateChatChannelParams{
			WorkspaceID: workspaceID,
			Name:        name,
			Kind:        kind,
		})
		if err != nil {
			return repository.ChatChannel{}, fmt.Errorf("create chat channel: %w", err)
		}
		if err := activity.Record(ctx, q, actorID, workspaceID, created.ID, "created", "chat_channel", nil, created); err != nil {
			return repository.ChatChannel{}, err
		}
		return created, nil
	})
	return channel, err
}

func (s *Service) ListChannels(
	ctx context.Context,
	workspaceID, userID pgtype.UUID,
) ([]repository.ChatChannel, error) {
	if !workspaceID.Valid || !userID.Valid {
		return nil, invalid("workspace and user IDs are required")
	}
	q := repository.New(s.pool)
	if err := activity.RequireWorkspaceMember(ctx, q, workspaceID, userID); err != nil {
		return nil, err
	}
	channels, err := q.ListChatChannels(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list chat channels: %w", err)
	}
	return channels, nil
}

func (s *Service) PostMessage(
	ctx context.Context,
	workspaceID, channelID, senderID pgtype.UUID,
	content string,
) (repository.ChatMessage, error) {
	content = strings.TrimSpace(content)
	if !workspaceID.Valid || !channelID.Valid || !senderID.Valid || content == "" {
		return repository.ChatMessage{}, invalid("workspace, channel, sender, and non-empty message content are required")
	}

	message, err := inTransaction(ctx, s.pool, func(q *repository.Queries) (repository.ChatMessage, error) {
		if err := activity.RequireWorkspaceMember(ctx, q, workspaceID, senderID); err != nil {
			return repository.ChatMessage{}, err
		}
		if _, err := q.GetChatChannel(ctx, repository.GetChatChannelParams{
			ID:          channelID,
			WorkspaceID: workspaceID,
		}); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return repository.ChatMessage{}, ErrNotFound
			}
			return repository.ChatMessage{}, fmt.Errorf("get chat channel: %w", err)
		}
		created, err := q.CreateChatMessage(ctx, repository.CreateChatMessageParams{
			WorkspaceID: workspaceID,
			ChannelID:   channelID,
			SenderID:    senderID,
			Content:     content,
		})
		if err != nil {
			return repository.ChatMessage{}, fmt.Errorf("create chat message: %w", err)
		}
		if err := activity.Record(ctx, q, senderID, workspaceID, created.ID, "created", "chat_message", nil, created); err != nil {
			return repository.ChatMessage{}, err
		}
		return created, nil
	})
	return message, err
}

func (s *Service) ListMessages(
	ctx context.Context,
	workspaceID, channelID, userID pgtype.UUID,
	pageSize, pageOffset int32,
) ([]repository.ChatMessage, error) {
	if !workspaceID.Valid || !channelID.Valid || !userID.Valid {
		return nil, invalid("workspace, channel, and user IDs are required")
	}
	if err := validatePage(pageSize, pageOffset); err != nil {
		return nil, err
	}
	q := repository.New(s.pool)
	if err := activity.RequireWorkspaceMember(ctx, q, workspaceID, userID); err != nil {
		return nil, err
	}
	if _, err := q.GetChatChannel(ctx, repository.GetChatChannelParams{
		ID:          channelID,
		WorkspaceID: workspaceID,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get chat channel: %w", err)
	}
	messages, err := q.ListChannelMessages(ctx, repository.ListChannelMessagesParams{
		ChannelID:   channelID,
		WorkspaceID: workspaceID,
		PageSize:    pageSize,
		PageOffset:  pageOffset,
	})
	if err != nil {
		return nil, fmt.Errorf("list channel messages: %w", err)
	}
	return messages, nil
}

func inTransaction[T any](ctx context.Context, pool *pgxpool.Pool, operation func(*repository.Queries) (T, error)) (T, error) {
	var zero T
	tx, err := pool.Begin(ctx)
	if err != nil {
		return zero, fmt.Errorf("begin chat transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	result, err := operation(repository.New(tx))
	if err != nil {
		return zero, err
	}
	if err := tx.Commit(ctx); err != nil {
		return zero, fmt.Errorf("commit chat transaction: %w", err)
	}
	return result, nil
}

func validatePage(pageSize, pageOffset int32) error {
	if pageSize < 1 || pageSize > maxPageSize || pageOffset < 0 {
		return invalid("page size must be between 1 and 100 and offset cannot be negative")
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
