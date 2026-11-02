// Package prompts provides reusable Bubble Tea prompt models for the CLI and
// TUI. Passwords are masked in the view and never written to logs.
package prompts

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/ben-anass/DouzTach/client/terminal/styles"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ErrCancelled is returned when the user aborts a prompt.
var ErrCancelled = errors.New("cancelled")

// Field describes one form input.
type Field struct {
	Key         string
	Label       string
	Placeholder string
	Help        string
	Default     string
	Secret      bool
	Optional    bool
	CharLimit   int
	Validate    func(string) error
}

// Form is a multi-field Bubble Tea prompt.
type Form struct {
	Title           string
	Fields          []Field
	Styles          styles.Styles
	SubmitValidator func(map[string]string) error

	inputs    []textinput.Model
	focus     int
	errs      []error
	submitErr error
	submitted bool
	cancelled bool
	width     int
}

// NewForm builds a form. The first field receives focus in Init.
func NewForm(s styles.Styles, title string, fields []Field) *Form {
	inputs := make([]textinput.Model, len(fields))
	for i, field := range fields {
		input := textinput.New()
		input.Placeholder = field.Placeholder
		input.Prompt = "› "
		input.CharLimit = field.CharLimit
		if field.Secret {
			input.EchoMode = textinput.EchoPassword
			input.EchoCharacter = '•'
		}
		input.PromptStyle = s.Key
		input.TextStyle = s.Text
		input.PlaceholderStyle = s.Muted
		input.Cursor.Style = s.SecondaryStyle()
		if field.Default != "" {
			input.SetValue(field.Default)
			input.CursorEnd()
		}
		inputs[i] = input
	}
	return &Form{
		Title:  title,
		Fields: fields,
		Styles: s,
		inputs: inputs,
		errs:   make([]error, len(fields)),
		width:  80,
	}
}

// Init implements tea.Model.
func (m *Form) Init() tea.Cmd {
	if len(m.inputs) == 0 {
		return tea.Quit
	}
	m.focus = 0
	m.inputs[0].Focus()
	return textinput.Blink
}

// Update implements tea.Model.
func (m *Form) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.resizeInputs()
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.cancelled = true
			return m, tea.Quit
		case "tab", "down":
			m.focusNext()
			return m, nil
		case "shift+tab", "up":
			m.focusPrev()
			return m, nil
		case "enter":
			return m.submitOrAdvance()
		default:
			if len(m.inputs) == 0 {
				return m, nil
			}
			var cmd tea.Cmd
			m.inputs[m.focus], cmd = m.inputs[m.focus].Update(msg)
			m.errs[m.focus] = nil
			m.submitErr = nil
			return m, cmd
		}
	}
	if len(m.inputs) == 0 {
		return m, nil
	}
	var cmd tea.Cmd
	m.inputs[m.focus], cmd = m.inputs[m.focus].Update(msg)
	return m, cmd
}

func (m *Form) submitOrAdvance() (tea.Model, tea.Cmd) {
	if len(m.inputs) == 0 {
		m.submitted = true
		return m, tea.Quit
	}
	if err := m.validateField(m.focus); err != nil {
		m.errs[m.focus] = err
		return m, nil
	}
	if m.focus < len(m.inputs)-1 {
		m.focusNext()
		return m, nil
	}
	if err := m.validateAll(); err != nil {
		m.submitErr = err
		for i, fieldErr := range m.errs {
			if fieldErr != nil {
				m.focus = i
				m.syncFocus()
				break
			}
		}
		return m, nil
	}
	m.submitted = true
	return m, tea.Quit
}

func (m *Form) validateField(index int) error {
	if index < 0 || index >= len(m.Fields) {
		return nil
	}
	field := m.Fields[index]
	value := m.inputs[index].Value()
	if field.Optional && strings.TrimSpace(value) == "" {
		m.errs[index] = nil
		return nil
	}
	if field.Validate != nil {
		err := field.Validate(value)
		m.errs[index] = err
		return err
	}
	m.errs[index] = nil
	return nil
}

func (m *Form) validateAll() error {
	var firstErr error
	for i := range m.Fields {
		if err := m.validateField(i); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if firstErr != nil {
		return firstErr
	}
	if m.SubmitValidator != nil {
		return m.SubmitValidator(m.Values())
	}
	return nil
}

func (m *Form) focusNext() {
	if len(m.inputs) == 0 {
		return
	}
	m.focus = (m.focus + 1) % len(m.inputs)
	m.syncFocus()
}

func (m *Form) focusPrev() {
	if len(m.inputs) == 0 {
		return
	}
	m.focus--
	if m.focus < 0 {
		m.focus = len(m.inputs) - 1
	}
	m.syncFocus()
}

func (m *Form) syncFocus() {
	for i := range m.inputs {
		if i == m.focus {
			m.inputs[i].Focus()
			continue
		}
		m.inputs[i].Blur()
	}
}

func (m *Form) resizeInputs() {
	width := m.width - 8
	if width < 20 {
		width = 20
	}
	if width > 72 {
		width = 72
	}
	for i := range m.inputs {
		m.inputs[i].Width = width
	}
}

// Values returns the current field values. Non-secret values are trimmed;
// passwords are preserved exactly.
func (m *Form) Values() map[string]string {
	values := make(map[string]string, len(m.Fields))
	for i, field := range m.Fields {
		value := ""
		if i < len(m.inputs) {
			value = m.inputs[i].Value()
		}
		if !field.Secret {
			value = strings.TrimSpace(value)
		}
		values[field.Key] = value
	}
	return values
}

// Value returns one field value.
func (m *Form) Value(key string) string { return m.Values()[key] }

// Submitted reports whether the form completed successfully.
func (m *Form) Submitted() bool { return m.submitted }

// Cancelled reports whether the user aborted the form.
func (m *Form) Cancelled() bool { return m.cancelled }

// SubmitError returns the cross-field error shown after a failed submission.
func (m *Form) SubmitError() error { return m.submitErr }

// View implements tea.Model.
func (m *Form) View() string {
	var b strings.Builder
	if m.Title != "" {
		b.WriteString(m.Styles.SectionTitle.Render(m.Title))
		b.WriteString("\n\n")
	}
	for i, field := range m.Fields {
		label := field.Label
		if field.Optional {
			label += m.Styles.Muted.Render(" (optional)")
		}
		if field.Secret {
			label += m.Styles.Muted.Render(" (hidden)")
		}
		labelStyle := m.Styles.InputLabel
		if i == m.focus {
			labelStyle = m.Styles.Info
		}
		b.WriteString(labelStyle.Render(label))
		b.WriteString("\n")
		b.WriteString(m.inputs[i].View())
		b.WriteString("\n")
		if field.Help != "" && (m.errs[i] == nil) {
			b.WriteString(m.Styles.Help.Render(field.Help))
			b.WriteString("\n")
		}
		if m.errs[i] != nil {
			b.WriteString(m.Styles.ErrorText.Render(m.errs[i].Error()))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	if m.submitErr != nil {
		b.WriteString(m.Styles.ErrorText.Render(m.submitErr.Error()))
		b.WriteString("\n\n")
	}
	b.WriteString(m.Styles.Help.Render("enter next/submit  ·  tab switch  ·  esc cancel"))
	return lipgloss.NewStyle().Width(min(m.width, 80)).Render(b.String())
}

// RunForm executes a form and returns its values. ErrCancelled is returned when
// the user presses escape.
func RunForm(ctx context.Context, form *Form) (map[string]string, error) {
	if form == nil {
		return nil, errors.New("form is nil")
	}
	program := tea.NewProgram(form, tea.WithContext(ctx))
	final, err := program.Run()
	if err != nil {
		return nil, err
	}
	result, ok := final.(*Form)
	if !ok {
		return nil, errors.New("unexpected prompt result")
	}
	if result.Cancelled() {
		return nil, ErrCancelled
	}
	if !result.Submitted() {
		return nil, errors.New("form did not complete")
	}
	return result.Values(), nil
}

// RunInput executes a single validated input and returns its value.
func RunInput(ctx context.Context, s styles.Styles, label, placeholder string, validate func(string) error) (string, error) {
	form := NewForm(s, "", []Field{{
		Key:         "value",
		Label:       label,
		Placeholder: placeholder,
		Validate:    validate,
	}})
	values, err := RunForm(ctx, form)
	if err != nil {
		return "", err
	}
	return values["value"], nil
}

// RunPassword executes a masked password prompt.
func RunPassword(ctx context.Context, s styles.Styles, label string, validate func(string) error) (string, error) {
	form := NewForm(s, "", []Field{{
		Key:      "password",
		Label:    label,
		Secret:   true,
		Validate: validate,
	}})
	values, err := RunForm(ctx, form)
	if err != nil {
		return "", err
	}
	return values["password"], nil
}

// RequireValue is a small required-field validator.
func RequireValue(label string) func(string) error {
	return func(value string) error {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", label)
		}
		return nil
	}
}
