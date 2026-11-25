package commands_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/ben-anass/DouzTach/internal/database"
	"github.com/joho/godotenv"
)

func TestCLIRealWorkflowPostgresIntegration(t *testing.T) {
	if os.Getenv("DOUZTACH_CLI_E2E") != "1" {
		t.Skip("set DOUZTACH_CLI_E2E=1 to run the real CLI against an isolated API and PostgreSQL database")
	}

	apiURL := strings.TrimRight(os.Getenv("DOUZTACH_API_URL"), "/")
	if apiURL == "" {
		t.Fatal("DOUZTACH_API_URL must point to the running E2E API server")
	}
	if !strings.HasPrefix(os.Getenv("POSTGRES_DB"), "douztach_e2e_") {
		t.Fatal("CLI integration tests require an isolated POSTGRES_DB named douztach_e2e_*")
	}

	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	if err := godotenv.Load(filepath.Join(root, ".env")); err != nil && os.Getenv("POSTGRES_USER") == "" {
		t.Fatalf("load test database environment: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	pool, err := database.NewPool(ctx)
	if err != nil {
		t.Fatalf("connect isolated E2E database: %v", err)
	}
	defer pool.Close()

	binary := filepath.Join(t.TempDir(), "douztech")
	build := exec.CommandContext(ctx, "go", "build", "-o", binary, "./cmd/client")
	build.Dir = root
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build actual CLI executable: %v\n%s", err, output)
	}

	suffix := uniqueCLISuffix(t)
	ownerEmail := "cli-owner-" + suffix + "@example.test"
	memberEmail := "cli-member-" + suffix + "@example.test"
	defer func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM users WHERE email = $1", ownerEmail)
		_, _ = pool.Exec(context.Background(), "DELETE FROM users WHERE email = $1", memberEmail)
	}()
	ownerName := "CLI Owner " + suffix
	memberName := "CLI Member " + suffix
	password := "E2E!Password2026"
	ownerConfig := filepath.Join(t.TempDir(), "owner-config")
	memberConfig := filepath.Join(t.TempDir(), "member-config")

	run := func(configDir string, stdin string, args ...string) string {
		t.Helper()
		baseArgs := []string{"--api-url", apiURL, "--config-dir", configDir, "--no-color"}
		cmd := exec.CommandContext(ctx, binary, append(baseArgs, args...)...)
		cmd.Stdin = strings.NewReader(stdin)
		var output bytes.Buffer
		cmd.Stdout = &output
		cmd.Stderr = &output
		err := cmd.Run()
		if err != nil {
			t.Fatalf("CLI %q failed: %v\n%s", strings.Join(args, " "), err, output.String())
		}
		t.Logf("exit 0: douztech %s\n%s", strings.Join(args, " "), strings.TrimSpace(output.String()))
		return output.String()
	}
	runFailure := func(configDir string, args ...string) string {
		t.Helper()
		baseArgs := []string{"--api-url", apiURL, "--config-dir", configDir, "--no-color"}
		cmd := exec.CommandContext(ctx, binary, append(baseArgs, args...)...)
		var output bytes.Buffer
		cmd.Stdout = &output
		cmd.Stderr = &output
		err := cmd.Run()
		if err == nil {
			t.Fatalf("CLI %q unexpectedly succeeded\n%s", strings.Join(args, " "), output.String())
		}
		t.Logf("expected failure: douztech %s -> %v\n%s", strings.Join(args, " "), err, strings.TrimSpace(output.String()))
		return output.String()
	}

	ownerOutput := run(ownerConfig, password+"\n", "register", "--name", ownerName, "--email", ownerEmail, "--password-stdin")
	requireOutput(t, ownerOutput, "Account created")
	run(ownerConfig, "", "profile")
	run(ownerConfig, "", "logout", "--yes")
	run(ownerConfig, password+"\n", "login", "--email", ownerEmail, "--password-stdin")
	run(ownerConfig, "", "profile")

	workspaceOutput := run(ownerConfig, "", "workspace", "create", "--name", "CLI E2E "+suffix)
	workspaceID := firstUUID(t, workspaceOutput, "workspace")
	run(ownerConfig, "", "workspace", "list")

	memberOutput := run(memberConfig, password+"\n", "register", "--name", memberName, "--email", memberEmail, "--password-stdin")
	requireOutput(t, memberOutput, "Account created")
	memberProfile := run(memberConfig, "", "profile")
	memberID := firstUUID(t, memberProfile, "member profile")

	invitationOutput := run(ownerConfig, "", "workspace", "invite", "--email", memberEmail, "--role", "member")
	invitationToken := firstUUID(t, invitationOutput, "workspace invitation")
	run(memberConfig, "", "workspace", "accept-invitation", invitationToken)
	run(memberConfig, "", "workspace", "switch", workspaceID)

	epicOutput := run(ownerConfig, "", "epic", "create", "--title", "CLI Epic "+suffix)
	epicID := firstUUID(t, epicOutput, "epic")
	run(ownerConfig, "", "epic", "list")
	run(ownerConfig, "", "epic", "show", epicID)
	run(ownerConfig, "", "epic", "update", epicID, "--title", "CLI Epic Updated "+suffix)

	storyOutput := run(ownerConfig, "", "story", "create", "--title", "CLI Story "+suffix, "--epic", epicID, "--assignee", memberID, "--points", "5")
	storyID := firstUUID(t, storyOutput, "user story")
	run(ownerConfig, "", "story", "list")
	storyShowOutput := run(ownerConfig, "", "story", "show", storyID)
	requireOutput(t, storyShowOutput, "Story points          5")
	run(ownerConfig, "", "story", "update", storyID, "--title", "CLI Story Updated "+suffix, "--points", "8")
	storyShowOutput = run(ownerConfig, "", "story", "show", storyID)
	requireOutput(t, storyShowOutput, "Story points          8")
	secondStoryOutput := run(ownerConfig, "", "story", "create", "--title", "CLI Story Two "+suffix, "--epic", epicID, "--points", "3")
	secondStoryID := firstUUID(t, secondStoryOutput, "second user story")
	run(ownerConfig, "", "sprint", "reorder-story", secondStoryID, "--position", "1")
	var backlogPosition int32
	if err := pool.QueryRow(ctx, "SELECT backlog_position FROM user_stories WHERE id = $1", secondStoryID).Scan(&backlogPosition); err != nil {
		t.Fatalf("verify persisted backlog order: %v", err)
	}
	if backlogPosition != 1 {
		t.Fatalf("persisted backlog position = %d, want 1", backlogPosition)
	}
	backlogOutput := run(ownerConfig, "", "sprint", "backlog")
	requireOrderedOutput(t, backlogOutput, "CLI Story Updated "+suffix, "CLI Story Two "+suffix)

	taskOutput := run(ownerConfig, "", "task", "create", "--title", "CLI Task "+suffix, "--assignee", memberID, "--points", "2")
	requireOutput(t, taskOutput, "CLI Task "+suffix)
	var taskID string
	if err := pool.QueryRow(ctx, "SELECT id::text FROM work_items WHERE workspace_id = $1 AND title = $2", workspaceID, "CLI Task "+suffix).Scan(&taskID); err != nil {
		t.Fatalf("find persisted CLI-created task: %v", err)
	}
	run(ownerConfig, "", "task", "show", taskID)
	run(ownerConfig, "", "task", "update", taskID, "--status", "IN_PROGRESS")
	var taskStatus string
	if err := pool.QueryRow(ctx, "SELECT status FROM work_items WHERE id = $1", taskID).Scan(&taskStatus); err != nil {
		t.Fatalf("verify persisted task status: %v", err)
	}
	if taskStatus != "in_progress" {
		t.Fatalf("persisted task status = %q, want in_progress", taskStatus)
	}
	var taskPoints int32
	if err := pool.QueryRow(ctx, "SELECT story_points FROM work_items WHERE id = $1", taskID).Scan(&taskPoints); err != nil {
		t.Fatalf("verify task story points persisted through update: %v", err)
	}
	if taskPoints != 2 {
		t.Fatalf("persisted task story points = %d, want 2", taskPoints)
	}

	today := time.Now().Format("2006-01-02")
	tomorrow := time.Now().Add(24 * time.Hour).Format("2006-01-02")
	sprintOutput := run(ownerConfig, "", "sprint", "create", "--name", "CLI Sprint "+suffix, "--start-date", today, "--end-date", tomorrow)
	sprintID := firstUUID(t, sprintOutput, "sprint")
	run(ownerConfig, "", "sprint", "list")
	run(ownerConfig, "", "sprint", "show", sprintID)
	run(ownerConfig, "", "sprint", "update", sprintID, "--goal", "CLI E2E goal")
	run(ownerConfig, "", "sprint", "add-story", sprintID, storyID)
	runFailure(ownerConfig, "sprint", "add-story", sprintID, storyID)
	runFailure(ownerConfig, "sprint", "add-story", sprintID, "00000000-0000-0000-0000-000000000001")
	var sprintMembershipCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM sprint_stories WHERE sprint_id = $1 AND story_id = $2", sprintID, storyID).Scan(&sprintMembershipCount); err != nil {
		t.Fatalf("verify duplicate story membership was not added: %v", err)
	}
	if sprintMembershipCount != 1 {
		t.Fatalf("sprint-story membership count after duplicate add = %d, want 1", sprintMembershipCount)
	}
	run(ownerConfig, "", "sprint", "add-story", sprintID, secondStoryID)
	emptyBacklogOutput := run(ownerConfig, "", "sprint", "backlog")
	requireOutput(t, emptyBacklogOutput, "sprint backlog is empty")
	boardOutput := run(ownerConfig, "", "sprint", "board", sprintID)
	requireOutput(t, boardOutput, "CLI Story Updated "+suffix)
	requireOutput(t, boardOutput, "CLI Story Two "+suffix)
	run(ownerConfig, "", "sprint", "remove-story", sprintID, storyID)
	run(ownerConfig, "", "sprint", "remove-story", sprintID, secondStoryID)
	backlogOutput = run(ownerConfig, "", "sprint", "backlog")
	requireOrderedOutput(t, backlogOutput, "CLI Story Updated "+suffix, "CLI Story Two "+suffix)

	plannedOutput := run(ownerConfig, "", "sprint", "create", "--name", "CLI Planned Delete "+suffix)
	plannedSprintID := firstUUID(t, plannedOutput, "planned sprint")
	run(ownerConfig, "", "sprint", "add-story", plannedSprintID, secondStoryID)
	run(ownerConfig, "", "sprint", "delete", plannedSprintID, "--yes")
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM sprint_stories WHERE sprint_id = $1", plannedSprintID).Scan(&sprintMembershipCount); err != nil {
		t.Fatalf("verify planned sprint deletion cleanup: %v", err)
	}
	if sprintMembershipCount != 0 {
		t.Fatalf("sprint-story memberships remain after planned sprint deletion: %d", sprintMembershipCount)
	}
	backlogOutput = run(ownerConfig, "", "sprint", "backlog")
	requireOutput(t, backlogOutput, "CLI Story Two "+suffix)
	runFailure(ownerConfig, "sprint", "show", "00000000-0000-0000-0000-000000000001")
	runFailure(ownerConfig, "sprint", "complete", sprintID)
	run(ownerConfig, "", "sprint", "start", sprintID)
	runFailure(ownerConfig, "sprint", "start", sprintID)
	runFailure(ownerConfig, "sprint", "delete", sprintID, "--yes")
	run(ownerConfig, "", "sprint", "complete", sprintID)
	runFailure(ownerConfig, "sprint", "delete", sprintID, "--yes")
	run(ownerConfig, "", "sprint", "reopen", sprintID)
	run(ownerConfig, "", "sprint", "delete", sprintID, "--yes")

	otherWorkspaceOutput := run(ownerConfig, "", "workspace", "create", "--name", "CLI Other "+suffix)
	otherWorkspaceID := firstUUID(t, otherWorkspaceOutput, "second workspace")
	otherSprintOutput := run(ownerConfig, "", "sprint", "create", "--name", "CLI Other Sprint "+suffix)
	otherSprintID := firstUUID(t, otherSprintOutput, "second-workspace sprint")
	runFailure(ownerConfig, "sprint", "add-story", otherSprintID, secondStoryID)
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM sprint_stories WHERE sprint_id = $1 AND story_id = $2", otherSprintID, secondStoryID).Scan(&sprintMembershipCount); err != nil {
		t.Fatalf("verify cross-workspace story rejection: %v", err)
	}
	if sprintMembershipCount != 0 {
		t.Fatalf("cross-workspace sprint-story membership was persisted: %d", sprintMembershipCount)
	}
	run(ownerConfig, "", "workspace", "switch", workspaceID)
	run(ownerConfig, "", "story", "delete", storyID, "--yes")
	run(ownerConfig, "", "story", "delete", secondStoryID, "--yes")
	run(ownerConfig, "", "epic", "delete", epicID, "--yes")
	run(ownerConfig, "", "activity", "--limit", "20")

	notificationOutput := run(memberConfig, "", "notifications", "list")
	for _, eventText := range []string{"workspace_invitation", "task_assigned", "user_story_assigned"} {
		requireOutput(t, notificationOutput, eventText)
	}
	run(memberConfig, "", "notifications", "read-all")
	readOutput := run(memberConfig, "", "notifications", "list")
	requireOutput(t, readOutput, "0 unread")

	var invitationCount, taskNotificationCount, unreadCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM notifications WHERE recipient_user_id = $1 AND workspace_id = $2 AND event_type = 'workspace_invitation'", memberID, workspaceID).Scan(&invitationCount); err != nil {
		t.Fatalf("verify invitation notification persisted: %v", err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM notifications WHERE recipient_user_id = $1 AND workspace_id = $2 AND event_type = 'task_assigned'", memberID, workspaceID).Scan(&taskNotificationCount); err != nil {
		t.Fatalf("verify task notification persisted: %v", err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM notifications WHERE recipient_user_id = $1 AND workspace_id = $2 AND read_at IS NULL", memberID, workspaceID).Scan(&unreadCount); err != nil {
		t.Fatalf("verify notification read state persisted: %v", err)
	}
	if invitationCount != 1 || taskNotificationCount == 0 || unreadCount != 0 {
		t.Fatalf("notification evidence: invitations=%d task_assignments=%d unread=%d", invitationCount, taskNotificationCount, unreadCount)
	}
	var role string
	if err := pool.QueryRow(ctx, "SELECT role FROM workspace_members WHERE workspace_id = $1 AND user_id = $2", workspaceID, memberID).Scan(&role); err != nil {
		t.Fatalf("verify accepted workspace membership persisted: %v", err)
	}
	if role != "member" {
		t.Fatalf("persisted member role = %q, want member", role)
	}

	run(ownerConfig, "", "logout", "--yes")
	run(memberConfig, "", "logout", "--yes")
	t.Logf("PostgreSQL evidence: workspace=%s member=%s task=%s status=%s notifications=%d/%d unread=%d other_workspace=%s", workspaceID, memberID, taskID, taskStatus, invitationCount, taskNotificationCount, unreadCount, otherWorkspaceID)
}

func uniqueCLISuffix(t *testing.T) string {
	t.Helper()
	random := make([]byte, 8)
	if _, err := rand.Read(random); err != nil {
		t.Fatalf("generate unique CLI integration-test suffix: %v", err)
	}
	for i := range random {
		random[i] = 'a' + random[i]%26
	}
	return string(random)
}

var cliUUIDPattern = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)

func firstUUID(t *testing.T, output, context string) string {
	t.Helper()
	id := cliUUIDPattern.FindString(output)
	if id == "" {
		t.Fatalf("%s output did not contain a UUID:\n%s", context, output)
	}
	return id
}

func requireOutput(t *testing.T, output, expected string) {
	t.Helper()
	if !strings.Contains(output, expected) {
		t.Fatalf("CLI output did not contain %q:\n%s", expected, output)
	}
}

func requireOrderedOutput(t *testing.T, output, first, second string) {
	t.Helper()
	firstIndex := strings.Index(output, first)
	secondIndex := strings.Index(output, second)
	if firstIndex < 0 || secondIndex < 0 || firstIndex >= secondIndex {
		t.Fatalf("CLI output did not list %q before %q:\n%s", first, second, output)
	}
}
