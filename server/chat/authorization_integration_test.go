package chat_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ben-anass/DouzTach/internal/database"
	"github.com/ben-anass/DouzTach/internal/repository"
	"github.com/ben-anass/DouzTach/server/activity"
	"github.com/ben-anass/DouzTach/server/chat"
	"github.com/ben-anass/DouzTach/server/scrum"
	"github.com/ben-anass/DouzTach/server/workspaces"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/joho/godotenv"
)

func TestWorkspaceWritersOnlyPostgresIntegration(t *testing.T) {
	if os.Getenv("DOUZTACH_ROLE_E2E") != "1" {
		t.Skip("set DOUZTACH_ROLE_E2E=1 to run against an isolated PostgreSQL database")
	}
	if !strings.HasPrefix(os.Getenv("POSTGRES_DB"), "douztach_e2e_") {
		t.Fatal("role integration tests require an isolated POSTGRES_DB named douztach_e2e_*")
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
		Username: "roleowner" + suffix, Nickname: "Role Test Owner",
		Email: "role-owner-" + suffix + "@example.test", PasswordHash: "integration-test-only",
		FavoriteLanguage: pgtype.Text{String: "Go", Valid: true},
	})
	if err != nil {
		t.Fatalf("create test owner: %v", err)
	}
	viewer, err := q.CreateUser(ctx, repository.CreateUserParams{
		Username: "roleviewer" + suffix, Nickname: "Role Test Viewer",
		Email: "role-viewer-" + suffix + "@example.test", PasswordHash: "integration-test-only",
		FavoriteLanguage: pgtype.Text{String: "Go", Valid: true},
	})
	if err != nil {
		_, _ = pool.Exec(context.Background(), "DELETE FROM users WHERE id = $1", owner.ID)
		t.Fatalf("create test viewer: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM users WHERE id = $1", owner.ID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM users WHERE id = $1", viewer.ID)
	}()

	workspaceService := workspaces.NewService(q, pool)
	workspace, err := workspaceService.CreateWorkspace(ctx, owner.ID.String(), "Role test "+suffix, "role-"+suffix, "", "team")
	if err != nil {
		t.Fatalf("create test workspace: %v", err)
	}
	invitation, err := workspaceService.CreateInvitation(ctx, workspace.ID.String(), owner.ID.String(), viewer.Email, "viewer")
	if err != nil {
		t.Fatalf("invite test viewer: %v", err)
	}
	if _, err := workspaceService.AcceptInvitation(ctx, invitation.Token.String(), viewer.ID.String()); err != nil {
		t.Fatalf("accept viewer invitation: %v", err)
	}

	scrumService := scrum.NewService(pool)
	_, err = scrumService.CreateEpic(ctx, scrum.CreateEpicInput{
		WorkspaceID: workspace.ID,
		ActorUserID: viewer.ID,
		Title:       "Must not be created",
	})
	if !errors.Is(err, activity.ErrReadOnly) {
		t.Fatalf("viewer CreateEpic() error = %v, want read-only denial", err)
	}
	epics, err := scrumService.ListEpics(ctx, workspace.ID)
	if err != nil {
		t.Fatalf("list epics after denied write: %v", err)
	}
	if len(epics) != 0 {
		t.Fatalf("viewer write left %d epics behind", len(epics))
	}
	if _, err := scrumService.CreateEpic(ctx, scrum.CreateEpicInput{
		WorkspaceID: workspace.ID,
		ActorUserID: owner.ID,
		Title:       "Owner write",
	}); err != nil {
		t.Fatalf("owner CreateEpic(): %v", err)
	}

	chatService := chat.NewService(pool)
	channel, err := chatService.CreateChannel(ctx, workspace.ID, owner.ID, "general", "general")
	if err != nil {
		t.Fatalf("owner CreateChannel(): %v", err)
	}
	if _, err := chatService.CreateChannel(ctx, workspace.ID, viewer.ID, "viewer-created", "custom"); !errors.Is(err, activity.ErrReadOnly) {
		t.Fatalf("viewer CreateChannel() error = %v, want read-only denial", err)
	}
	if _, err := chatService.PostMessage(ctx, workspace.ID, channel.ID, viewer.ID, "must not be posted"); !errors.Is(err, activity.ErrReadOnly) {
		t.Fatalf("viewer PostMessage() error = %v, want read-only denial", err)
	}
	channels, err := chatService.ListChannels(ctx, workspace.ID, viewer.ID)
	if err != nil {
		t.Fatalf("viewer ListChannels(): %v", err)
	}
	if len(channels) != 1 {
		t.Fatalf("viewer sees %d channels, want 1", len(channels))
	}
	messages, err := chatService.ListMessages(ctx, workspace.ID, channel.ID, viewer.ID, 10, 0)
	if err != nil {
		t.Fatalf("viewer ListMessages(): %v", err)
	}
	if len(messages) != 0 {
		t.Fatalf("viewer writes left %d messages behind", len(messages))
	}
	if _, err := chatService.PostMessage(ctx, workspace.ID, channel.ID, owner.ID, "owner message"); err != nil {
		t.Fatalf("owner PostMessage(): %v", err)
	}
}
