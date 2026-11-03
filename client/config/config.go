package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ben-anass/DouzTach/client/validation"
)

// DefaultTimeout is used when the user has not configured a request timeout.
const DefaultTimeout = 15 * time.Second

// Duration is a time.Duration that serialises as a human-readable string such
// as "15s" in config.json.
type Duration time.Duration

// Std returns the standard library duration.
func (d Duration) Std() time.Duration { return time.Duration(d) }

func (d Duration) String() string { return time.Duration(d).String() }

// MarshalJSON implements json.Marshaler.
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

// UnmarshalJSON accepts either a duration string ("15s") or a number of
// seconds for backwards compatibility.
func (d *Duration) UnmarshalJSON(data []byte) error {
	var raw string
	if err := json.Unmarshal(data, &raw); err == nil {
		parsed, err := time.ParseDuration(raw)
		if err != nil {
			return fmt.Errorf("invalid duration %q: %w", raw, err)
		}
		*d = Duration(parsed)
		return nil
	}
	var seconds float64
	if err := json.Unmarshal(data, &seconds); err != nil {
		return fmt.Errorf("duration must be a string such as \"15s\" or a number of seconds")
	}
	*d = Duration(time.Duration(seconds * float64(time.Second)))
	return nil
}

// Config is the persistent client configuration. It never contains secrets.
type Config struct {
	APIBaseURL  string   `json:"api_base_url"`
	Timeout     Duration `json:"timeout"`
	WorkspaceID string   `json:"workspace_id,omitempty"`
}

// Default returns a configuration with no API URL and a safe default timeout.
// No backend URL is guessed: the team's API contract and deployment address
// must be supplied explicitly.
func Default() Config {
	return Config{Timeout: Duration(DefaultTimeout)}
}

// Normalize trims user input and removes a trailing slash from the API URL.
func (c *Config) Normalize() {
	c.APIBaseURL = strings.TrimSpace(c.APIBaseURL)
	c.APIBaseURL = strings.TrimRight(c.APIBaseURL, "/")
	c.WorkspaceID = strings.TrimSpace(c.WorkspaceID)
}

// Validate checks the configuration. An empty API URL is allowed; it means the
// client has not been pointed at a backend yet.
func (c Config) Validate() error {
	if _, err := validation.BaseURL(c.APIBaseURL); err != nil {
		return err
	}
	if c.Timeout == 0 {
		return &validation.FieldError{Field: "timeout", Message: "is required"}
	}
	if err := validation.Timeout(c.Timeout.Std()); err != nil {
		return err
	}
	if c.WorkspaceID != "" {
		if err := validation.ID("workspace ID", c.WorkspaceID); err != nil {
			return err
		}
	}
	return nil
}

// TimeoutDuration returns the configured timeout as a time.Duration.
func (c Config) TimeoutDuration() time.Duration {
	if c.Timeout == 0 {
		return DefaultTimeout
	}
	return c.Timeout.Std()
}

// ApplyEnv overlays supported environment variables on cfg. Environment values
// are intended for local development and CI; they are not written back to disk
// unless the user explicitly saves the configuration.
func ApplyEnv(cfg Config) (Config, error) {
	if raw := firstEnv("DOUZTACH_API_URL", "DOUZTACH_API_BASE_URL"); raw != "" {
		normalized, err := validation.BaseURL(raw)
		if err != nil {
			return cfg, fmt.Errorf("environment variable DOUZTACH_API_URL: %w", err)
		}
		cfg.APIBaseURL = normalized
	}
	if raw := os.Getenv("DOUZTACH_TIMEOUT"); raw != "" {
		d, err := validation.ParseDuration("DOUZTACH_TIMEOUT", raw)
		if err != nil {
			return cfg, err
		}
		cfg.Timeout = Duration(d)
	}
	if raw := os.Getenv("DOUZTACH_WORKSPACE"); raw != "" {
		if err := validation.ID("DOUZTACH_WORKSPACE", raw); err != nil {
			return cfg, err
		}
		cfg.WorkspaceID = strings.TrimSpace(raw)
	}
	return cfg, nil
}

func firstEnv(keys ...string) string {
	for _, key := range keys {
		if value := os.Getenv(key); value != "" {
			return value
		}
	}
	return ""
}

// ParseTimeout is a convenience wrapper used by flags and prompts.
func ParseTimeout(value string) (time.Duration, error) {
	return validation.ParseDuration("timeout", value)
}

// FormatTimeout renders a duration consistently in help output.
func FormatTimeout(d time.Duration) string {
	if d == 0 {
		return DefaultTimeout.String()
	}
	return d.String()
}

// ParseBool is a small helper for environment flags.
func ParseBool(value string) (bool, error) {
	if value == "" {
		return false, nil
	}
	return strconv.ParseBool(value)
}
