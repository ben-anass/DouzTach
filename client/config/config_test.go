package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	t.Parallel()
	mgr, err := NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := mgr.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.APIBaseURL != "" {
		t.Fatalf("APIBaseURL = %q, want empty", cfg.APIBaseURL)
	}
	if cfg.Timeout.Std() != DefaultTimeout {
		t.Fatalf("Timeout = %s, want %s", cfg.Timeout, DefaultTimeout)
	}
	if _, err := os.Stat(mgr.ConfigPath()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("config file should not exist yet: %v", err)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mgr, err := NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{
		APIBaseURL:  "https://api.example.com/v1/",
		Timeout:     Duration(30 * time.Second),
		WorkspaceID: "workspace-1",
	}
	if err := mgr.Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := mgr.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.APIBaseURL != "https://api.example.com/v1" {
		t.Fatalf("APIBaseURL = %q", loaded.APIBaseURL)
	}
	if loaded.Timeout.Std() != 30*time.Second {
		t.Fatalf("Timeout = %s", loaded.Timeout)
	}
	if loaded.WorkspaceID != "workspace-1" {
		t.Fatalf("WorkspaceID = %q", loaded.WorkspaceID)
	}

	raw, err := os.ReadFile(mgr.ConfigPath())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "access_token") {
		t.Fatal("config must never contain a token")
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("config is not valid JSON: %v", err)
	}
	if decoded["timeout"] != "30s" {
		t.Fatalf("timeout JSON = %v", decoded["timeout"])
	}
}

func TestSaveRejectsInvalidConfig(t *testing.T) {
	t.Parallel()
	mgr, err := NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	tests := []Config{
		{APIBaseURL: "ftp://example.com", Timeout: Duration(time.Second)},
		{APIBaseURL: "https://example.com", Timeout: Duration(0)},
		{APIBaseURL: "https://example.com", Timeout: Duration(10 * time.Minute)},
		{APIBaseURL: "https://example.com", Timeout: Duration(time.Second), WorkspaceID: "has space"},
	}
	for _, cfg := range tests {
		if err := mgr.Save(cfg); err == nil {
			t.Errorf("Save(%+v) should fail", cfg)
		}
	}
}

func TestEnvironmentOverrides(t *testing.T) {
	dir := t.TempDir()
	mgr, err := NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := mgr.Save(Config{APIBaseURL: "https://file.example.com", Timeout: Duration(10 * time.Second)}); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOUZTACH_API_URL", "https://env.example.com")
	t.Setenv("DOUZTACH_TIMEOUT", "45s")
	t.Setenv("DOUZTACH_WORKSPACE", "env-workspace")

	loaded, err := mgr.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.APIBaseURL != "https://env.example.com" {
		t.Fatalf("APIBaseURL = %q", loaded.APIBaseURL)
	}
	if loaded.Timeout.Std() != 45*time.Second {
		t.Fatalf("Timeout = %s", loaded.Timeout)
	}
	if loaded.WorkspaceID != "env-workspace" {
		t.Fatalf("WorkspaceID = %q", loaded.WorkspaceID)
	}
}

func TestInvalidEnvironmentOverride(t *testing.T) {
	t.Setenv("DOUZTACH_TIMEOUT", "not-a-duration")
	mgr, err := NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mgr.Load(); err == nil {
		t.Fatal("expected invalid environment error")
	}
}

func TestSessionRoundTrip(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mgr, err := NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	expires := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
	session := &Session{
		AccessToken:  "super-secret-token",
		RefreshToken: "refresh-token",
		TokenType:    "Bearer",
		ExpiresAt:    &expires,
		User: User{
			ID:    "u1",
			Name:  "Jane Doe",
			Email: "jane@example.com",
		},
	}
	if err := mgr.SaveSession(session); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}
	loaded, err := mgr.LoadSession()
	if err != nil {
		t.Fatalf("LoadSession: %v", err)
	}
	if loaded == nil || loaded.AccessToken != "super-secret-token" {
		t.Fatalf("unexpected session: %+v", loaded)
	}
	if loaded.User.DisplayName() != "Jane Doe" {
		t.Fatalf("user = %q", loaded.User.DisplayName())
	}
	if !loaded.Valid() {
		t.Fatal("session should be valid")
	}

	if runtime.GOOS != "windows" {
		info, err := os.Stat(mgr.SessionPath())
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm()&0o077 != 0 {
			t.Fatalf("session permissions = %o, want no group/other access", info.Mode().Perm())
		}
	}

	if err := mgr.ClearSession(); err != nil {
		t.Fatalf("ClearSession: %v", err)
	}
	loaded, err = mgr.LoadSession()
	if err != nil {
		t.Fatalf("LoadSession after clear: %v", err)
	}
	if loaded != nil {
		t.Fatalf("expected no session, got %+v", loaded)
	}
}

func TestSessionStringRedactsToken(t *testing.T) {
	t.Parallel()
	session := Session{AccessToken: "super-secret-token", User: User{Name: "Jane"}}
	if strings.Contains(session.String(), "super-secret-token") {
		t.Fatal("Session.String leaked the access token")
	}
	if strings.Contains(session.GoString(), "super-secret-token") {
		t.Fatal("Session.GoString leaked the access token")
	}
}

func TestEphemeralSessionIsNotPersisted(t *testing.T) {
	t.Parallel()
	mgr, err := NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	err = mgr.SaveSession(&Session{AccessToken: "env-token", Ephemeral: true})
	if !errors.Is(err, ErrEphemeralSession) {
		t.Fatalf("expected ErrEphemeralSession, got %v", err)
	}
	if _, statErr := os.Stat(mgr.SessionPath()); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("session file should not exist: %v", statErr)
	}
}

func TestEnvironmentTokenSession(t *testing.T) {
	t.Setenv("DOUZTACH_TOKEN", "env-token")
	t.Setenv("DOUZTACH_USER_NAME", "Env User")
	mgr, err := NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	session, err := mgr.LoadSession()
	if err != nil {
		t.Fatalf("LoadSession: %v", err)
	}
	if session == nil || !session.Ephemeral || session.AccessToken != "env-token" {
		t.Fatalf("unexpected session: %+v", session)
	}
	if session.User.DisplayName() != "Env User" {
		t.Fatalf("user = %q", session.User.DisplayName())
	}
}

func TestExpiredSessionIsInvalid(t *testing.T) {
	t.Parallel()
	past := time.Now().Add(-time.Hour)
	session := &Session{AccessToken: "token", ExpiresAt: &past}
	if session.Valid() {
		t.Fatal("expired session should be invalid")
	}
	if got := session.ExpiresIn(time.Now()); got != "expired" {
		t.Fatalf("ExpiresIn = %q", got)
	}
}

func TestExpandHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory")
	}
	got, err := expandHome("~/douztech")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, "douztech")
	if got != want {
		t.Fatalf("expandHome = %q, want %q", got, want)
	}
}

func TestDurationJSONAcceptsSeconds(t *testing.T) {
	t.Parallel()
	var d Duration
	if err := json.Unmarshal([]byte(`30`), &d); err != nil {
		t.Fatal(err)
	}
	if d.Std() != 30*time.Second {
		t.Fatalf("duration = %s", d)
	}
	raw, err := json.Marshal(Duration(5 * time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != `"5s"` {
		t.Fatalf("json = %s", raw)
	}
}
