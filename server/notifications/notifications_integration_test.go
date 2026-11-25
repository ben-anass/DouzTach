package notifications_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ben-anass/DouzTach/internal/database"
	"github.com/ben-anass/DouzTach/internal/repository"
	"github.com/ben-anass/DouzTach/server/notifications"
	"github.com/ben-anass/DouzTach/server/scrum"
	"github.com/ben-anass/DouzTach/server/workspaces"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/joho/godotenv"
)

func TestNotificationTriggersPostgresIntegration(t *testing.T) {
	if os.Getenv("DOUZTACH_NOTIFICATION_E2E") != "1" {
		t.Skip("set DOUZTACH_NOTIFICATION_E2E=1 to run against the isolated PostgreSQL test database")
	}
	if !strings.HasPrefix(os.Getenv("POSTGRES_DB"), "douztach_e2e_") {
		t.Fatal("notification integration tests require an isolated POSTGRES_DB named douztach_e2e_*")
	}
	if err := godotenv.Load(filepath.Join("..", "..", ".env")); err != nil && os.Getenv("POSTGRES_USER") == "" {
		t.Fatalf("load test database environment: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := database.NewPool(ctx)
	if err != nil {
		t.Fatalf("connect isolated database: %v", err)
	}
	defer pool.Close()
	q := repository.New(pool)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	owner, err := q.CreateUser(ctx, repository.CreateUserParams{
		Username: "notifowner" + suffix, Nickname: "Notification Owner",
		Email: "notification-owner-" + suffix + "@example.test", PasswordHash: "e2e-unused",
		FavoriteLanguage: pgtype.Text{String: "Go", Valid: true},
	})
	if err != nil {
		t.Fatalf("create test owner: %v", err)
	}
	member, err := q.CreateUser(ctx, repository.CreateUserParams{
		Username: "notifmember" + suffix, Nickname: "Notification Member",
		Email: "notification-member-" + suffix + "@example.test", PasswordHash: "e2e-unused",
		FavoriteLanguage: pgtype.Text{String: "Go", Valid: true},
	})
	if err != nil {
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", owner.ID)
		t.Fatalf("create test member: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM users WHERE id = $1", owner.ID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM users WHERE id = $1", member.ID)
	}()

	workspaceService := workspaces.NewService(q, pool)
	workspace, err := workspaceService.CreateWorkspace(ctx, owner.ID.String(), "Notification workspace "+suffix, "notif-"+suffix, "", "team")
	if err != nil {
		t.Fatalf("create test workspace: %v", err)
	}
	notificationService := notifications.NewService(pool)

	invitation, err := workspaceService.CreateInvitation(ctx, workspace.ID.String(), owner.ID.String(), member.Email, "member")
	if err != nil {
		t.Fatalf("create invitation: %v", err)
	}
	memberNotifications, err := notificationService.ListForUser(ctx, member.ID, 100, 0)
	if err != nil {
		t.Fatalf("list invitee notifications: %v", err)
	}
	assertNotificationType(t, memberNotifications, "workspace_invitation")

	if _, err := workspaceService.CreateInvitation(ctx, workspace.ID.String(), owner.ID.String(), member.Email, "member"); err == nil {
		t.Fatal("duplicate pending invitation unexpectedly succeeded")
	}
	memberNotifications, err = notificationService.ListForUser(ctx, member.ID, 100, 0)
	if err != nil {
		t.Fatalf("list notifications after duplicate invitation: %v", err)
	}
	if countNotificationType(memberNotifications, "workspace_invitation") != 1 {
		t.Fatal("duplicate invitation produced another notification")
	}

	if _, err := workspaceService.AcceptInvitation(ctx, invitation.Token.String(), member.ID.String()); err != nil {
		t.Fatalf("accept invitation: %v", err)
	}
	ownerNotifications, err := notificationService.ListForUser(ctx, owner.ID, 100, 0)
	if err != nil {
		t.Fatalf("list inviter notifications: %v", err)
	}
	assertNotificationType(t, ownerNotifications, "workspace_member_added")

	if _, err := workspaceService.ChangeMemberRole(ctx, workspace.ID.String(), owner.ID.String(), member.ID.String(), "admin"); err != nil {
		t.Fatalf("change member role: %v", err)
	}
	memberNotifications, err = notificationService.ListForUser(ctx, member.ID, 100, 0)
	if err != nil {
		t.Fatalf("list role-change notification: %v", err)
	}
	assertNotificationType(t, memberNotifications, "workspace_role_changed")

	scrumService := scrum.NewService(pool)
	assigneeID := member.ID
	item, err := scrumService.CreateWorkItem(ctx, scrum.CreateWorkItemInput{
		WorkspaceID: workspace.ID,
		ActorUserID: owner.ID,
		Title:       "Notification assignment " + suffix,
		ItemType:    "task",
		AssigneeID:  assigneeID,
	})
	if err != nil {
		t.Fatalf("create assigned task: %v", err)
	}
	memberNotifications, err = notificationService.ListForUser(ctx, member.ID, 100, 0)
	if err != nil {
		t.Fatalf("list task-assignment notification: %v", err)
	}
	assertNotificationType(t, memberNotifications, "task_assigned")
	initialTaskAssignmentCount := countNotificationType(memberNotifications, "task_assigned")

	_, err = scrumService.UpdateWorkItem(ctx, scrum.UpdateWorkItemInput{
		WorkItemID: item.ID,
		CreateWorkItemInput: scrum.CreateWorkItemInput{
			WorkspaceID: workspace.ID,
			ActorUserID: owner.ID,
			Title:       item.Title,
			Description: item.Description.String,
			ItemType:    item.ItemType,
			Status:      "in_progress",
			Priority:    item.Priority,
			AssigneeID:  assigneeID,
			SprintID:    item.SprintID,
			DueDate:     item.DueDate,
			StoryPoints: item.StoryPoints,
			UserStoryID: item.UserStoryID,
			ParentID:    item.ParentID,
		},
	})
	if err != nil {
		t.Fatalf("update task without changing assignee: %v", err)
	}
	memberNotifications, err = notificationService.ListForUser(ctx, member.ID, 100, 0)
	if err != nil {
		t.Fatalf("list notifications after status update: %v", err)
	}
	if countNotificationType(memberNotifications, "task_assigned") != initialTaskAssignmentCount {
		t.Fatal("unchanged task assignment created a duplicate notification")
	}

	story, err := scrumService.CreateUserStory(ctx, scrum.CreateUserStoryInput{
		WorkspaceID: workspace.ID,
		ActorUserID: owner.ID,
		Title:       "Notification story assignment " + suffix,
		Status:      "to_do",
		Priority:    "medium",
		AssigneeID:  assigneeID,
	})
	if err != nil {
		t.Fatalf("create assigned user story: %v", err)
	}
	memberNotifications, err = notificationService.ListForUser(ctx, member.ID, 100, 0)
	if err != nil {
		t.Fatalf("list user-story assignment notification: %v", err)
	}
	assertNotificationType(t, memberNotifications, "user_story_assigned")

	var unauthorizedID pgtype.UUID
	if err := unauthorizedID.Scan("00000000-0000-0000-0000-000000000001"); err != nil {
		t.Fatal(err)
	}
	if _, err := notifications.CreateInTransaction(ctx, q, notifications.CreateInput{
		RecipientUserID: unauthorizedID,
		WorkspaceID:     workspace.ID,
		EventType:       "workspace_invitation",
		RelatedEntityID: story.ID,
		Message:         "must not cross workspace boundaries",
	}); err == nil {
		t.Fatal("non-member notification recipient unexpectedly passed workspace authorization")
	}

	var unread repository.Notification
	for _, item := range memberNotifications {
		if item.EventType == "task_assigned" && item.ReadAt.Valid == false {
			unread = item
			break
		}
	}
	if !unread.ID.Valid {
		t.Fatal("expected an unread task-assignment notification")
	}
	if _, err := notificationService.MarkRead(ctx, member.ID, unread.ID); err != nil {
		t.Fatalf("mark task notification read: %v", err)
	}
	persisted, err := notificationService.ListForUser(ctx, member.ID, 100, 0)
	if err != nil {
		t.Fatalf("reload persisted notifications: %v", err)
	}
	if !notificationIsRead(persisted, unread.ID) {
		t.Fatal("notification read timestamp was not persisted")
	}
}

func assertNotificationType(t *testing.T, items []repository.Notification, eventType string) {
	t.Helper()
	if countNotificationType(items, eventType) == 0 {
		t.Fatalf("no %q notification found", eventType)
	}
}

func countNotificationType(items []repository.Notification, eventType string) int {
	count := 0
	for _, item := range items {
		if item.EventType == eventType {
			count++
		}
	}
	return count
}

func notificationIsRead(items []repository.Notification, id pgtype.UUID) bool {
	for _, item := range items {
		if item.ID == id {
			return item.ReadAt.Valid
		}
	}
	return false
}
