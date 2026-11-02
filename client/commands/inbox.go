package commands

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ben-anass/DouzTach/client/terminal/prompts"
	"github.com/ben-anass/DouzTach/client/validation"
	"github.com/spf13/cobra"
)

const (
	defaultActivityLimit = 20
	maxActivityLimit     = 200
	defaultMessageLimit  = 50
	maxMessageLimit      = 200
)

func newNotificationCommand(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "notifications",
		Aliases: []string{"notification", "notif", "inbox"},
		Short:   "List notifications and mark them as read",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runNotificationList(app, cmd)
		},
	}
	cmd.AddCommand(
		newNotificationListCommand(app),
		newNotificationReadCommand(app),
		newNotificationReadAllCommand(app),
	)
	return cmd
}

func newNotificationListCommand(app *App) *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List notifications for your account",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runNotificationList(app, cmd)
		},
	}
}

func runNotificationList(app *App, cmd *cobra.Command) error {
	if err := app.requireReady("Notifications"); err != nil {
		return err
	}
	ctx, cancel := app.requestContext(cmd.Context())
	defer cancel()
	notifications, err := app.Services.Notifications.List(ctx)
	if err != nil {
		return err
	}
	if len(notifications) == 0 {
		app.empty("No notifications.")
		return nil
	}
	rows := make([][]string, 0, len(notifications))
	unread := 0
	for _, notification := range notifications {
		marker := " "
		if !notification.Read {
			marker = "●"
			unread++
		}
		rows = append(rows, []string{
			marker,
			cell(notification.Title, 48),
			dash(notification.Type),
			relativeTime(notification.CreatedAt, app.Now()),
		})
	}
	app.table([]string{"", "TITLE", "TYPE", "RECEIVED"}, rows)
	app.spacer()
	app.muted("%d unread · %d total", unread, len(notifications))
	return nil
}

func newNotificationReadCommand(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "read <notification-id>",
		Short: "Mark one notification as read",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.requireReady("Notifications"); err != nil {
				return err
			}
			notificationID := strings.TrimSpace(args[0])
			if err := validation.ID("notification ID", notificationID); err != nil {
				return err
			}
			ctx, cancel := app.requestContext(cmd.Context())
			defer cancel()
			if err := app.Services.Notifications.MarkRead(ctx, notificationID); err != nil {
				return err
			}
			app.success("Marked notification %s as read.", notificationID)
			return nil
		},
	}
}

func newNotificationReadAllCommand(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "read-all",
		Short: "Mark every notification as read",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.requireReady("Notifications"); err != nil {
				return err
			}
			ctx, cancel := app.requestContext(cmd.Context())
			defer cancel()
			if err := app.Services.Notifications.MarkAllRead(ctx); err != nil {
				return err
			}
			app.success("Marked all notifications as read.")
			return nil
		},
	}
}

func newActivityCommand(app *App) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "activity",
		Short: "Show the workspace activity feed",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if limit < 1 || limit > maxActivityLimit {
				return fmt.Errorf("--limit must be between 1 and %d", maxActivityLimit)
			}
			if err := app.requireReady("Activity history"); err != nil {
				return err
			}
			workspaceID, err := app.workspaceID()
			if err != nil {
				return err
			}
			ctx, cancel := app.requestContext(cmd.Context())
			defer cancel()
			entries, err := app.Services.Activity.List(ctx, workspaceID, limit)
			if err != nil {
				return err
			}
			if len(entries) == 0 {
				app.empty("No activity recorded for this workspace.")
				return nil
			}
			rows := make([][]string, 0, len(entries))
			for _, entry := range entries {
				actor := entry.ActorName
				if strings.TrimSpace(actor) == "" {
					actor = "Someone"
				}
				rows = append(rows, []string{
					relativeTime(entry.CreatedAt, app.Now()),
					cell(actor, 20),
					dash(entry.Action),
					cell(entry.Subject, 48),
				})
			}
			app.table([]string{"WHEN", "ACTOR", "ACTION", "SUBJECT"}, rows)
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", defaultActivityLimit, "maximum entries to show")
	return cmd
}

func newChatCommand(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "chat",
		Short: "List chat channels and exchange messages",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(
		newChatChannelsCommand(app),
		newChatMessagesCommand(app),
		newChatSendCommand(app),
	)
	return cmd
}

func newChatChannelsCommand(app *App) *cobra.Command {
	return &cobra.Command{
		Use:     "channels",
		Aliases: []string{"ls"},
		Short:   "List chat channels available to you",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.requireReady("Chat"); err != nil {
				return err
			}
			ctx, cancel := app.requestContext(cmd.Context())
			defer cancel()
			channels, err := app.Services.Chat.Channels(ctx)
			if err != nil {
				return err
			}
			if len(channels) == 0 {
				app.empty("No chat channels.")
				return nil
			}
			rows := make([][]string, 0, len(channels))
			for _, channel := range channels {
				rows = append(rows, []string{
					dash(channel.ID),
					"# " + cell(channel.Name, 30),
					fmt.Sprintf("%d", channel.MemberCount),
					cell(channel.Description, 40),
				})
			}
			app.table([]string{"ID", "CHANNEL", "MEMBERS", "DESCRIPTION"}, rows)
			app.spacer()
			app.muted("Read a channel with 'douztech chat messages <channel-id>'.")
			return nil
		},
	}
}

func newChatMessagesCommand(app *App) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "messages <channel-id>",
		Short: "Show the most recent messages in a channel",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if limit < 1 || limit > maxMessageLimit {
				return fmt.Errorf("--limit must be between 1 and %d", maxMessageLimit)
			}
			if err := app.requireReady("Chat"); err != nil {
				return err
			}
			channelID := strings.TrimSpace(args[0])
			if err := validation.ID("channel ID", channelID); err != nil {
				return err
			}
			ctx, cancel := app.requestContext(cmd.Context())
			defer cancel()
			messages, err := app.Services.Chat.Messages(ctx, channelID, limit)
			if err != nil {
				return err
			}
			if len(messages) == 0 {
				app.empty("This channel has no messages yet.")
				return nil
			}
			app.title("#" + channelID)
			for _, message := range messages {
				sender := message.SenderName
				if strings.TrimSpace(sender) == "" {
					sender = "Someone"
				}
				fmt.Fprintf(app.Out, "%s %s\n  %s\n",
					app.Styles.Muted.Render(formatTimestamp(message.CreatedAt)),
					app.Styles.Text.Render(sender),
					app.Styles.Text.Render(cell(message.Body, 100)),
				)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", defaultMessageLimit, "maximum messages to show")
	return cmd
}

func newChatSendCommand(app *App) *cobra.Command {
	var message string
	cmd := &cobra.Command{
		Use:   "send <channel-id>",
		Short: "Send a message to a channel",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.requireReady("Chat"); err != nil {
				return err
			}
			channelID := strings.TrimSpace(args[0])
			if err := validation.ID("channel ID", channelID); err != nil {
				return err
			}
			body := strings.TrimSpace(message)
			if body == "" {
				if !app.Interactive {
					return errors.New("message body is required; pass --message")
				}
				typed, err := prompts.RunInput(cmd.Context(), app.Styles, "Message", "Write a message",
					func(value string) error { return validation.MessageBody("message", value) })
				if err != nil {
					return err
				}
				body = strings.TrimSpace(typed)
			}
			if body == "" {
				return errors.New("the message body is empty")
			}
			if err := validation.MessageBody("message", body); err != nil {
				return err
			}

			ctx, cancel := app.requestContext(cmd.Context())
			defer cancel()
			sent, err := app.Services.Chat.Send(ctx, channelID, body)
			if err != nil {
				return err
			}
			if sent == nil || strings.TrimSpace(sent.ID) == "" {
				return errors.New("the backend returned no stored message")
			}
			app.success("Message sent to #%s.", channelID)
			return nil
		},
	}
	cmd.Flags().StringVar(&message, "message", "", "message body")
	return cmd
}
