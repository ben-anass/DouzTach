package commands

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/ben-anass/DouzTach/client/config"
	"github.com/ben-anass/DouzTach/client/terminal"
	"github.com/ben-anass/DouzTach/client/terminal/components"
	"github.com/ben-anass/DouzTach/client/terminal/prompts"
	"github.com/ben-anass/DouzTach/client/validation"
	"github.com/spf13/cobra"
)

func newVersionCommand(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the client version and build information",
		Args:  cobra.NoArgs,
		// Version information must stay available even when the saved
		// configuration cannot be read, so the global pre-run is skipped.
		PersistentPreRunE: func(*cobra.Command, []string) error { return nil },
		RunE: func(cmd *cobra.Command, args []string) error {
			version := strings.TrimSpace(app.Version)
			if version == "" {
				version = "development"
			}
			app.title("DouzTach " + version)
			app.keyValues([][2]string{
				{"Client", "douztech"},
				{"Version", version},
				{"Go", runtime.Version()},
				{"Platform", runtime.GOOS + "/" + runtime.GOARCH},
			}, 10)
			return nil
		},
	}
}

func newProfileCommand(app *App) *cobra.Command {
	return &cobra.Command{
		Use:     "profile",
		Aliases: []string{"whoami"},
		Short:   "Show the signed-in account",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			app.title("Profile")
			if app.Session == nil || !app.Session.Valid() {
				app.empty("Not signed in. Run 'douztech login' to sign in.")
				return errNotSignedIn()
			}

			user := app.Session.User
			refreshed := false
			if app.Services.Ready() {
				ctx, cancel := app.requestContext(cmd.Context())
				profile, err := app.Services.Auth.Profile(ctx)
				cancel()
				if err != nil {
					app.warn("could not refresh the profile from the backend: %v", err)
				} else if profile != nil {
					user = config.User{
						ID:       profile.ID,
						Name:     profile.Name,
						Username: profile.Username,
						Email:    profile.Email,
					}
					refreshed = true
				}
			}

			app.keyValues([][2]string{
				{"Name", dash(user.DisplayName())},
				{"Email", dash(user.Email)},
				{"Username", dash(user.Username)},
				{"User ID", dash(user.ID)},
				{"Workspace", dash(app.Config.WorkspaceID)},
				{"Session", app.Session.ExpiresIn(app.Now())},
			}, 12)
			if !refreshed {
				app.spacer()
				app.muted("This is the identity cached on this device. The live profile needs the backend API contract.")
			}
			return nil
		},
	}
}

func newSettingsCommand(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "settings",
		Short: "Show and change the client configuration",
		Long: `Show and change the client configuration.

Configuration is stored in config.json inside the configuration directory and
never contains credentials. Session tokens live in a separate session.json that
is written with owner-only permissions.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSettingsShow(app)
		},
	}
	cmd.AddCommand(
		newSettingsListCommand(app),
		newSettingsGetCommand(app),
		newSettingsSetCommand(app),
		newSettingsUnsetCommand(app),
	)
	return cmd
}

func newSettingsListCommand(app *App) *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls", "show"},
		Short:   "Show the current configuration",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSettingsShow(app)
		},
	}
}

func newSettingsGetCommand(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "get <key>",
		Short: "Print one configuration value (api-url, timeout, or workspace)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			value, err := readSetting(app, strings.TrimSpace(args[0]))
			if err != nil {
				return err
			}
			fmt.Fprintln(app.Out, value)
			return nil
		},
	}
}

func newSettingsSetCommand(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Persist one configuration value",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := strings.TrimSpace(args[0])
			if err := applySetting(app, key, args[1]); err != nil {
				return err
			}
			value, err := readSetting(app, key)
			if err != nil {
				return err
			}
			app.success("Set %s to %s.", key, value)
			return nil
		},
	}
}

func newSettingsUnsetCommand(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "unset <key>",
		Short: "Clear one configuration value",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := strings.TrimSpace(args[0])
			if err := clearSetting(app, key); err != nil {
				return err
			}
			app.success("Cleared %s.", key)
			return nil
		},
	}
}

func runSettingsShow(app *App) error {
	app.title("Settings")
	apiURL := app.Config.APIBaseURL
	if apiURL == "" {
		apiURL = "not configured"
	}
	workspace := app.Config.WorkspaceID
	if workspace == "" {
		workspace = "not selected"
	}
	configPath := "unavailable"
	if app.Manager != nil {
		configPath = app.Manager.ConfigPath()
	}
	app.keyValues([][2]string{
		{"api-url", apiURL},
		{"timeout", app.Config.TimeoutDuration().String()},
		{"workspace", workspace},
		{"config", configPath},
		{"backend", app.Services.Status.Summary()},
	}, 12)
	app.spacer()
	app.muted("Change a value with 'douztech settings set <key> <value>'.")
	return nil
}

func readSetting(app *App, key string) (string, error) {
	switch key {
	case "api-url", "api_url", "url":
		return app.Config.APIBaseURL, nil
	case "timeout":
		return app.Config.TimeoutDuration().String(), nil
	case "workspace":
		return app.Config.WorkspaceID, nil
	default:
		return "", unknownSettingError(key)
	}
}

func applySetting(app *App, key, value string) error {
	switch key {
	case "api-url", "api_url", "url":
		normalized, err := validation.BaseURL(value)
		if err != nil {
			return err
		}
		if normalized == "" {
			return errors.New("api-url must not be empty; use 'douztech settings unset api-url' to clear it")
		}
		app.Config.APIBaseURL = normalized
	case "timeout":
		parsed, err := validation.ParseDuration("timeout", value)
		if err != nil {
			return err
		}
		app.Config.Timeout = config.Duration(parsed)
	case "workspace":
		if err := validation.ID("workspace", value); err != nil {
			return err
		}
		app.Config.WorkspaceID = strings.TrimSpace(value)
	default:
		return unknownSettingError(key)
	}
	return app.saveConfig()
}

func clearSetting(app *App, key string) error {
	switch key {
	case "api-url", "api_url", "url":
		app.Config.APIBaseURL = ""
	case "timeout":
		app.Config.Timeout = config.Duration(config.DefaultTimeout)
	case "workspace":
		app.Config.WorkspaceID = ""
	default:
		return unknownSettingError(key)
	}
	return app.saveConfig()
}

func unknownSettingError(key string) error {
	return fmt.Errorf("unknown setting %q (expected api-url, timeout, or workspace)", key)
}

func newSetupCommand(app *App) *cobra.Command {
	var (
		apiURL  string
		timeout string
	)
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Configure the backend URL and verify the connection",
		Long: `Configure this installation.

The wizard asks for the backend URL, writes it to config.json, and reports
whether the backend answers. It never asks for a password on a non-interactive
terminal and never stores credentials in the configuration file.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !app.Interactive && strings.TrimSpace(apiURL) == "" {
				return errors.New("setup needs an interactive terminal, or pass --api-url (and optionally --timeout)")
			}
			terminal.PrintBanner()

			value := strings.TrimSpace(apiURL)
			if value == "" {
				typed, err := readInputWithDefault(cmd.Context(), app,
					"Backend configuration",
					"API base URL",
					"http://localhost:8080",
					app.Config.APIBaseURL,
					func(input string) error {
						normalized, err := validation.BaseURL(input)
						if err != nil {
							return err
						}
						if normalized == "" {
							return errors.New("an API URL is required")
						}
						return nil
					})
				if err != nil {
					return err
				}
				value = typed
			}
			normalized, err := validation.BaseURL(value)
			if err != nil {
				return fmt.Errorf("API URL: %w", err)
			}
			if normalized == "" {
				return errors.New("an API URL is required")
			}
			app.Config.APIBaseURL = normalized

			if strings.TrimSpace(timeout) != "" {
				parsed, err := validation.ParseDuration("timeout", timeout)
				if err != nil {
					return fmt.Errorf("--timeout: %w", err)
				}
				app.Config.Timeout = config.Duration(parsed)
			}
			if err := app.saveConfig(); err != nil {
				return err
			}

			ctx, cancel := app.requestContext(cmd.Context())
			status := app.Services.RefreshStatus(ctx)
			cancel()

			configPath := "unavailable"
			if app.Manager != nil {
				configPath = app.Manager.ConfigPath()
			}
			session := "not signed in"
			if app.Session != nil && app.Session.Valid() {
				session = app.Session.User.DisplayName()
			}
			app.keyValues([][2]string{
				{"API URL", app.Config.APIBaseURL},
				{"Timeout", app.Config.TimeoutDuration().String()},
				{"Config", configPath},
				{"Backend", status.Summary()},
				{"Session", session},
			}, 10)

			if !status.Reachable {
				app.warn("the backend did not answer %s; the URL was saved so you can retry", app.Config.APIBaseURL)
			}
			if !app.Services.Ready() {
				app.spacer()
				app.muted("Next: sign in with 'douztech login' once Member 2 publishes the API contract.")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&apiURL, "api-url", "", "backend API base URL")
	cmd.Flags().StringVar(&timeout, "timeout", "", "request timeout (for example 30s)")
	return cmd
}

func readInputWithDefault(ctx context.Context, app *App, title, label, placeholder, current string, validate func(string) error) (string, error) {
	form := prompts.NewForm(app.Styles, title, []prompts.Field{{
		Key:         "value",
		Label:       label,
		Placeholder: placeholder,
		Default:     current,
		Validate:    validate,
	}})
	values, err := prompts.RunForm(ctx, form)
	if err != nil {
		return "", err
	}
	return values["value"], nil
}

// doctorCheck is one diagnostic result shown by the doctor command.
type doctorCheck struct {
	Name    string
	Status  string
	Detail  string
	Failure bool
}

const (
	checkOK   = "ok"
	checkWarn = "warn"
	checkFail = "fail"
)

func newDoctorCommand(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose the configuration, session, and backend connection",
		Long: `Diagnose this installation.

Doctor checks the configuration file, the API URL, the saved session, the
environment overrides, and whether the backend answers. It exits non-zero when
a required check fails so it can be used in scripts.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			app.title("DouzTach doctor")
			checks := []doctorCheck{}

			configPath := "unavailable"
			if app.Manager != nil {
				configPath = app.Manager.ConfigPath()
				if _, err := os.Stat(configPath); err == nil {
					checks = append(checks, doctorCheck{
						Name:   "configuration",
						Status: checkOK,
						Detail: configPath,
					})
				} else {
					checks = append(checks, doctorCheck{
						Name:   "configuration",
						Status: checkWarn,
						Detail: "no config.json yet; defaults are in use (" + configPath + ")",
					})
				}
			}

			if app.Config.APIBaseURL == "" {
				checks = append(checks, doctorCheck{
					Name:    "api url",
					Status:  checkFail,
					Detail:  "not configured; run 'douztech setup' or set DOUZTACH_API_URL",
					Failure: true,
				})
			} else {
				checks = append(checks, doctorCheck{
					Name:   "api url",
					Status: checkOK,
					Detail: app.Config.APIBaseURL + " (timeout " + app.Config.TimeoutDuration().String() + ")",
				})
			}

			if app.Session != nil && app.Session.Valid() {
				source := "saved session file"
				if app.Session.Ephemeral {
					source = "DOUZTACH_TOKEN environment variable"
				}
				checks = append(checks, doctorCheck{
					Name:   "session",
					Status: checkOK,
					Detail: app.Session.User.DisplayName() + " · expires " + app.Session.ExpiresIn(app.Now()) + " · " + source,
				})
			} else {
				checks = append(checks, doctorCheck{
					Name:   "session",
					Status: checkWarn,
					Detail: "not signed in; run 'douztech login'",
				})
			}

			if overrides := environmentOverrides(); len(overrides) > 0 {
				checks = append(checks, doctorCheck{
					Name:   "environment",
					Status: checkWarn,
					Detail: "overrides " + strings.Join(overrides, ", "),
				})
			} else {
				checks = append(checks, doctorCheck{
					Name:   "environment",
					Status: checkOK,
					Detail: "no DOUZTACH_* overrides",
				})
			}

			if app.Config.APIBaseURL != "" {
				ctx, cancel := app.requestContext(cmd.Context())
				status := app.Services.RefreshStatus(ctx)
				cancel()
				if status.Reachable {
					checks = append(checks, doctorCheck{
						Name:   "backend",
						Status: checkOK,
						Detail: fmt.Sprintf("reachable · HTTP %d · %s", status.StatusCode, status.Latency.Round(1_000_000)),
					})
				} else {
					checks = append(checks, doctorCheck{
						Name:    "backend",
						Status:  checkFail,
						Detail:  "unreachable: " + status.Detail,
						Failure: true,
					})
				}
			}

			if app.Services.Ready() {
				checks = append(checks, doctorCheck{
					Name:   "contract",
					Status: checkOK,
					Detail: "implemented",
				})
			} else {
				checks = append(checks, doctorCheck{
					Name:   "contract",
					Status: checkWarn,
					Detail: "pending: the backend endpoints from Member 2 are not available yet",
				})
			}

			failures := 0
			for _, check := range checks {
				icon := app.Styles.Success.Render("✓")
				switch check.Status {
				case checkWarn:
					icon = app.Styles.Warning.Render("!")
				case checkFail:
					icon = app.Styles.Error.Render("✗")
				}
				if check.Failure {
					failures++
				}
				fmt.Fprintf(app.Out, "%s  %s%s\n", icon,
					app.Styles.Muted.Render(components.PadRight(check.Name, 14)),
					check.Detail)
			}
			fmt.Fprintln(app.Out)

			if failures > 0 {
				return fmt.Errorf("doctor found %d problem(s) that need attention", failures)
			}
			app.success("All required checks passed.")
			return nil
		},
	}
}

// environmentOverrides lists the DOUZTACH_* variables that are set. Values are
// never printed because one of them can carry an access token.
func environmentOverrides() []string {
	names := []string{
		"DOUZTACH_API_URL",
		"DOUZTACH_API_BASE_URL",
		"DOUZTACH_TIMEOUT",
		"DOUZTACH_WORKSPACE",
		"DOUZTACH_CONFIG_DIR",
		"DOUZTACH_TOKEN",
	}
	var set []string
	for _, name := range names {
		if os.Getenv(name) != "" {
			set = append(set, name)
		}
	}
	return set
}
