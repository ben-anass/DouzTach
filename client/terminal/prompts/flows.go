package prompts

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/ben-anass/DouzTach/client/api"
	"github.com/ben-anass/DouzTach/client/terminal/styles"
	"github.com/ben-anass/DouzTach/client/validation"
)

// LoginForm builds the sign-in prompt. It is only shown once a real auth
// contract exists, so credentials are not collected for a non-functional flow.
func LoginForm(s styles.Styles) *Form {
	return NewForm(s, "Sign in to DouzTach", []Field{
		{
			Key:         "email",
			Label:       "Email",
			Placeholder: "you@example.com",
			CharLimit:   validation.MaxEmailLength,
			Validate:    func(value string) error { return validation.Email("email", value) },
		},
		{
			Key:       "password",
			Label:     "Password",
			Secret:    true,
			CharLimit: validation.MaxPasswordLength,
			Validate:  func(value string) error { return validation.LoginPassword("password", value) },
		},
	})
}

// RegisterForm builds the account creation prompt.
func RegisterForm(s styles.Styles) *Form {
	form := NewForm(s, "Create a DouzTach account", []Field{
		{
			Key:         "name",
			Label:       "Full name",
			Placeholder: "Jane Doe",
			CharLimit:   validation.MaxNameLength,
			Validate:    func(value string) error { return validation.PersonName("name", value) },
		},
		{
			Key:         "email",
			Label:       "Email",
			Placeholder: "you@example.com",
			CharLimit:   validation.MaxEmailLength,
			Validate:    func(value string) error { return validation.Email("email", value) },
		},
		{
			Key:       "password",
			Label:     "Password",
			Help:      "At least 8 characters with a letter and a number.",
			Secret:    true,
			CharLimit: validation.MaxPasswordLength,
			Validate:  func(value string) error { return validation.Password("password", value) },
		},
		{
			Key:      "confirm_password",
			Label:    "Confirm password",
			Secret:   true,
			Validate: RequireValue("password confirmation"),
		},
	})
	form.SubmitValidator = func(values map[string]string) error {
		if values["password"] != values["confirm_password"] {
			return errors.New("passwords do not match")
		}
		return nil
	}
	return form
}

// WorkspaceForm builds the workspace creation prompt.
func WorkspaceForm(s styles.Styles) *Form {
	return NewForm(s, "Create a workspace", []Field{
		{
			Key:         "name",
			Label:       "Workspace name",
			Placeholder: "Platform Team",
			CharLimit:   validation.MaxNameLength,
			Validate:    func(value string) error { return validation.WorkspaceName("workspace name", value) },
		},
		{
			Key:         "description",
			Label:       "Description",
			Placeholder: "What this workspace is for",
			Optional:    true,
			CharLimit:   validation.MaxDescriptionLength,
			Validate:    func(value string) error { return validation.Description("description", value) },
		},
	})
}

// TaskForm builds the task creation prompt for the free-text fields. Task
// type, priority, and sprint are chosen with keyboard pickers instead, because
// they are fixed enums.
func TaskForm(s styles.Styles) *Form {
	return NewForm(s, "Create a task", []Field{
		{
			Key:         "title",
			Label:       "Title",
			Placeholder: "Fix the login redirect",
			CharLimit:   validation.MaxTitleLength,
			Validate:    func(value string) error { return validation.TaskTitle("title", value) },
		},
		{
			Key:         "description",
			Label:       "Description",
			Placeholder: "Context and acceptance criteria",
			Optional:    true,
			CharLimit:   validation.MaxDescriptionLength,
			Validate:    func(value string) error { return validation.Description("description", value) },
		},
		{
			Key:         "due_date",
			Label:       "Due date",
			Placeholder: "YYYY-MM-DD",
			Optional:    true,
			Help:        "Optional. Use YYYY-MM-DD.",
			Validate:    parseDueDate,
		},
		{
			Key:         "story_points",
			Label:       "Story points",
			Placeholder: "3",
			Optional:    true,
			Help:        "Optional whole number between 0 and 100.",
			Validate:    parseStoryPoints,
		},
	})
}

// TaskUpdateForm builds the task update prompt for the free-text fields. Status
// and priority are chosen with keyboard pickers, because they are fixed enums.
// Every field is optional, but at least one change is required.
func TaskUpdateForm(s styles.Styles) *Form {
	form := NewForm(s, "Update task", []Field{
		{
			Key:         "title",
			Label:       "New title",
			Placeholder: "Leave empty to keep the current title",
			Optional:    true,
			CharLimit:   validation.MaxTitleLength,
			Validate:    func(value string) error { return validation.TaskTitle("title", value) },
		},
		{
			Key:         "due_date",
			Label:       "Due date",
			Placeholder: "YYYY-MM-DD",
			Optional:    true,
			Validate:    parseDueDate,
		},
		{
			Key:         "description",
			Label:       "Description",
			Placeholder: "Leave empty to keep the current description",
			Optional:    true,
			CharLimit:   validation.MaxDescriptionLength,
			Validate:    func(value string) error { return validation.Description("description", value) },
		},
	})
	form.SubmitValidator = func(values map[string]string) error {
		for _, value := range values {
			if strings.TrimSpace(value) != "" {
				return nil
			}
		}
		return errors.New("provide at least one field to update")
	}
	return form
}

// LogoutConfirm builds the logout confirmation.
func LogoutConfirm(s styles.Styles) *Confirm {
	confirm := NewConfirm(s, "Sign out", "Sign out of DouzTach on this device?")
	confirm.Detail = "The saved session token will be removed from this computer. You can sign in again at any time."
	return confirm
}

// WorkspaceOptions converts workspaces into select options.
func WorkspaceOptions(workspaces []api.Workspace) []Option {
	options := make([]Option, 0, len(workspaces))
	for _, workspace := range workspaces {
		description := workspace.Description
		if workspace.Role != "" {
			if description != "" {
				description += " · "
			}
			description += "role: " + workspace.Role
		}
		options = append(options, Option{
			Label:       workspace.Name,
			Value:       workspace.ID,
			Description: description,
		})
	}
	return options
}

// SprintOptions converts sprints into select options.
func SprintOptions(sprints []api.Sprint) []Option {
	options := make([]Option, 0, len(sprints))
	for _, sprint := range sprints {
		description := string(sprint.Status)
		if sprint.Goal != "" {
			description += " · " + sprint.Goal
		}
		options = append(options, Option{
			Label:       sprint.Name,
			Value:       sprint.ID,
			Description: description,
		})
	}
	return options
}

// TaskOptions converts tasks into select options.
func TaskOptions(tasks []api.Task) []Option {
	options := make([]Option, 0, len(tasks))
	for _, task := range tasks {
		label := task.Title
		if task.Key != "" {
			label = task.Key + " · " + task.Title
		}
		options = append(options, Option{
			Label:       label,
			Value:       task.ID,
			Description: string(task.Status) + " · " + string(task.Priority),
		})
	}
	return options
}

// TaskStatusOptions returns the workflow statuses as select options.
func TaskStatusOptions() []Option {
	options := make([]Option, 0, len(api.TaskStatuses))
	for _, status := range api.TaskStatuses {
		options = append(options, Option{Label: status.Title(), Value: string(status)})
	}
	return options
}

// PriorityOptions returns the priorities as select options.
func PriorityOptions() []Option {
	options := make([]Option, 0, len(api.Priorities))
	for _, priority := range api.Priorities {
		options = append(options, Option{Label: string(priority), Value: string(priority)})
	}
	return options
}

// TaskTypeOptions returns the task types as select options.
func TaskTypeOptions() []Option {
	options := make([]Option, 0, len(api.TaskTypes))
	for _, taskType := range api.TaskTypes {
		options = append(options, Option{Label: string(taskType), Value: string(taskType)})
	}
	return options
}

func parseDueDate(value string) error {
	_, err := validation.Date("due date", value)
	return err
}

func parseStoryPoints(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	points, err := strconv.Atoi(value)
	if err != nil {
		return errors.New("story points must be a whole number")
	}
	if points < 0 || points > 100 {
		return fmt.Errorf("story points must be between 0 and 100")
	}
	return nil
}
