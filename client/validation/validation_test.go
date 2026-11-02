package validation

import (
	"strings"
	"testing"
	"time"
)

func TestRequired(t *testing.T) {
	t.Parallel()
	if err := Required("title", "  "); err == nil {
		t.Fatal("expected required error")
	}
	if err := Required("title", "Task"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEmail(t *testing.T) {
	t.Parallel()
	valid := []string{"jane@example.com", "a.b+tag@sub.example.co"}
	for _, value := range valid {
		if err := Email("email", value); err != nil {
			t.Errorf("Email(%q) = %v", value, err)
		}
	}
	invalid := []string{"", "jane", "jane@", "@example.com", "Jane <jane@example.com>", "jane@example", "jane@@example.com"}
	for _, value := range invalid {
		if err := Email("email", value); err == nil {
			t.Errorf("Email(%q) should fail", value)
		}
	}
}

func TestPassword(t *testing.T) {
	t.Parallel()
	valid := []string{"password1", "P4ssword", "with space 1"}
	for _, value := range valid {
		if err := Password("password", value); err != nil {
			t.Errorf("Password(%q) = %v", value, err)
		}
	}
	invalid := []string{"", "short1", "password", "12345678", "        "}
	for _, value := range invalid {
		if err := Password("password", value); err == nil {
			t.Errorf("Password(%q) should fail", value)
		}
	}
	if err := LoginPassword("password", "old"); err != nil {
		t.Fatalf("LoginPassword should accept existing passwords: %v", err)
	}
}

func TestPersonName(t *testing.T) {
	t.Parallel()
	if err := PersonName("name", "Jane Doe"); err != nil {
		t.Fatal(err)
	}
	if err := PersonName("name", "O'Connor-Smith"); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"", "J", "Jane123", "<script>"} {
		if err := PersonName("name", value); err == nil {
			t.Errorf("PersonName(%q) should fail", value)
		}
	}
}

func TestTaskTitle(t *testing.T) {
	t.Parallel()
	if err := TaskTitle("title", "Fix login"); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"", "ab", strings.Repeat("x", MaxTitleLength+1)} {
		if err := TaskTitle("title", value); err == nil {
			t.Errorf("TaskTitle(%q) should fail", value)
		}
	}
}

func TestID(t *testing.T) {
	t.Parallel()
	if err := ID("id", "task-123"); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"", "has space", "line\nbreak", strings.Repeat("x", MaxIDLength+1)} {
		if err := ID("id", value); err == nil {
			t.Errorf("ID(%q) should fail", value)
		}
	}
}

func TestBaseURL(t *testing.T) {
	t.Parallel()
	got, err := BaseURL("https://example.com/api/")
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://example.com/api" {
		t.Fatalf("BaseURL = %q", got)
	}
	if got, err := BaseURL(""); err != nil || got != "" {
		t.Fatalf("empty URL = %q, %v", got, err)
	}
	for _, value := range []string{"ftp://example.com", "example.com", "https://user:pass@example.com", "https://example.com?x=1", "https://example.com#frag"} {
		if _, err := BaseURL(value); err == nil {
			t.Errorf("BaseURL(%q) should fail", value)
		}
	}
}

func TestTimeout(t *testing.T) {
	t.Parallel()
	if err := Timeout(15 * time.Second); err != nil {
		t.Fatal(err)
	}
	for _, value := range []time.Duration{0, 500 * time.Millisecond, 10 * time.Minute} {
		if err := Timeout(value); err == nil {
			t.Errorf("Timeout(%s) should fail", value)
		}
	}
	if d, err := ParseDuration("timeout", "30s"); err != nil || d != 30*time.Second {
		t.Fatalf("ParseDuration = %s, %v", d, err)
	}
	if _, err := ParseDuration("timeout", "soon"); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestDate(t *testing.T) {
	t.Parallel()
	got, err := Date("due date", "2030-01-02")
	if err != nil {
		t.Fatal(err)
	}
	if got.Format("2006-01-02") != "2030-01-02" {
		t.Fatalf("date = %s", got)
	}
	if got.Hour() != 23 || got.Minute() != 59 {
		t.Fatalf("date should be end of day, got %s", got)
	}
	if _, err := Date("due date", "02/01/2030"); err == nil {
		t.Fatal("expected date error")
	}
	if empty, err := Date("due date", ""); err != nil || !empty.IsZero() {
		t.Fatalf("empty date = %v, %v", empty, err)
	}
}

func TestEnum(t *testing.T) {
	t.Parallel()
	got, err := Enum("status", "in_progress", []string{"TODO", "IN_PROGRESS", "REVIEW", "DONE"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "IN_PROGRESS" {
		t.Fatalf("Enum = %q", got)
	}
	if _, err := Enum("status", "blocked", []string{"TODO", "DONE"}); err == nil {
		t.Fatal("expected enum error")
	}
}

func TestFieldError(t *testing.T) {
	t.Parallel()
	err := Required("email", "")
	if !IsFieldError(err) {
		t.Fatalf("expected field error, got %T", err)
	}
	if !strings.Contains(err.Error(), "email") {
		t.Fatalf("error should mention field: %v", err)
	}
}
