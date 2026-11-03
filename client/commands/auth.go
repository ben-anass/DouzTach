package commands

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/ben-anass/DouzTach/client/api"
	"github.com/ben-anass/DouzTach/client/config"
	"github.com/ben-anass/DouzTach/client/terminal/prompts"
	"github.com/ben-anass/DouzTach/client/validation"
	"github.com/spf13/cobra"
)

func newLoginCommand(app *App) *cobra.Command {
	var (
		email         string
		passwordStdin bool
	)
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Sign in and store the session token",
		Long: `Sign in with your DouzTach account.

The password is never accepted as a command-line flag because process arguments
are visible to other users. Use the interactive prompt, or pipe the password
with --password-stdin for automation.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.requireContract("Authentication"); err != nil {
				return err
			}
			if app.Session != nil && app.Session.Valid() {
				return fmt.Errorf("already signed in as %s; run 'douztech logout' first",
					app.Session.User.DisplayName())
			}

			values, err := collectLoginValues(cmd.Context(), app, email, passwordStdin)
			if err != nil {
				return err
			}
			ctx, cancel := app.requestContext(cmd.Context())
			defer cancel()
			result, err := app.Services.Auth.Login(ctx, api.LoginRequest{
				Email:    values["email"],
				Password: values["password"],
			})
			if err != nil {
				return err
			}
			session, err := sessionFromAuth(result)
			if err != nil {
				return err
			}
			if err := app.saveSession(session); err != nil {
				return err
			}
			app.success("Signed in as %s.", session.User.DisplayName())
			return nil
		},
	}
	cmd.Flags().StringVar(&email, "email", "", "account email (use with --password-stdin)")
	cmd.Flags().BoolVar(&passwordStdin, "password-stdin", false, "read the password from standard input")
	return cmd
}

func newRegisterCommand(app *App) *cobra.Command {
	var (
		name          string
		email         string
		passwordStdin bool
	)
	cmd := &cobra.Command{
		Use:   "register",
		Short: "Create an account and store the session token",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.requireContract("Registration"); err != nil {
				return err
			}
			if app.Session != nil && app.Session.Valid() {
				return fmt.Errorf("already signed in as %s; run 'douztech logout' first",
					app.Session.User.DisplayName())
			}

			values, err := collectRegisterValues(cmd.Context(), app, name, email, passwordStdin)
			if err != nil {
				return err
			}
			ctx, cancel := app.requestContext(cmd.Context())
			defer cancel()
			result, err := app.Services.Auth.Register(ctx, api.RegisterRequest{
				Name:     values["name"],
				Email:    values["email"],
				Password: values["password"],
			})
			if err != nil {
				return err
			}
			session, err := sessionFromAuth(result)
			if err != nil {
				return err
			}
			if err := app.saveSession(session); err != nil {
				return err
			}
			app.success("Account created. Signed in as %s.", session.User.DisplayName())
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "full name (use with --email and --password-stdin)")
	cmd.Flags().StringVar(&email, "email", "", "account email (use with --name and --password-stdin)")
	cmd.Flags().BoolVar(&passwordStdin, "password-stdin", false, "read the password from standard input")
	return cmd
}

func newLogoutCommand(app *App) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Remove the saved session from this device",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if app.Session == nil && (app.Manager == nil || !app.Manager.SessionExists()) {
				app.muted("No saved session on this device.")
				return nil
			}
			if app.Session != nil && app.Session.Ephemeral {
				return errors.New("the session comes from DOUZTACH_TOKEN; unset that environment variable to sign out")
			}
			if !yes {
				if !app.Interactive {
					return errors.New("refusing to remove the saved session without confirmation; re-run with --yes")
				}
				confirmed, err := prompts.RunConfirm(cmd.Context(), prompts.LogoutConfirm(app.Styles))
				if err != nil {
					return err
				}
				if !confirmed {
					app.muted("Sign-out cancelled.")
					return nil
				}
			}
			if app.Services.Ready() {
				ctx, cancel := app.requestContext(cmd.Context())
				err := app.Services.Auth.Logout(ctx)
				cancel()
				if err != nil {
					app.warn("the server-side logout call failed: %v", err)
				}
			}
			if app.Manager != nil {
				if err := app.Manager.ClearSession(); err != nil {
					return err
				}
			}
			app.Session = nil
			if err := app.rebuildServices(); err != nil {
				return err
			}
			app.success("Signed out. The saved session token was removed.")
			return nil
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip the confirmation prompt")
	return cmd
}

func collectLoginValues(ctx context.Context, app *App, email string, passwordStdin bool) (map[string]string, error) {
	if email != "" || passwordStdin {
		if email == "" || !passwordStdin {
			return nil, errors.New("--email and --password-stdin must be used together")
		}
		if err := validation.Email("email", email); err != nil {
			return nil, err
		}
		password, err := readSecret(app.In)
		if err != nil {
			return nil, err
		}
		if err := validation.LoginPassword("password", password); err != nil {
			return nil, err
		}
		return map[string]string{"email": strings.TrimSpace(email), "password": password}, nil
	}
	if !app.Interactive {
		return nil, errors.New("login needs an interactive terminal, or --email with --password-stdin")
	}
	return prompts.RunForm(ctx, prompts.LoginForm(app.Styles))
}

func collectRegisterValues(ctx context.Context, app *App, name, email string, passwordStdin bool) (map[string]string, error) {
	if name != "" || email != "" || passwordStdin {
		if name == "" || email == "" || !passwordStdin {
			return nil, errors.New("--name, --email, and --password-stdin must be used together")
		}
		if err := validation.PersonName("name", name); err != nil {
			return nil, err
		}
		if err := validation.Email("email", email); err != nil {
			return nil, err
		}
		password, err := readSecret(app.In)
		if err != nil {
			return nil, err
		}
		if err := validation.Password("password", password); err != nil {
			return nil, err
		}
		return map[string]string{
			"name":     strings.TrimSpace(name),
			"email":    strings.TrimSpace(email),
			"password": password,
		}, nil
	}
	if !app.Interactive {
		return nil, errors.New("registration needs an interactive terminal, or --name, --email, and --password-stdin")
	}
	return prompts.RunForm(ctx, prompts.RegisterForm(app.Styles))
}

func readSecret(reader io.Reader) (string, error) {
	buffered := bufio.NewReader(io.LimitReader(reader, 4096))
	line, err := buffered.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("read password: %w", err)
	}
	line = strings.TrimRight(line, "\r\n")
	if line == "" {
		return "", errors.New("password read from stdin is empty")
	}
	return line, nil
}

func sessionFromAuth(result *api.AuthResult) (*config.Session, error) {
	if result == nil {
		return nil, errors.New("the backend returned no authentication result")
	}
	if strings.TrimSpace(result.AccessToken) == "" {
		return nil, errors.New("the backend returned an empty access token; no session was saved")
	}
	session := &config.Session{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		TokenType:    result.TokenType,
		ExpiresAt:    result.ExpiresAt,
		User: config.User{
			ID:       result.User.ID,
			Name:     result.User.Name,
			Username: result.User.Username,
			Email:    result.User.Email,
		},
	}
	if err := session.Validate(); err != nil {
		return nil, err
	}
	return session, nil
}
