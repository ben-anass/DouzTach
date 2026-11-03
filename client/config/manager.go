package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	configFileName  = "config.json"
	sessionFileName = "session.json"
	maxConfigBytes  = 1 << 20 // 1 MiB
	dirPerm         = 0o700
	filePerm        = 0o600
)

// ErrEphemeralSession is returned when attempting to persist a session whose
// token came from the environment rather than from a login response.
var ErrEphemeralSession = errors.New("sessions supplied through DOUZTACH_TOKEN are not written to disk")

// Manager owns the on-disk configuration and session files.
type Manager struct {
	dir string
}

// NewManager creates a manager rooted at dir. If dir is empty, the platform
// user config directory is used (or DOUZTACH_CONFIG_DIR when set).
func NewManager(dir string) (*Manager, error) {
	if dir == "" {
		var err error
		dir, err = DefaultDir()
		if err != nil {
			return nil, err
		}
	}
	dir, err := expandHome(dir)
	if err != nil {
		return nil, err
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("resolve config directory: %w", err)
	}
	return &Manager{dir: abs}, nil
}

// DefaultDir returns the platform-specific configuration directory.
func DefaultDir() (string, error) {
	if dir := strings.TrimSpace(os.Getenv("DOUZTACH_CONFIG_DIR")); dir != "" {
		return expandHome(dir)
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locate user config directory: %w", err)
	}
	return filepath.Join(base, "douztech"), nil
}

// Dir returns the configuration directory.
func (m *Manager) Dir() string { return m.dir }

// ConfigPath returns the absolute path to config.json.
func (m *Manager) ConfigPath() string { return filepath.Join(m.dir, configFileName) }

// SessionPath returns the absolute path to session.json.
func (m *Manager) SessionPath() string { return filepath.Join(m.dir, sessionFileName) }

// Load reads config.json and applies environment overrides. A missing file is
// not an error; it returns the default configuration.
func (m *Manager) Load() (Config, error) {
	cfg := Default()
	data, err := readLimitedFile(m.ConfigPath(), maxConfigBytes)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return cfg, err
		}
	} else {
		if err := json.Unmarshal(data, &cfg); err != nil {
			return cfg, fmt.Errorf("parse %s: %w", m.ConfigPath(), err)
		}
	}
	cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		return cfg, fmt.Errorf("invalid configuration in %s: %w", m.ConfigPath(), err)
	}
	cfg, err = ApplyEnv(cfg)
	if err != nil {
		return cfg, err
	}
	cfg.Normalize()
	return cfg, cfg.Validate()
}

// Save validates and atomically writes config.json with owner-only
// permissions.
func (m *Manager) Save(cfg Config) error {
	cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode configuration: %w", err)
	}
	data = append(data, '\n')
	if err := m.ensureDir(); err != nil {
		return err
	}
	return writeFileAtomic(m.ConfigPath(), data, filePerm)
}

// LoadSession reads session.json. A missing file returns (nil, nil). A token in
// DOUZTACH_TOKEN takes precedence and produces an ephemeral session that is
// never written back to disk.
func (m *Manager) LoadSession() (*Session, error) {
	if token := strings.TrimSpace(os.Getenv("DOUZTACH_TOKEN")); token != "" {
		return &Session{
			AccessToken: token,
			TokenType:   "Bearer",
			Ephemeral:   true,
			User: User{
				Name:  os.Getenv("DOUZTACH_USER_NAME"),
				Email: os.Getenv("DOUZTACH_USER_EMAIL"),
			},
		}, nil
	}

	data, err := readLimitedFile(m.SessionPath(), maxConfigBytes)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("parse %s: %w", m.SessionPath(), err)
	}
	if err := session.Validate(); err != nil {
		return nil, fmt.Errorf("invalid session in %s: %w", m.SessionPath(), err)
	}
	return &session, nil
}

// SaveSession atomically writes session.json with owner-only permissions.
func (m *Manager) SaveSession(session *Session) error {
	if session == nil {
		return errors.New("session is nil")
	}
	if session.Ephemeral {
		return ErrEphemeralSession
	}
	if err := session.Validate(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("encode session: %w", err)
	}
	data = append(data, '\n')
	if err := m.ensureDir(); err != nil {
		return err
	}
	return writeFileAtomic(m.SessionPath(), data, filePerm)
}

// ClearSession removes the local session file. Removing a missing file is not
// an error.
func (m *Manager) ClearSession() error {
	err := os.Remove(m.SessionPath())
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("clear session: %w", err)
	}
	return nil
}

// SessionExists reports whether a session file is present. It does not validate
// the token.
func (m *Manager) SessionExists() bool {
	_, err := os.Stat(m.SessionPath())
	return err == nil
}

func (m *Manager) ensureDir() error {
	if err := os.MkdirAll(m.dir, dirPerm); err != nil {
		return fmt.Errorf("create config directory %s: %w", m.dir, err)
	}
	if err := os.Chmod(m.dir, dirPerm); err != nil && !errors.Is(err, os.ErrPermission) {
		// Chmod is best-effort on Windows, where ACLs are used instead of
		// Unix permission bits. A permission error is not fatal here.
		return fmt.Errorf("secure config directory %s: %w", m.dir, err)
	}
	return nil
}

func readLimitedFile(path string, limit int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, fmt.Errorf("%s is a directory", path)
	}
	if info.Size() > limit {
		return nil, fmt.Errorf("%s is too large (limit %d bytes)", path, limit)
	}
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%s is too large (limit %d bytes)", path, limit)
	}
	return data, nil
}

func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".douztech-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary file in %s: %w", dir, err)
	}
	tmpName := tmp.Name()
	defer func() {
		_ = os.Remove(tmpName)
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close %s: %w", path, err)
	}
	if err := os.Chmod(tmpName, perm); err != nil && !errors.Is(err, os.ErrPermission) {
		return fmt.Errorf("secure %s: %w", path, err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		// On some Windows filesystems Rename cannot replace an existing file.
		// Remove the target and retry; the temporary file remains complete.
		if removeErr := os.Remove(path); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return fmt.Errorf("replace %s: %w", path, err)
		}
		if renameErr := os.Rename(tmpName, path); renameErr != nil {
			return fmt.Errorf("replace %s: %w", path, renameErr)
		}
	}
	if err := os.Chmod(path, perm); err != nil && !errors.Is(err, os.ErrPermission) {
		return fmt.Errorf("secure %s: %w", path, err)
	}
	return nil
}

func expandHome(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", errors.New("path is empty")
	}
	if path == "~" || strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("locate home directory: %w", err)
		}
		if path == "~" {
			return home, nil
		}
		return filepath.Join(home, path[2:]), nil
	}
	return path, nil
}
