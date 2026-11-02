package config

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// User is the locally cached identity associated with a session. It is a
// display model only; the backend remains the source of truth.
type User struct {
	ID       string `json:"id,omitempty"`
	Name     string `json:"name,omitempty"`
	Username string `json:"username,omitempty"`
	Email    string `json:"email,omitempty"`
}

// DisplayName returns the most useful human-readable identifier.
func (u User) DisplayName() string {
	switch {
	case strings.TrimSpace(u.Name) != "":
		return u.Name
	case strings.TrimSpace(u.Username) != "":
		return "@" + strings.TrimPrefix(u.Username, "@")
	case strings.TrimSpace(u.Email) != "":
		return u.Email
	case strings.TrimSpace(u.ID) != "":
		return u.ID
	default:
		return "Unknown user"
	}
}

// Session is the persisted authentication state. The access token is never
// printed by String or GoString.
type Session struct {
	AccessToken  string     `json:"access_token"`
	RefreshToken string     `json:"refresh_token,omitempty"`
	TokenType    string     `json:"token_type,omitempty"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	User         User       `json:"user,omitempty"`

	// Ephemeral sessions are supplied through DOUZTACH_TOKEN and must not be
	// persisted.
	Ephemeral bool `json:"-"`
}

// String redacts secrets so a session can be logged safely.
func (s Session) String() string {
	return fmt.Sprintf("Session{user:%q, expires_at:%v, ephemeral:%t, token:[REDACTED]}",
		s.User.DisplayName(), s.ExpiresAt, s.Ephemeral)
}

// GoString redacts secrets when using %#v.
func (s Session) GoString() string { return s.String() }

// Valid reports whether the session has a token and has not expired.
func (s *Session) Valid() bool {
	if s == nil || strings.TrimSpace(s.AccessToken) == "" {
		return false
	}
	if s.ExpiresAt != nil && !time.Now().Before(*s.ExpiresAt) {
		return false
	}
	return true
}

// Validate checks the persisted session structure.
func (s *Session) Validate() error {
	if s == nil {
		return errors.New("session is nil")
	}
	if strings.TrimSpace(s.AccessToken) == "" {
		return errors.New("access token is empty")
	}
	if s.ExpiresAt != nil && s.ExpiresAt.IsZero() {
		s.ExpiresAt = nil
	}
	return nil
}

// BearerToken returns the token value used in the Authorization header.
func (s *Session) BearerToken() string {
	if s == nil {
		return ""
	}
	return strings.TrimSpace(s.AccessToken)
}

// ExpiresIn returns a human-readable remaining lifetime. It is intentionally
// coarse and never exposes token material.
func (s *Session) ExpiresIn(now time.Time) string {
	if s == nil || s.ExpiresAt == nil {
		return "no expiry reported"
	}
	remaining := s.ExpiresAt.Sub(now)
	if remaining <= 0 {
		return "expired"
	}
	if remaining < time.Minute {
		return "less than a minute"
	}
	if remaining < time.Hour {
		return fmt.Sprintf("%d minutes", int(remaining.Minutes()))
	}
	if remaining < 24*time.Hour {
		return fmt.Sprintf("%d hours", int(remaining.Hours()))
	}
	return fmt.Sprintf("%d days", int(remaining.Hours()/24))
}
