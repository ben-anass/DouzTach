package commands

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/ben-anass/DouzTach/client/api"
	"github.com/ben-anass/DouzTach/client/config"
	"github.com/ben-anass/DouzTach/client/terminal/screens"
	"github.com/ben-anass/DouzTach/client/terminal/styles"
	"github.com/ben-anass/DouzTach/client/validation"
	"github.com/charmbracelet/x/term"
)

// GlobalFlags are the persistent options accepted by every command.
type GlobalFlags struct {
	ConfigDir string
	APIURL    string
	Timeout   time.Duration
	NoColor   bool
}

// App is the shared command environment. It is constructed once per process
// and populated by the root command's PersistentPreRunE after flag parsing.
type App struct {
	Version string

	Manager  *config.Manager
	Config   config.Config
	Session  *config.Session
	Services api.Services
	Styles   styles.Styles

	In          io.Reader
	Out         io.Writer
	Err         io.Writer
	Interactive bool

	now func() time.Time
}

// NewApp builds an application environment with standard streams.
func NewApp(version string) *App {
	return &App{
		Version: version,
		In:      os.Stdin,
		Out:     os.Stdout,
		Err:     os.Stderr,
		Styles:  styles.New(false),
		now:     time.Now,
	}
}

// Now returns the current time through the injectable clock.
func (a *App) Now() time.Time {
	if a.now == nil {
		return time.Now()
	}
	return a.now()
}

// Load reads configuration and session state, applies flag overrides, and
// builds the service bundle. Flag overrides are in-memory only; use
// 'douztech settings set' to persist a value.
func (a *App) Load(flags GlobalFlags) error {
	manager, err := config.NewManager(flags.ConfigDir)
	if err != nil {
		return err
	}
	cfg, err := manager.Load()
	if err != nil {
		return err
	}
	if flags.APIURL != "" {
		normalized, err := validation.BaseURL(flags.APIURL)
		if err != nil {
			return fmt.Errorf("--api-url: %w", err)
		}
		cfg.APIBaseURL = normalized
	}
	if flags.Timeout != 0 {
		if err := validation.Timeout(flags.Timeout); err != nil {
			return fmt.Errorf("--timeout: %w", err)
		}
		cfg.Timeout = config.Duration(flags.Timeout)
	}
	session, err := manager.LoadSession()
	if err != nil {
		return err
	}

	a.Manager = manager
	a.Config = cfg
	a.Session = session
	a.Styles = styles.New(flags.NoColor || styles.DetectNoColor())
	a.Interactive = isInteractive(a.In)
	return a.rebuildServices()
}

// rebuildServices recreates the API bundle after a config or session change.
func (a *App) rebuildServices() error {
	services, err := api.NewServices(a.Config.APIBaseURL, a.Config.TimeoutDuration(), func() string {
		if a.Session == nil {
			return ""
		}
		return a.Session.BearerToken()
	})
	if err != nil {
		return err
	}
	a.Services = services
	return nil
}

// requireReady fails fast when the backend contract is unavailable or the user
// is not signed in. It is always called before collecting credentials.
func (a *App) requireReady(feature string) error {
	if err := a.Services.RequireReady(); err != nil {
		return pending(feature)
	}
	if a.Session == nil || !a.Session.Valid() {
		return errNotSignedIn()
	}
	return nil
}

// requireContract checks only the backend contract. Commands that work with
// local state but benefit from a backend use this.
func (a *App) requireContract(feature string) error {
	if err := a.Services.RequireReady(); err != nil {
		return pending(feature)
	}
	return nil
}

// workspaceID returns the selected workspace, or an actionable error.
func (a *App) workspaceID() (string, error) {
	if id := strings.TrimSpace(a.Config.WorkspaceID); id != "" {
		return id, nil
	}
	return "", fmt.Errorf("no workspace selected; run 'douztech workspace list' and 'douztech workspace switch <id>'")
}

// saveConfig persists the current configuration.
func (a *App) saveConfig() error {
	if a.Manager == nil {
		return fmt.Errorf("configuration manager is not initialized")
	}
	if err := a.Manager.Save(a.Config); err != nil {
		return err
	}
	return a.rebuildServices()
}

// saveSession persists a session returned by the backend and rebuilds the API
// client so subsequent requests use the new token.
func (a *App) saveSession(session *config.Session) error {
	if a.Manager == nil {
		return fmt.Errorf("configuration manager is not initialized")
	}
	if err := a.Manager.SaveSession(session); err != nil {
		return err
	}
	a.Session = session
	return a.rebuildServices()
}

// requestContext applies the configured timeout to a command context.
func (a *App) requestContext(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	return context.WithTimeout(parent, a.Config.TimeoutDuration())
}

// screenContext builds a rendering context for reusable screen components.
func (a *App) screenContext(width int) screens.Context {
	configDir := ""
	if a.Manager != nil {
		configDir = a.Manager.Dir()
	}
	return screens.Context{
		Styles:    a.Styles,
		Width:     width,
		Services:  a.Services,
		Config:    a.Config,
		Session:   a.Session,
		ConfigDir: configDir,
		Now:       a.Now(),
	}
}

// stdoutIsTerminal reports whether the output stream can host the interactive
// dashboard. Redirecting stdout to a file or a pipe must fall back to the
// scriptable commands instead of streaming escape codes into the file.
func (a *App) stdoutIsTerminal() bool { return streamIsTerminal(a.Out) }

func streamIsTerminal(w io.Writer) bool {
	file, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(file.Fd())
}

// isInteractive reports whether the stream is a real terminal. A character
// device is not enough: /dev/null and NUL are character devices too, and
// launching the dashboard with them would read no input at all.
func isInteractive(r io.Reader) bool {
	file, ok := r.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(file.Fd())
}
