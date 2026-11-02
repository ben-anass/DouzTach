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

	// Prevent removing yourself.
	if requesterUUID.Bytes == targetUUID.Bytes {
		return repository.WorkspaceMember{}, errors.New(
			"cannot remove yourself",
		)
	}

	// Get requester's role.
	requesterRole, err := s.queries.GetWorkspaceMemberRole(
		ctx,
		repository.GetWorkspaceMemberRoleParams{
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

	// Get target member's role.
	targetRole, err := s.queries.GetWorkspaceMemberRole(
		ctx,
		repository.GetWorkspaceMemberRoleParams{
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
			"get target member role: %w", err,
		)
	}

	// Never remove the workspace owner.
	if targetRole == "owner" {
		return repository.WorkspaceMember{}, errors.New(
			"cannot remove workspace owner",
		)
	}

	// Check requester permissions.
	switch requesterRole {
	case "owner":
		// Owner can remove admin, member and viewer.

	case "admin":
		if targetRole != "member" && targetRole != "viewer" {
			return repository.WorkspaceMember{}, errors.New(
				"admin cannot remove this member",
			)
		}

	default:
		return repository.WorkspaceMember{}, errors.New(
			"you do not have permission to remove members",
		)
	}

	// Remove target member.
	removedMember, err := s.queries.RemoveWorkspaceMember(
		ctx,
		repository.RemoveWorkspaceMemberParams{
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
			"remove workspace member: %w", err,
		)
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

	// Workspace owner must transfer ownership first.
	if role == "owner" {
		return repository.WorkspaceMember{}, errors.New(
			"workspace owner cannot leave before transferring ownership",
		)
	}

	// Remove current user's membership.
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

	// Lock and get requester's role.
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

	// Lock and get target member's role.
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

	// Never change the workspace owner's role.
	if targetRole == "owner" {
		return repository.WorkspaceMember{}, errors.New(
			"cannot change workspace owner role",
		)
	}

	// Check permissions.
	switch requesterRole {
	case "owner":
		// Owner can assign admin, member or viewer.

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

	// Update member role.
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
