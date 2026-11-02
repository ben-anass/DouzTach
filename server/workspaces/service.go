package workspaces

import (
	"context"
	"errors"
	"fmt"
	"github.com/ben-anass/DouzTach/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
)

type Service struct {
	queries *repository.Queries
	db      *pgxpool.Pool
}

func NewService(queries *repository.Queries, db *pgxpool.Pool) *Service {
	return &Service{
		queries: queries,
		db:      db,
	}
}

func (s *Service) CreateWorkspace(
	ctx context.Context,
	ownerUserID string,
	name string,
	nickname string,
	description string,
	kind string,
) (repository.Workspace, error) {
	name = strings.TrimSpace(name)
	nickname = strings.TrimSpace(nickname)
	description = strings.TrimSpace(description)
	kind = strings.TrimSpace(kind)

	if name == "" {
		return repository.Workspace{}, errors.New("workspace name is required")
	}

	if kind != "personal" && kind != "team" && kind != "squad" {
		return repository.Workspace{}, errors.New("invalid workspace kind")
	}

	var ownerID pgtype.UUID
	if err := ownerID.Scan(ownerUserID); err != nil {
		return repository.Workspace{}, errors.New("invalid owner user ID")
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return repository.Workspace{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	qtx := s.queries.WithTx(tx)

	workspace, err := qtx.CreateWorkspace(ctx, repository.CreateWorkspaceParams{
		OwnerUserID: ownerID,
		Name:        name,
		Nickname:    pgtype.Text{String: nickname, Valid: nickname != ""},
		Description: pgtype.Text{
			String: description,
			Valid:  description != "",
		},
		Kind:        kind,
		Theme:       pgtype.Text{String: "default", Valid: true},
		AccentColor: pgtype.Text{String: "#6D7CFF", Valid: true},
	})
	if err != nil {
		return repository.Workspace{}, fmt.Errorf("create workspace: %w", err)
	}

	_, err = qtx.AddWorkspaceMember(ctx, repository.AddWorkspaceMemberParams{
		WorkspaceID: workspace.ID,
		UserID:      ownerID,
		Role:        "owner",
	})
	if err != nil {
		return repository.Workspace{}, fmt.Errorf("add workspace owner: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return repository.Workspace{}, fmt.Errorf("commit workspace transaction: %w", err)
	}

	return workspace, nil
}

func (s *Service) ListWorkspacesForUser(ctx context.Context, userID string) ([]repository.Workspace, error) {
	var id pgtype.UUID

	if err := id.Scan(userID); err != nil {
		return nil, errors.New("invalid user ID")
	}

	workspaces, err := s.queries.ListWorkspacesForUser(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("list workspaces: %w", err)
	}

	return workspaces, nil
}

func (s *Service) ListWorkspaceMembers(ctx context.Context, workspaceID, userID string) ([]repository.WorkspaceMember, error) {
	var workspaceUUID pgtype.UUID
	var userUUID pgtype.UUID

	if err := workspaceUUID.Scan(workspaceID); err != nil {
		return nil, errors.New("invalid workspace ID")
	}

	if err := userUUID.Scan(userID); err != nil {
		return nil, errors.New("invalid user ID")
	}

	// Check if the current user is a member.
	isMember, err := s.queries.IsWorkspaceMember(ctx, repository.IsWorkspaceMemberParams{
		WorkspaceID: workspaceUUID,
		UserID:      userUUID,
	})
	if err != nil {
		return nil, fmt.Errorf("check workspace membership: %w", err)
	}

	if !isMember {
		return nil, errors.New("user is not a workspace member")
	}

	// Get all workspace members.
	members, err := s.queries.ListWorkspaceMembers(ctx, workspaceUUID)
	if err != nil {
		return nil, fmt.Errorf("list workspace members: %w", err)
	}

	if members == nil {
		return []repository.WorkspaceMember{}, nil
	}

	return members, nil
}
