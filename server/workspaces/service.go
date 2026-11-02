package workspaces

import (
	"context"
	"errors"
	"fmt"
	"github.com/ben-anass/DouzTach/internal/repository"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"strings"
	"time"
	"unicode/utf8"

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

func ensureWorkspaceActive(
	ctx context.Context,
	q *repository.Queries,
	workspaceID pgtype.UUID,
) error {

	workspace, err := q.GetWorkspaceByID(ctx, workspaceID)

	if errors.Is(err, pgx.ErrNoRows) {
		return errors.New("workspace not found")
	}

	if err != nil {
		return fmt.Errorf("get workspace: %w", err)
	}

	if workspace.DeletedAt.Valid {
		return errors.New("workspace is deleted")
	}

	return nil
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
	if err := ensureWorkspaceActive(
		ctx,
		s.queries,
		workspaceUUID,
	); err != nil {
		return nil, err
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

func (s *Service) TransferOwnership(
	ctx context.Context,
	workspaceID string,
	requesterUserID string,
	newOwnerUserID string,
) (repository.Workspace, error) {

	var workspaceUUID pgtype.UUID
	var requesterUUID pgtype.UUID
	var newOwnerUUID pgtype.UUID

	if err := workspaceUUID.Scan(workspaceID); err != nil {
		return repository.Workspace{}, errors.New("invalid workspace ID")
	}

	if err := requesterUUID.Scan(requesterUserID); err != nil {
		return repository.Workspace{}, errors.New("invalid user ID")
	}

	if err := newOwnerUUID.Scan(newOwnerUserID); err != nil {
		return repository.Workspace{}, errors.New("invalid target user ID")
	}

	if requesterUUID.Bytes == newOwnerUUID.Bytes {
		return repository.Workspace{}, errors.New(
			"cannot transfer ownership to yourself",
		)
	}

	// Begin transaction.
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return repository.Workspace{}, fmt.Errorf(
			"begin transaction: %w", err,
		)
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	qtx := s.queries.WithTx(tx)

	// Lock workspace.
	workspace, err := qtx.GetWorkspaceByIDForUpdate(
		ctx,
		workspaceUUID,
	)
	if workspace.DeletedAt.Valid {
		return repository.Workspace{}, errors.New("workspace is deleted")
	}

	if errors.Is(err, pgx.ErrNoRows) {
		return repository.Workspace{}, errors.New("workspace not found")
	}

	if err != nil {
		return repository.Workspace{}, fmt.Errorf(
			"get workspace: %w", err,
		)
	}

	// Verify current owner.
	if !workspace.OwnerUserID.Valid ||
		workspace.OwnerUserID.Bytes != requesterUUID.Bytes {
		return repository.Workspace{}, errors.New(
			"only workspace owner can transfer ownership",
		)
	}

	// Lock current owner's membership.
	currentRole, err := qtx.GetWorkspaceMemberRoleForUpdate(
		ctx,
		repository.GetWorkspaceMemberRoleForUpdateParams{
			WorkspaceID: workspaceUUID,
			UserID:      requesterUUID,
		},
	)

	if err != nil {
		return repository.Workspace{}, fmt.Errorf(
			"verify current owner membership: %w", err,
		)
	}

	if currentRole != "owner" {
		return repository.Workspace{}, errors.New(
			"workspace owner membership is inconsistent",
		)
	}

	// Verify and lock new owner's membership.
	targetRole, err := qtx.GetWorkspaceMemberRoleForUpdate(
		ctx,
		repository.GetWorkspaceMemberRoleForUpdateParams{
			WorkspaceID: workspaceUUID,
			UserID:      newOwnerUUID,
		},
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return repository.Workspace{}, errors.New(
			"target user is not a workspace member",
		)
	}

	if err != nil {
		return repository.Workspace{}, fmt.Errorf(
			"verify target membership: %w", err,
		)
	}

	if targetRole == "owner" {
		return repository.Workspace{}, errors.New(
			"target user is already an owner",
		)
	}

	// Demote old owner to admin.
	_, err = qtx.DemoteWorkspaceOwner(
		ctx,
		repository.DemoteWorkspaceOwnerParams{
			WorkspaceID: workspaceUUID,
			UserID:      requesterUUID,
		},
	)

	if err != nil {
		return repository.Workspace{}, fmt.Errorf(
			"demote current owner: %w", err,
		)
	}

	// Promote target member to owner.
	_, err = qtx.UpdateWorkspaceMemberRole(
		ctx,
		repository.UpdateWorkspaceMemberRoleParams{
			Role:        "owner",
			WorkspaceID: workspaceUUID,
			UserID:      newOwnerUUID,
		},
	)

	if err != nil {
		return repository.Workspace{}, fmt.Errorf(
			"promote new owner: %w", err,
		)
	}

	// Update workspace owner_user_id.
	updatedWorkspace, err := qtx.UpdateWorkspaceOwner(
		ctx,
		repository.UpdateWorkspaceOwnerParams{
			NewOwnerUserID:     newOwnerUUID,
			ID:                 workspaceUUID,
			CurrentOwnerUserID: requesterUUID,
		},
	)

	if err != nil {
		return repository.Workspace{}, fmt.Errorf(
			"update workspace owner: %w", err,
		)
	}

	// Commit all changes together.
	if err := tx.Commit(ctx); err != nil {
		return repository.Workspace{}, fmt.Errorf(
			"commit ownership transfer: %w", err,
		)
	}

	return updatedWorkspace, nil
}

func (s *Service) UpdateWorkspace(
	ctx context.Context,
	workspaceID string,
	requesterUserID string,
	name string,
	nickname string,
	description string,
	theme string,
	accentColor string,
) (repository.Workspace, error) {

	// Normalize input.
	name = strings.TrimSpace(name)
	nickname = strings.TrimSpace(nickname)
	description = strings.TrimSpace(description)
	theme = strings.TrimSpace(theme)
	accentColor = strings.TrimSpace(accentColor)

	// Validate required fields.
	if name == "" {
		return repository.Workspace{}, errors.New("workspace name is required")
	}

	if utf8.RuneCountInString(name) > 150 {
		return repository.Workspace{}, errors.New("workspace name is too long")
	}

	if utf8.RuneCountInString(nickname) > 100 {
		return repository.Workspace{}, errors.New("nickname is too long")
	}

	if utf8.RuneCountInString(theme) > 50 {
		return repository.Workspace{}, errors.New("theme is too long")
	}

	if theme == "" {
		theme = "default"
	}

	if accentColor == "" {
		accentColor = "#6D7CFF"
	}

	if utf8.RuneCountInString(accentColor) > 30 {
		return repository.Workspace{}, errors.New("accent color is too long")
	}

	// Parse UUIDs.
	var workspaceUUID pgtype.UUID
	var requesterUUID pgtype.UUID

	if err := workspaceUUID.Scan(workspaceID); err != nil {
		return repository.Workspace{}, errors.New("invalid workspace ID")
	}

	if err := requesterUUID.Scan(requesterUserID); err != nil {
		return repository.Workspace{}, errors.New("invalid user ID")
	}

	// Begin transaction.
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return repository.Workspace{}, fmt.Errorf("begin transaction: %w", err)
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	qtx := s.queries.WithTx(tx)

	// Lock workspace.
	workspace, err := qtx.GetWorkspaceByIDForUpdate(
		ctx,
		workspaceUUID,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return repository.Workspace{}, errors.New("workspace not found")
	}

	if err != nil {
		return repository.Workspace{}, fmt.Errorf("get workspace: %w", err)
	}

	if workspace.DeletedAt.Valid {
		return repository.Workspace{}, errors.New("workspace is deleted")
	}

	// Lock and check requester role.
	requesterRole, err := qtx.GetWorkspaceMemberRoleForUpdate(
		ctx,
		repository.GetWorkspaceMemberRoleForUpdateParams{
			WorkspaceID: workspaceUUID,
			UserID:      requesterUUID,
		},
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return repository.Workspace{}, errors.New("user is not a workspace member")
	}

	if err != nil {
		return repository.Workspace{}, fmt.Errorf(
			"get requester role: %w", err,
		)
	}

	// Validate ownership consistency.
	if requesterRole == "owner" {
		if !workspace.OwnerUserID.Valid ||
			workspace.OwnerUserID.Bytes != requesterUUID.Bytes {
			return repository.Workspace{}, errors.New(
				"workspace owner membership is inconsistent",
			)
		}
	}

	// Only Owner and Admin can update workspace.
	if requesterRole != "owner" && requesterRole != "admin" {
		return repository.Workspace{}, errors.New(
			"you do not have permission to update workspace",
		)
	}

	// Update workspace information.
	updatedWorkspace, err := qtx.UpdateWorkspace(
		ctx,
		repository.UpdateWorkspaceParams{
			Name:        name,
			Nickname:    pgtype.Text{String: nickname, Valid: nickname != ""},
			Description: pgtype.Text{String: description, Valid: description != ""},
			Theme:       pgtype.Text{String: theme, Valid: true},
			AccentColor: pgtype.Text{String: accentColor, Valid: true},
			ID:          workspaceUUID,
		},
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return repository.Workspace{}, errors.New("workspace not found")
	}

	if err != nil {
		return repository.Workspace{}, fmt.Errorf(
			"update workspace: %w", err,
		)
	}

	// Commit transaction.
	if err := tx.Commit(ctx); err != nil {
		return repository.Workspace{}, fmt.Errorf(
			"commit workspace update: %w", err,
		)
	}

	return updatedWorkspace, nil
}

func (s *Service) DeleteWorkspace(
	ctx context.Context,
	workspaceID string,
	requesterUserID string,
) (repository.Workspace, error) {

	var workspaceUUID pgtype.UUID
	var requesterUUID pgtype.UUID

	if err := workspaceUUID.Scan(workspaceID); err != nil {
		return repository.Workspace{}, errors.New("invalid workspace ID")
	}

	if err := requesterUUID.Scan(requesterUserID); err != nil {
		return repository.Workspace{}, errors.New("invalid user ID")
	}

	// Begin transaction.
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return repository.Workspace{}, fmt.Errorf(
			"begin transaction: %w", err,
		)
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	qtx := s.queries.WithTx(tx)

	// Lock workspace.
	workspace, err := qtx.GetWorkspaceByIDForUpdate(
		ctx,
		workspaceUUID,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return repository.Workspace{}, errors.New("workspace not found")
	}

	if err != nil {
		return repository.Workspace{}, fmt.Errorf(
			"get workspace: %w", err,
		)
	}

	if workspace.DeletedAt.Valid {
		return repository.Workspace{}, errors.New("workspace already deleted")
	}

	// Verify that requester is the current owner.
	if !workspace.OwnerUserID.Valid ||
		workspace.OwnerUserID.Bytes != requesterUUID.Bytes {
		return repository.Workspace{}, errors.New(
			"only workspace owner can delete workspace",
		)
	}

	// Verify owner membership.
	role, err := qtx.GetWorkspaceMemberRoleForUpdate(
		ctx,
		repository.GetWorkspaceMemberRoleForUpdateParams{
			WorkspaceID: workspaceUUID,
			UserID:      requesterUUID,
		},
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return repository.Workspace{}, errors.New(
			"workspace owner membership is missing",
		)
	}

	if err != nil {
		return repository.Workspace{}, fmt.Errorf(
			"verify owner membership: %w", err,
		)
	}

	if role != "owner" {
		return repository.Workspace{}, errors.New(
			"workspace ownership is inconsistent",
		)
	}
	// Cancel all pending invitations before deleting workspace.
	if err := qtx.CancelPendingWorkspaceInvitations(
		ctx,
		workspaceUUID,
	); err != nil {
		return repository.Workspace{}, fmt.Errorf(
			"cancel pending workspace invitations: %w",
			err,
		)
	}

	// Soft-delete workspace.
	deletedWorkspace, err := qtx.SoftDeleteWorkspace(
		ctx,
		workspaceUUID,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return repository.Workspace{}, errors.New("workspace already deleted")
	}

	if err != nil {
		return repository.Workspace{}, fmt.Errorf(
			"soft delete workspace: %w", err,
		)
	}

	// Commit transaction.
	if err := tx.Commit(ctx); err != nil {
		return repository.Workspace{}, fmt.Errorf(
			"commit workspace deletion: %w", err,
		)
	}

	return deletedWorkspace, nil
}

func (s *Service) RemoveMember(
	ctx context.Context,
	workspaceID string,
	requesterUserID string,
	targetUserID string,
) (repository.WorkspaceMember, error) {

	var workspaceUUID pgtype.UUID
	var requesterUUID pgtype.UUID
	var targetUUID pgtype.UUID

	if err := workspaceUUID.Scan(workspaceID); err != nil {
		return repository.WorkspaceMember{}, errors.New("invalid workspace ID")
	}

	if err := requesterUUID.Scan(requesterUserID); err != nil {
		return repository.WorkspaceMember{}, errors.New("invalid user ID")
	}

	if err := targetUUID.Scan(targetUserID); err != nil {
		return repository.WorkspaceMember{}, errors.New("invalid target user ID")
	}

	if requesterUUID.Bytes == targetUUID.Bytes {
		return repository.WorkspaceMember{}, errors.New("cannot remove yourself")
	}

	// Begin transaction.
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return repository.WorkspaceMember{}, fmt.Errorf("begin transaction: %w", err)
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	qtx := s.queries.WithTx(tx)

	// Lock workspace first.
	workspace, err := qtx.GetWorkspaceByIDForUpdate(ctx, workspaceUUID)

	if errors.Is(err, pgx.ErrNoRows) {
		return repository.WorkspaceMember{}, errors.New("workspace not found")
	}

	if err != nil {
		return repository.WorkspaceMember{}, fmt.Errorf("get workspace: %w", err)
	}

	if workspace.DeletedAt.Valid {
		return repository.WorkspaceMember{}, errors.New("workspace is deleted")
	}

	// Lock requester membership.
	requesterRole, err := qtx.GetWorkspaceMemberRoleForUpdate(
		ctx,
		repository.GetWorkspaceMemberRoleForUpdateParams{
			WorkspaceID: workspaceUUID,
			UserID:      requesterUUID,
		},
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return repository.WorkspaceMember{}, errors.New("user is not a workspace member")
	}

	if err != nil {
		return repository.WorkspaceMember{}, fmt.Errorf("get requester role: %w", err)
	}

	// Verify owner consistency.
	if requesterRole == "owner" &&
		(!workspace.OwnerUserID.Valid ||
			workspace.OwnerUserID.Bytes != requesterUUID.Bytes) {
		return repository.WorkspaceMember{}, errors.New("workspace ownership is inconsistent")
	}

	// Lock target membership.
	targetRole, err := qtx.GetWorkspaceMemberRoleForUpdate(
		ctx,
		repository.GetWorkspaceMemberRoleForUpdateParams{
			WorkspaceID: workspaceUUID,
			UserID:      targetUUID,
		},
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return repository.WorkspaceMember{}, errors.New("target user is not a workspace member")
	}

	if err != nil {
		return repository.WorkspaceMember{}, fmt.Errorf("get target role: %w", err)
	}

	if targetRole == "owner" {
		return repository.WorkspaceMember{}, errors.New("cannot remove workspace owner")
	}

	// Check permissions.
	switch requesterRole {
	case "owner":
		// Owner can remove admin, member and viewer.

	case "admin":
		if targetRole != "member" && targetRole != "viewer" {
			return repository.WorkspaceMember{}, errors.New("admin cannot remove this member")
		}

	default:
		return repository.WorkspaceMember{}, errors.New(
			"you do not have permission to remove members",
		)
	}

	// Remove member.
	removedMember, err := qtx.RemoveWorkspaceMember(
		ctx,
		repository.RemoveWorkspaceMemberParams{
			WorkspaceID: workspaceUUID,
			UserID:      targetUUID,
		},
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return repository.WorkspaceMember{}, errors.New("target user is not a workspace member")
		}
		return repository.WorkspaceMember{}, fmt.Errorf("remove workspace member: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return repository.WorkspaceMember{}, fmt.Errorf("commit member removal: %w", err)
	}

	return removedMember, nil
}

func (s *Service) LeaveWorkspace(
	ctx context.Context,
	workspaceID string,
	userID string,
) (repository.WorkspaceMember, error) {

	var workspaceUUID pgtype.UUID
	var userUUID pgtype.UUID

	if err := workspaceUUID.Scan(workspaceID); err != nil {
		return repository.WorkspaceMember{}, errors.New("invalid workspace ID")
	}

	if err := userUUID.Scan(userID); err != nil {
		return repository.WorkspaceMember{}, errors.New("invalid user ID")
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return repository.WorkspaceMember{}, fmt.Errorf(
			"begin transaction: %w", err,
		)
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	qtx := s.queries.WithTx(tx)

	// Lock workspace first.
	workspace, err := qtx.GetWorkspaceByIDForUpdate(ctx, workspaceUUID)

	if errors.Is(err, pgx.ErrNoRows) {
		return repository.WorkspaceMember{}, errors.New("workspace not found")
	}

	if err != nil {
		return repository.WorkspaceMember{}, fmt.Errorf(
			"get workspace: %w", err,
		)
	}

	if workspace.DeletedAt.Valid {
		return repository.WorkspaceMember{}, errors.New("workspace is deleted")
	}

	// Lock current user's membership.
	role, err := qtx.GetWorkspaceMemberRoleForUpdate(
		ctx,
		repository.GetWorkspaceMemberRoleForUpdateParams{
			WorkspaceID: workspaceUUID,
			UserID:      userUUID,
		},
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return repository.WorkspaceMember{}, errors.New(
			"user is not a workspace member",
		)
	}

	if err != nil {
		return repository.WorkspaceMember{}, fmt.Errorf(
			"get workspace member role: %w", err,
		)
	}

	isWorkspaceOwner := workspace.OwnerUserID.Valid &&
		workspace.OwnerUserID.Bytes == userUUID.Bytes

	// Ensure owner_user_id and membership role are consistent.
	if (role == "owner") != isWorkspaceOwner {
		return repository.WorkspaceMember{}, errors.New(
			"workspace ownership is inconsistent",
		)
	}

	// Owner must transfer ownership before leaving.
	if role == "owner" {
		return repository.WorkspaceMember{}, errors.New(
			"workspace owner cannot leave before transferring ownership",
		)
	}

	// Remove current membership.
	member, err := qtx.RemoveWorkspaceMember(
		ctx,
		repository.RemoveWorkspaceMemberParams{
			WorkspaceID: workspaceUUID,
			UserID:      userUUID,
		},
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return repository.WorkspaceMember{}, errors.New(
			"user is not a workspace member",
		)
	}

	if err != nil {
		return repository.WorkspaceMember{}, fmt.Errorf(
			"leave workspace: %w", err,
		)
	}

	if err := tx.Commit(ctx); err != nil {
		return repository.WorkspaceMember{}, fmt.Errorf(
			"commit leave workspace: %w", err,
		)
	}

	return member, nil
}

func (s *Service) ChangeMemberRole(
	ctx context.Context,
	workspaceID string,
	requesterUserID string,
	targetUserID string,
	newRole string,
) (repository.WorkspaceMember, error) {

	newRole = strings.TrimSpace(newRole)

	if newRole != "admin" &&
		newRole != "member" &&
		newRole != "viewer" {
		return repository.WorkspaceMember{}, errors.New("invalid member role")
	}

	var workspaceUUID pgtype.UUID
	var requesterUUID pgtype.UUID
	var targetUUID pgtype.UUID

	if err := workspaceUUID.Scan(workspaceID); err != nil {
		return repository.WorkspaceMember{}, errors.New("invalid workspace ID")
	}

	if err := requesterUUID.Scan(requesterUserID); err != nil {
		return repository.WorkspaceMember{}, errors.New("invalid user ID")
	}

	if err := targetUUID.Scan(targetUserID); err != nil {
		return repository.WorkspaceMember{}, errors.New("invalid target user ID")
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return repository.WorkspaceMember{}, fmt.Errorf(
			"begin transaction: %w", err,
		)
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	qtx := s.queries.WithTx(tx)

	// 1. Lock workspace first.
	workspace, err := qtx.GetWorkspaceByIDForUpdate(
		ctx,
		workspaceUUID,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return repository.WorkspaceMember{}, errors.New("workspace not found")
	}

	if err != nil {
		return repository.WorkspaceMember{}, fmt.Errorf(
			"get workspace: %w", err,
		)
	}

	if workspace.DeletedAt.Valid {
		return repository.WorkspaceMember{}, errors.New("workspace is deleted")
	}

	// 2. Lock requester membership.
	requesterRole, err := qtx.GetWorkspaceMemberRoleForUpdate(
		ctx,
		repository.GetWorkspaceMemberRoleForUpdateParams{
			WorkspaceID: workspaceUUID,
			UserID:      requesterUUID,
		},
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return repository.WorkspaceMember{}, errors.New(
			"user is not a workspace member",
		)
	}

	if err != nil {
		return repository.WorkspaceMember{}, fmt.Errorf(
			"get requester role: %w", err,
		)
	}

	requesterIsOwner := workspace.OwnerUserID.Valid &&
		workspace.OwnerUserID.Bytes == requesterUUID.Bytes

	if (requesterRole == "owner") != requesterIsOwner {
		return repository.WorkspaceMember{}, errors.New(
			"workspace ownership is inconsistent",
		)
	}

	// 3. Lock target membership.
	targetRole, err := qtx.GetWorkspaceMemberRoleForUpdate(
		ctx,
		repository.GetWorkspaceMemberRoleForUpdateParams{
			WorkspaceID: workspaceUUID,
			UserID:      targetUUID,
		},
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return repository.WorkspaceMember{}, errors.New(
			"target user is not a workspace member",
		)
	}

	if err != nil {
		return repository.WorkspaceMember{}, fmt.Errorf(
			"get target role: %w", err,
		)
	}

	targetIsOwner := workspace.OwnerUserID.Valid &&
		workspace.OwnerUserID.Bytes == targetUUID.Bytes

	if (targetRole == "owner") != targetIsOwner {
		return repository.WorkspaceMember{}, errors.New(
			"workspace ownership is inconsistent",
		)
	}

	if targetRole == "owner" {
		return repository.WorkspaceMember{}, errors.New(
			"cannot change workspace owner role",
		)
	}

	// 4. Check permissions.
	switch requesterRole {
	case "owner":
		// Owner can assign admin, member and viewer.

	case "admin":
		if (targetRole != "member" && targetRole != "viewer") ||
			(newRole != "member" && newRole != "viewer") {
			return repository.WorkspaceMember{}, errors.New(
				"admin cannot change this member role",
			)
		}

	default:
		return repository.WorkspaceMember{}, errors.New(
			"you do not have permission to change member roles",
		)
	}

	// 5. Update target membership.
	updatedMember, err := qtx.UpdateWorkspaceMemberRole(
		ctx,
		repository.UpdateWorkspaceMemberRoleParams{
			Role:        newRole,
			WorkspaceID: workspaceUUID,
			UserID:      targetUUID,
		},
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return repository.WorkspaceMember{}, errors.New(
			"target user is not a workspace member",
		)
	}

	if err != nil {
		return repository.WorkspaceMember{}, fmt.Errorf(
			"update workspace member role: %w", err,
		)
	}

	// 6. Commit.
	if err := tx.Commit(ctx); err != nil {
		return repository.WorkspaceMember{}, fmt.Errorf(
			"commit role update: %w", err,
		)
	}

	return updatedMember, nil
}

func (s *Service) CreateInvitation(
	ctx context.Context,
	workspaceID string,
	inviterUserID string,
	inviteeEmail string,
	role string,
) (repository.WorkspaceInvitation, error) {

	inviteeEmail = strings.TrimSpace(strings.ToLower(inviteeEmail))
	role = strings.TrimSpace(role)

	if inviteeEmail == "" {
		return repository.WorkspaceInvitation{}, errors.New("invitee email is required")
	}

	if role != "admin" && role != "member" && role != "viewer" {
		return repository.WorkspaceInvitation{}, errors.New("invalid invitation role")
	}

	var workspaceUUID pgtype.UUID
	var inviterUUID pgtype.UUID

	if err := workspaceUUID.Scan(workspaceID); err != nil {
		return repository.WorkspaceInvitation{}, errors.New("invalid workspace ID")
	}

	if err := inviterUUID.Scan(inviterUserID); err != nil {
		return repository.WorkspaceInvitation{}, errors.New("invalid inviter user ID")
	}

	// Check inviter permissions.
	memberRole, err := s.queries.GetWorkspaceMemberRole(
		ctx,
		repository.GetWorkspaceMemberRoleParams{
			WorkspaceID: workspaceUUID,
			UserID:      inviterUUID,
		},
	)
	if err != nil {
		return repository.WorkspaceInvitation{}, errors.New("user is not a workspace member")
	}

	if memberRole != "owner" && memberRole != "admin" {
		return repository.WorkspaceInvitation{}, errors.New("you do not have permission to invite members")
	}

	// Check whether the invited user already has an account.
	var inviteeUUID pgtype.UUID

	invitee, err := s.queries.GetUserByEmail(ctx, inviteeEmail)

	if err == nil {
		inviteeUUID = invitee.ID
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return repository.WorkspaceInvitation{}, fmt.Errorf(
			"find invited user: %w", err,
		)
	}

	// Check whether the invited user is already a member.
	if inviteeUUID.Valid {
		isMember, err := s.queries.IsWorkspaceMember(
			ctx,
			repository.IsWorkspaceMemberParams{
				WorkspaceID: workspaceUUID,
				UserID:      inviteeUUID,
			},
		)
		if err != nil {
			return repository.WorkspaceInvitation{}, fmt.Errorf(
				"check workspace membership: %w", err,
			)
		}

		if isMember {
			return repository.WorkspaceInvitation{}, errors.New(
				"user is already a workspace member",
			)
		}
	}

	// Check whether a pending invitation already exists.
	_, err = s.queries.GetWorkspaceInvitationByEmail(
		ctx,
		repository.GetWorkspaceInvitationByEmailParams{
			WorkspaceID: workspaceUUID,
			Email:       inviteeEmail,
		},
	)

	if err == nil {
		return repository.WorkspaceInvitation{}, errors.New(
			"a pending invitation already exists for this email",
		)
	}

	if !errors.Is(err, pgx.ErrNoRows) {
		return repository.WorkspaceInvitation{}, fmt.Errorf(
			"check existing invitation: %w", err,
		)
	}

	invitation, err := s.queries.CreateWorkspaceInvitation(
		ctx,
		repository.CreateWorkspaceInvitationParams{
			WorkspaceID:   workspaceUUID,
			InviterUserID: inviterUUID,
			InviteeUserID: inviteeUUID,
			Email:         pgtype.Text{String: inviteeEmail, Valid: true},
			Role:          role,
			ExpiresAt: pgtype.Timestamptz{
				Time:  time.Now().Add(7 * 24 * time.Hour),
				Valid: true,
			},
		},
	)
	if err != nil {
		return repository.WorkspaceInvitation{}, fmt.Errorf("create workspace invitation: %w", err)
	}

	return invitation, nil
}

func getInvitationForUser(
	ctx context.Context,
	q *repository.Queries,
	token string,
	userID string,
) (repository.WorkspaceInvitation, pgtype.UUID, error) {

	var tokenUUID pgtype.UUID
	if err := tokenUUID.Scan(token); err != nil {
		return repository.WorkspaceInvitation{}, pgtype.UUID{}, errors.New("invalid invitation token")
	}

	var userUUID pgtype.UUID
	if err := userUUID.Scan(userID); err != nil {
		return repository.WorkspaceInvitation{}, pgtype.UUID{}, errors.New("invalid user ID")
	}

	invitation, err := q.GetWorkspaceInvitationByToken(ctx, tokenUUID)
	if errors.Is(err, pgx.ErrNoRows) {
		return repository.WorkspaceInvitation{}, pgtype.UUID{}, errors.New("invitation not found")
	}
	if err != nil {
		return repository.WorkspaceInvitation{}, pgtype.UUID{}, fmt.Errorf("get invitation: %w", err)
	}

	if invitation.Status != "pending" {
		return repository.WorkspaceInvitation{}, pgtype.UUID{}, errors.New("invitation is not pending")
	}

	user, err := q.GetUserByID(ctx, userUUID)
	if err != nil {
		return repository.WorkspaceInvitation{}, pgtype.UUID{}, fmt.Errorf("get user: %w", err)
	}

	if !invitation.Email.Valid ||
		!strings.EqualFold(strings.TrimSpace(invitation.Email.String), strings.TrimSpace(user.Email)) {
		return repository.WorkspaceInvitation{}, pgtype.UUID{}, errors.New("invitation does not belong to this user")
	}

	if invitation.InviteeUserID.Valid &&
		invitation.InviteeUserID.Bytes != userUUID.Bytes {
		return repository.WorkspaceInvitation{}, pgtype.UUID{}, errors.New("invitation belongs to another user")
	}

	return invitation, userUUID, nil
}

func (s *Service) AcceptInvitation(
	ctx context.Context,
	token string,
	userID string,
) (repository.WorkspaceInvitation, error) {

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return repository.WorkspaceInvitation{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	qtx := s.queries.WithTx(tx)

	invitation, userUUID, err := getInvitationForUser(ctx, qtx, token, userID)
	if err != nil {
		return repository.WorkspaceInvitation{}, err
	}

	if !invitation.ExpiresAt.Valid {
		return repository.WorkspaceInvitation{}, errors.New("invitation expiry is invalid")
	}

	if !time.Now().Before(invitation.ExpiresAt.Time) {
		_, err := qtx.ExpireWorkspaceInvitation(ctx, invitation.ID)
		if err != nil {
			return repository.WorkspaceInvitation{}, fmt.Errorf("expire invitation: %w", err)
		}

		if err := tx.Commit(ctx); err != nil {
			return repository.WorkspaceInvitation{}, fmt.Errorf("commit expired invitation: %w", err)
		}

		return repository.WorkspaceInvitation{}, errors.New("invitation has expired")
	}

	isMember, err := qtx.IsWorkspaceMember(ctx, repository.IsWorkspaceMemberParams{
		WorkspaceID: invitation.WorkspaceID,
		UserID:      userUUID,
	})
	if err != nil {
		return repository.WorkspaceInvitation{}, fmt.Errorf("check workspace membership: %w", err)
	}

	if isMember {
		return repository.WorkspaceInvitation{}, errors.New("user is already a workspace member")
	}

	_, err = qtx.AddWorkspaceMember(ctx, repository.AddWorkspaceMemberParams{
		WorkspaceID: invitation.WorkspaceID,
		UserID:      userUUID,
		Role:        invitation.Role,
	})
	if err != nil {
		return repository.WorkspaceInvitation{}, fmt.Errorf("add workspace member: %w", err)
	}

	accepted, err := qtx.AcceptWorkspaceInvitation(ctx, repository.AcceptWorkspaceInvitationParams{
		InviteeUserID: userUUID,
		ID:            invitation.ID,
	})
	if err != nil {
		return repository.WorkspaceInvitation{}, fmt.Errorf("accept invitation: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return repository.WorkspaceInvitation{}, fmt.Errorf("commit invitation acceptance: %w", err)
	}

	return accepted, nil
}

func (s *Service) RejectInvitation(
	ctx context.Context,
	token string,
	userID string,
) (repository.WorkspaceInvitation, error) {

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return repository.WorkspaceInvitation{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	qtx := s.queries.WithTx(tx)

	invitation, _, err := getInvitationForUser(ctx, qtx, token, userID)
	if err != nil {
		return repository.WorkspaceInvitation{}, err
	}

	if !invitation.ExpiresAt.Valid {
		return repository.WorkspaceInvitation{}, errors.New("invitation expiry is invalid")
	}

	if !time.Now().Before(invitation.ExpiresAt.Time) {
		_, err := qtx.ExpireWorkspaceInvitation(ctx, invitation.ID)
		if err != nil {
			return repository.WorkspaceInvitation{}, fmt.Errorf("expire invitation: %w", err)
		}

		if err := tx.Commit(ctx); err != nil {
			return repository.WorkspaceInvitation{}, fmt.Errorf("commit expired invitation: %w", err)
		}

		return repository.WorkspaceInvitation{}, errors.New("invitation has expired")
	}

	rejected, err := qtx.RejectWorkspaceInvitation(ctx, invitation.ID)
	if err != nil {
		return repository.WorkspaceInvitation{}, fmt.Errorf("reject invitation: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return repository.WorkspaceInvitation{}, fmt.Errorf("commit invitation rejection: %w", err)
	}

	return rejected, nil
}

func (s *Service) ListWorkspaceInvitations(
	ctx context.Context,
	workspaceID string,
	userID string,
) ([]repository.WorkspaceInvitation, error) {

	var workspaceUUID pgtype.UUID
	var userUUID pgtype.UUID

	if err := workspaceUUID.Scan(workspaceID); err != nil {
		return nil, errors.New("invalid workspace ID")
	}

	if err := userUUID.Scan(userID); err != nil {
		return nil, errors.New("invalid user ID")
	}

	// Check the user's workspace role.
	memberRole, err := s.queries.GetWorkspaceMemberRole(
		ctx,
		repository.GetWorkspaceMemberRoleParams{
			WorkspaceID: workspaceUUID,
			UserID:      userUUID,
		},
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errors.New("user is not a workspace member")
	}

	if err != nil {
		return nil, fmt.Errorf("get workspace member role: %w", err)
	}

	if memberRole != "owner" && memberRole != "admin" {
		return nil, errors.New("you do not have permission to view invitations")
	}

	invitations, err := s.queries.ListWorkspaceInvitations(
		ctx,
		workspaceUUID,
	)
	if err != nil {
		return nil, fmt.Errorf("list workspace invitations: %w", err)
	}

	if invitations == nil {
		return []repository.WorkspaceInvitation{}, nil
	}

	return invitations, nil
}

func (s *Service) CancelInvitation(
	ctx context.Context,
	workspaceID string,
	invitationID string,
	userID string,
) (repository.WorkspaceInvitation, error) {

	var workspaceUUID pgtype.UUID
	var invitationUUID pgtype.UUID
	var userUUID pgtype.UUID

	if err := workspaceUUID.Scan(workspaceID); err != nil {
		return repository.WorkspaceInvitation{}, errors.New("invalid workspace ID")
	}

	if err := invitationUUID.Scan(invitationID); err != nil {
		return repository.WorkspaceInvitation{}, errors.New("invalid invitation ID")
	}

	if err := userUUID.Scan(userID); err != nil {
		return repository.WorkspaceInvitation{}, errors.New("invalid user ID")
	}

	// Check inviter permissions.
	memberRole, err := s.queries.GetWorkspaceMemberRole(
		ctx,
		repository.GetWorkspaceMemberRoleParams{
			WorkspaceID: workspaceUUID,
			UserID:      userUUID,
		},
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return repository.WorkspaceInvitation{}, errors.New("user is not a workspace member")
	}

	if err != nil {
		return repository.WorkspaceInvitation{}, fmt.Errorf(
			"get workspace member role: %w", err,
		)
	}

	if memberRole != "owner" && memberRole != "admin" {
		return repository.WorkspaceInvitation{}, errors.New(
			"you do not have permission to cancel invitations",
		)
	}

	// Cancel only a pending invitation belonging to this workspace.
	invitation, err := s.queries.CancelWorkspaceInvitation(
		ctx,
		repository.CancelWorkspaceInvitationParams{
			ID:          invitationUUID,
			WorkspaceID: workspaceUUID,
		},
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return repository.WorkspaceInvitation{}, errors.New(
			"invitation not found or not pending",
		)
	}

	if err != nil {
		return repository.WorkspaceInvitation{}, fmt.Errorf(
			"cancel workspace invitation: %w", err,
		)
	}

	return invitation, nil
}
