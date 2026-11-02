package commands

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/ben-anass/DouzTach/client/api"
	"github.com/ben-anass/DouzTach/client/config"
	"github.com/ben-anass/DouzTach/client/terminal/prompts"
	"github.com/ben-anass/DouzTach/client/validation"
	"github.com/spf13/cobra"
)

// newTestApp builds an isolated command environment: temporary config
// directory, captured streams, no inherited DOUZTACH_* variables, and no
// colours so assertions can compare plain text.
func newTestApp(t *testing.T) (*App, string) {
	t.Helper()
	for _, key := range []string{
		"DOUZTACH_API_URL",
		"DOUZTACH_API_BASE_URL",
		"DOUZTACH_TIMEOUT",
		"DOUZTACH_WORKSPACE",
		"DOUZTACH_TOKEN",
		"DOUZTACH_USER_NAME",
		"DOUZTACH_USER_EMAIL",
		"DOUZTACH_CONFIG_DIR",
		"NO_COLOR",
		"TERM",
	} {
		t.Setenv(key, "")
	}
	t.Setenv("NO_COLOR", "1")

	dir := t.TempDir()
	app := NewApp("test-1.2.3")
	app.In = strings.NewReader("")
	app.Out = &bytes.Buffer{}
	app.Err = &bytes.Buffer{}
	app.Interactive = false
	if err := app.Load(GlobalFlags{ConfigDir: dir}); err != nil {
		t.Fatalf("load app: %v", err)
	}
	return app, dir
}

// exec runs the command tree with the given arguments and returns everything it
// wrote (stdout and stderr share one buffer for simpler assertions).
func exec(t *testing.T, app *App, dir string, args ...string) (string, error) {
	t.Helper()
	out := &bytes.Buffer{}
	app.Out = out
	app.Err = out

	root := NewRootCommand(app)
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs(append([]string{"--config-dir", dir}, args...))
	err := root.Execute()
	return out.String(), err
}

func TestVersionCommandReportsBuildInfo(t *testing.T) {
	app, dir := newTestApp(t)
	out, err := exec(t, app, dir, "version")
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	for _, want := range []string{"test-1.2.3", "douztech", "Platform"} {
		if !strings.Contains(out, want) {
			t.Errorf("version output missing %q:\n%s", want, out)
		}
	}
}

func TestVersionWorksWithoutReadableConfiguration(t *testing.T) {
	app, dir := newTestApp(t)
	if err := os.WriteFile(dir+"/config.json", []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := exec(t, app, dir, "version")
	if err != nil {
		t.Fatalf("version must not depend on the configuration file: %v", err)
	}
	if !strings.Contains(out, "test-1.2.3") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

func TestRootHelpListsEveryCommandGroup(t *testing.T) {
	app, dir := newTestApp(t)
	out, err := exec(t, app, dir, "--help")
	if err != nil {
		t.Fatalf("help: %v", err)
	}
	commands := []string{
		"login", "register", "logout", "dashboard", "workspace", "task", "board",
		"sprint", "epic", "notifications", "activity", "chat", "profile",
		"settings", "setup", "doctor", "version",
	}
	for _, name := range commands {
		if !strings.Contains(out, name) {
			t.Errorf("root help does not list %q", name)
		}
	}
}

func TestSubcommandHelpIsAvailable(t *testing.T) {
	app, dir := newTestApp(t)
	for _, args := range [][]string{
		{"task", "--help"},
		{"workspace", "--help"},
		{"settings", "--help"},
		{"notifications", "--help"},
		{"chat", "--help"},
	} {
		out, err := exec(t, app, dir, args...)
		if err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		if !strings.Contains(out, "Usage:") {
			t.Errorf("%v produced no usage section:\n%s", args, out)
		}
	}
}

func TestSettingsSetGetAndUnsetRoundTrip(t *testing.T) {
	app, dir := newTestApp(t)

	if _, err := exec(t, app, dir, "settings", "set", "api-url", "http://localhost:8080/api/"); err != nil {
		t.Fatalf("set api-url: %v", err)
	}
	if app.Config.APIBaseURL != "http://localhost:8080/api" {
		t.Fatalf("APIBaseURL = %q", app.Config.APIBaseURL)
	}
	if _, err := exec(t, app, dir, "settings", "set", "timeout", "45s"); err != nil {
		t.Fatalf("set timeout: %v", err)
	}
	if _, err := exec(t, app, dir, "settings", "set", "workspace", "ws-1"); err != nil {
		t.Fatalf("set workspace: %v", err)
	}

	// The values must survive a reload from disk.
	manager, err := config.NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	reloaded, err := manager.Load()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.APIBaseURL != "http://localhost:8080/api" {
		t.Errorf("persisted APIBaseURL = %q", reloaded.APIBaseURL)
	}
	if reloaded.TimeoutDuration().String() != "45s" {
		t.Errorf("persisted timeout = %s", reloaded.TimeoutDuration())
	}
	if reloaded.WorkspaceID != "ws-1" {
		t.Errorf("persisted workspace = %q", reloaded.WorkspaceID)
	}

	out, err := exec(t, app, dir, "settings", "get", "api-url")
	if err != nil {
		t.Fatalf("get api-url: %v", err)
	}
	if strings.TrimSpace(out) != "http://localhost:8080/api" {
		t.Fatalf("get api-url = %q", strings.TrimSpace(out))
	}

	if _, err := exec(t, app, dir, "settings", "unset", "api-url"); err != nil {
		t.Fatalf("unset api-url: %v", err)
	}
	if app.Config.APIBaseURL != "" {
		t.Fatalf("APIBaseURL should be cleared, got %q", app.Config.APIBaseURL)
	}
}

func TestSettingsRejectsInvalidValues(t *testing.T) {
	app, dir := newTestApp(t)
	tests := []struct {
		name string
		args []string
		want int
	}{
		{name: "scheme", args: []string{"settings", "set", "api-url", "ftp://example.com"}, want: ExitUsage},
		{name: "credentials in URL", args: []string{"settings", "set", "api-url", "https://u:p@example.com"}, want: ExitUsage},
		{name: "empty URL", args: []string{"settings", "set", "api-url", "  "}, want: ExitFailure},
		{name: "timeout too long", args: []string{"settings", "set", "timeout", "10m"}, want: ExitUsage},
		{name: "timeout not a duration", args: []string{"settings", "set", "timeout", "soon"}, want: ExitUsage},
		{name: "workspace with space", args: []string{"settings", "set", "workspace", "two words"}, want: ExitUsage},
		{name: "unknown key", args: []string{"settings", "set", "colour", "blue"}, want: ExitFailure},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := exec(t, app, dir, tt.args...)
			if err == nil {
				t.Fatalf("%v should fail", tt.args)
			}
			if got := ExitCode(err); got != tt.want {
				t.Fatalf("exit code = %d, want %d (err: %v)", got, tt.want, err)
			}
		})
	}
}

func TestSettingsShowPrintsCurrentValues(t *testing.T) {
	app, dir := newTestApp(t)
	if _, err := exec(t, app, dir, "settings", "set", "api-url", "http://localhost:9000"); err != nil {
		t.Fatal(err)
	}
	out, err := exec(t, app, dir, "settings")
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	for _, want := range []string{"api-url", "http://localhost:9000", "timeout", "workspace", "backend"} {
		if !strings.Contains(out, want) {
			t.Errorf("settings output missing %q:\n%s", want, out)
		}
	}
}

func TestDoctorReportsMissingAPIURL(t *testing.T) {
	app, dir := newTestApp(t)
	out, err := exec(t, app, dir, "doctor")
	if err == nil {
		t.Fatal("doctor should fail when no API URL is configured")
	}
	if !strings.Contains(out, "api url") || !strings.Contains(out, "not configured") {
		t.Fatalf("doctor output should explain the missing URL:\n%s", out)
	}
	if !strings.Contains(out, "contract") || !strings.Contains(out, "pending") {
		t.Fatalf("doctor output should report the pending contract:\n%s", out)
	}
	if got := ExitCode(err); got != ExitFailure {
		t.Fatalf("exit code = %d, want %d", got, ExitFailure)
	}
}

func TestDoctorReportsUnreachableBackend(t *testing.T) {
	app, dir := newTestApp(t)
	// Port 1 on the loopback interface refuses connections immediately.
	if _, err := exec(t, app, dir, "settings", "set", "api-url", "http://127.0.0.1:1"); err != nil {
		t.Fatal(err)
	}
	out, err := exec(t, app, dir, "doctor")
	if err == nil {
		t.Fatal("doctor should fail when the backend does not answer")
	}
	if !strings.Contains(out, "unreachable") {
		t.Fatalf("doctor output should mention the unreachable backend:\n%s", out)
	}
}

func TestDataCommandsFailHonestlyWithoutTheContract(t *testing.T) {
	app, dir := newTestApp(t)
	if _, err := exec(t, app, dir, "settings", "set", "workspace", "ws-1"); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		args []string
	}{
		{name: "task list", args: []string{"task", "list"}},
		{name: "task show", args: []string{"task", "show", "t-1"}},
		{name: "task create", args: []string{"task", "create", "--title", "A task"}},
		{name: "task update", args: []string{"task", "update", "t-1", "--status", "DONE"}},
		{name: "task delete", args: []string{"task", "delete", "t-1", "--yes"}},
		{name: "workspace list", args: []string{"workspace", "list"}},
		{name: "workspace members", args: []string{"workspace", "members"}},
		{name: "dashboard", args: []string{"dashboard"}},
		{name: "board", args: []string{"board"}},
		{name: "sprint list", args: []string{"sprint", "list"}},
		{name: "epic list", args: []string{"epic", "list"}},
		{name: "notifications", args: []string{"notifications", "list"}},
		{name: "notifications read", args: []string{"notifications", "read", "n-1"}},
		{name: "notifications read-all", args: []string{"notifications", "read-all"}},
		{name: "activity", args: []string{"activity"}},
		{name: "chat channels", args: []string{"chat", "channels"}},
		{name: "chat messages", args: []string{"chat", "messages", "c-1"}},
		{name: "chat send", args: []string{"chat", "send", "c-1", "--message", "hi"}},
		{name: "login", args: []string{"login"}},
		{name: "register", args: []string{"register"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := exec(t, app, dir, tt.args...)
			if err == nil {
				t.Fatalf("%v should fail while the contract is pending", tt.args)
			}
			if !isPending(err) {
				t.Fatalf("%v should fail with a pending-contract error, got %v", tt.args, err)
			}
			if got := ExitCode(err); got != ExitUnavailable {
				t.Fatalf("exit code = %d, want %d", got, ExitUnavailable)
			}
		})
	}
}

// connectedApp returns an application that looks signed in and connected. The
// root pre-run rebuilds the pending services, so commands that need the ready
// state are executed directly instead of through the root command.
func connectedApp(t *testing.T) *App {
	t.Helper()
	app, _ := newTestApp(t)
	app.Services.Status.ContractReady = true
	app.Session = &config.Session{AccessToken: "token", User: config.User{ID: "u-1", Name: "Jane"}}
	return app
}

// runDirect executes one command without the root persistent pre-run.
func runDirect(t *testing.T, app *App, cmd *cobra.Command, args ...string) (string, error) {
	t.Helper()
	out := &bytes.Buffer{}
	app.Out = out
	app.Err = out
	cmd.SetOut(out)
	cmd.SetErr(out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func TestCommandsRequireAWorkspace(t *testing.T) {
	tests := []struct {
		name string
		cmd  func(*App) *cobra.Command
		args []string
	}{
		{name: "task list", cmd: newTaskListCommand},
		{name: "task create", cmd: newTaskCreateCommand, args: []string{"--title", "A task"}},
		{name: "board", cmd: newBoardCommand},
		{name: "sprint list", cmd: newSprintListCommand},
		{name: "activity", cmd: newActivityCommand},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := connectedApp(t)
			_, err := runDirect(t, app, tt.cmd(app), tt.args...)
			if err == nil {
				t.Fatalf("%s should require a workspace", tt.name)
			}
			if !strings.Contains(err.Error(), "no workspace selected") {
				t.Errorf("error = %v, want workspace guidance", err)
			}
		})
	}
}

func TestChatSendValidatesTheMessageBody(t *testing.T) {
	app := connectedApp(t)

	_, err := runDirect(t, app, newChatSendCommand(app), "c-1")
	if err == nil || !strings.Contains(err.Error(), "message body is required") {
		t.Fatalf("expected a missing-body error, got %v", err)
	}

	_, err = runDirect(t, app, newChatSendCommand(app), "c-1", "--message", "   ")
	if err == nil || !strings.Contains(err.Error(), "message body is required") {
		t.Fatalf("expected a missing-body error, got %v", err)
	}

	long := strings.Repeat("x", validation.MaxMessageLength+1)
	_, err = runDirect(t, app, newChatSendCommand(app), "c-1", "--message", long)
	if err == nil || !validation.IsFieldError(err) {
		t.Fatalf("expected a length validation error, got %v", err)
	}
}

func TestActivityRejectsOutOfRangeLimit(t *testing.T) {
	app, dir := newTestApp(t)
	for _, value := range []string{"0", "500", "-3"} {
		_, err := exec(t, app, dir, "activity", "--limit", value)
		if err == nil || !strings.Contains(err.Error(), "--limit") {
			t.Errorf("--limit %s should fail, got %v", value, err)
		}
	}
}

func TestProfileRequiresSignIn(t *testing.T) {
	app, dir := newTestApp(t)
	out, err := exec(t, app, dir, "profile")
	if err == nil {
		t.Fatal("profile should fail when no session exists")
	}
	if got := ExitCode(err); got != ExitNotSignedIn {
		t.Fatalf("exit code = %d, want %d", got, ExitNotSignedIn)
	}
	if !strings.Contains(out, "Not signed in") {
		t.Fatalf("profile output should say the user is not signed in:\n%s", out)
	}
}

func TestProfileShowsCachedIdentity(t *testing.T) {
	app, dir := newTestApp(t)
	manager, err := config.NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	session := &config.Session{
		AccessToken: "secret-token-value",
		User:        config.User{ID: "u-1", Name: "Jane Doe", Email: "jane@example.com"},
	}
	if err := manager.SaveSession(session); err != nil {
		t.Fatal(err)
	}
	if err := app.Load(GlobalFlags{ConfigDir: dir}); err != nil {
		t.Fatal(err)
	}
	out, err := exec(t, app, dir, "profile")
	if err != nil {
		t.Fatalf("profile: %v", err)
	}
	if !strings.Contains(out, "Jane Doe") || !strings.Contains(out, "jane@example.com") {
		t.Fatalf("profile output missing identity:\n%s", out)
	}
	if strings.Contains(out, "secret-token-value") {
		t.Fatal("profile must never print the access token")
	}
}

func TestSetupRequiresInputWhenNotInteractive(t *testing.T) {
	app, dir := newTestApp(t)
	_, err := exec(t, app, dir, "setup")
	if err == nil || !strings.Contains(err.Error(), "--api-url") {
		t.Fatalf("setup should ask for --api-url when not interactive, got %v", err)
	}
}

func TestSetupSavesAPIURL(t *testing.T) {
	app, dir := newTestApp(t)
	// The backend does not need to answer for the URL to be saved.
	out, err := exec(t, app, dir, "setup", "--api-url", "http://127.0.0.1:1", "--timeout", "20s")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	if app.Config.APIBaseURL != "http://127.0.0.1:1" {
		t.Fatalf("APIBaseURL = %q", app.Config.APIBaseURL)
	}
	if app.Config.TimeoutDuration().String() != "20s" {
		t.Fatalf("timeout = %s", app.Config.TimeoutDuration())
	}
	if !strings.Contains(out, "http://127.0.0.1:1") {
		t.Fatalf("setup output should confirm the saved URL:\n%s", out)
	}
}

func TestDashboardWithoutContractShowsLocalSummary(t *testing.T) {
	app, dir := newTestApp(t)
	out, err := exec(t, app, dir, "dashboard")
	if err == nil {
		t.Fatal("dashboard should report the pending contract")
	}
	if !isPending(err) {
		t.Fatalf("expected a pending error, got %v", err)
	}
	if !strings.Contains(out, "DouzTach") || !strings.Contains(out, "Contract") {
		t.Fatalf("dashboard should print the local summary:\n%s", out)
	}
	if strings.Contains(out, "Total tasks") {
		t.Fatal("dashboard must not fabricate task counts")
	}
}

func TestInvalidArgumentsAreRejected(t *testing.T) {
	app, dir := newTestApp(t)
	tests := [][]string{
		{"task", "show"},
		{"task", "show", "one", "two"},
		{"workspace", "switch", "a", "b"},
		{"settings", "get"},
		{"logout", "extra"},
	}
	for _, args := range tests {
		if _, err := exec(t, app, dir, args...); err == nil {
			t.Errorf("%v should reject the arguments", args)
		}
	}
}

func TestUnknownSettingDoesNotChangeConfiguration(t *testing.T) {
	app, dir := newTestApp(t)
	if _, err := exec(t, app, dir, "settings", "set", "api-url", "http://localhost:8080"); err != nil {
		t.Fatal(err)
	}
	if _, err := exec(t, app, dir, "settings", "set", "nope", "value"); err == nil {
		t.Fatal("unknown setting should fail")
	}
	if app.Config.APIBaseURL != "http://localhost:8080" {
		t.Fatalf("configuration changed unexpectedly: %q", app.Config.APIBaseURL)
	}
}

func TestExitCodeMapping(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "nil", err: nil, want: ExitOK},
		{name: "generic", err: errors.New("boom"), want: ExitFailure},
		{name: "cancelled", err: prompts.ErrCancelled, want: ExitCancelled},
		{name: "not signed in", err: api.ErrNotSignedIn, want: ExitNotSignedIn},
		{name: "contract pending", err: &PendingError{Feature: "Tasks"}, want: ExitUnavailable},
		{name: "not configured", err: api.ErrNotConfigured, want: ExitUnavailable},
		{name: "field error", err: &validation.FieldError{Field: "email", Message: "is required"}, want: ExitUsage},
		{name: "unauthorized", err: &api.APIError{StatusCode: http.StatusUnauthorized}, want: ExitNotSignedIn},
		{name: "forbidden", err: &api.APIError{StatusCode: http.StatusForbidden}, want: ExitNotSignedIn},
		{name: "not found", err: &api.APIError{StatusCode: http.StatusNotFound}, want: ExitNotFound},
		{name: "rate limited", err: &api.APIError{StatusCode: http.StatusTooManyRequests}, want: ExitRateLimited},
		{name: "server error", err: &api.APIError{StatusCode: http.StatusInternalServerError}, want: ExitServerError},
		{name: "unprocessable", err: &api.APIError{StatusCode: http.StatusUnprocessableEntity}, want: ExitUsage},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExitCode(tt.err); got != tt.want {
				t.Fatalf("ExitCode(%v) = %d, want %d", tt.err, got, tt.want)
			}
		})
	}
}

func TestTerminalDetectionRejectsPipesAndBuffers(t *testing.T) {
	if streamIsTerminal(&bytes.Buffer{}) {
		t.Fatal("a buffer is not a terminal")
	}
	if streamIsTerminal(io.Discard) {
		t.Fatal("io.Discard is not a terminal")
	}
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()
	if streamIsTerminal(writer) {
		t.Fatal("a pipe is not a terminal")
	}
	if isInteractive(reader) {
		t.Fatal("piped input must not be treated as a terminal")
	}
	if isInteractive(strings.NewReader("")) {
		t.Fatal("a string reader is not a terminal")
	}
}

func TestReportErrorNeverPrintsTokens(t *testing.T) {
	app, _ := newTestApp(t)
	out := &bytes.Buffer{}
	ReportError(out, app.Styles, api.ErrNotConfigured)
	if out.Len() == 0 {
		t.Fatal("ReportError wrote nothing")
	}
	if strings.Contains(strings.ToLower(out.String()), "secret") {
		t.Fatalf("ReportError leaked sensitive text: %s", out.String())
	}
}

func TestReportErrorExplainsPendingContracts(t *testing.T) {
	app, _ := newTestApp(t)
	out := &bytes.Buffer{}
	ReportError(out, app.Styles, pending("Tasks"))
	text := out.String()
	if !strings.Contains(text, "Tasks") {
		t.Fatalf("ReportError should name the feature: %q", text)
	}
	if !strings.Contains(text, "douztech doctor") {
		t.Fatalf("ReportError should point at doctor: %q", text)
	}
}
