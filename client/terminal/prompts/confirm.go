package prompts

import (
	"context"
	"errors"
	"strings"

	"github.com/ben-anass/DouzTach/client/terminal/styles"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Confirm is a yes/no prompt. A destructive confirmation only accepts an
// explicit "y" so a stray enter cannot delete data.
type Confirm struct {
	Title       string
	Question    string
	Detail      string
	Destructive bool
	Styles      styles.Styles

	confirmed bool
	cancelled bool
	width     int
}

// NewConfirm builds a confirmation prompt.
func NewConfirm(s styles.Styles, title, question string) *Confirm {
	return &Confirm{
		Title:    title,
		Question: question,
		Styles:   s,
		width:    80,
	}
}

// Init implements tea.Model.
func (m *Confirm) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (m *Confirm) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil
	case tea.KeyMsg:
		switch strings.ToLower(msg.String()) {
		case "y", "yes":
			m.confirmed = true
			return m, tea.Quit
		case "n", "no", "esc", "ctrl+c", "q":
			m.cancelled = true
			return m, tea.Quit
		case "enter":
			if !m.Destructive {
				m.confirmed = true
				return m, tea.Quit
			}
		}
	}
	return m, nil
}

// Confirmed reports whether the user accepted.
func (m *Confirm) Confirmed() bool { return m.confirmed }

// Cancelled reports whether the user declined or aborted.
func (m *Confirm) Cancelled() bool { return m.cancelled }

// View implements tea.Model.
func (m *Confirm) View() string {
	var b strings.Builder
	titleStyle := m.Styles.PanelTitle
	if m.Destructive {
		titleStyle = m.Styles.Error
	}
	b.WriteString(titleStyle.Render(m.Title))
	b.WriteString("\n\n")
	b.WriteString(m.Styles.Text.Render(m.Question))
	if detail := strings.TrimSpace(m.Detail); detail != "" {
		b.WriteString("\n\n")
		b.WriteString(m.Styles.Muted.Render(detail))
	}
	b.WriteString("\n\n")
	confirmStyle := m.Styles.Success
	if m.Destructive {
		confirmStyle = m.Styles.Error
	}
	b.WriteString(confirmStyle.Render("[y] Confirm"))
	b.WriteString(m.Styles.Muted.Render("    [n] Cancel"))
	b.WriteString("\n\n")
	b.WriteString(m.Styles.Help.Render("Only 'y' confirms this action. Esc cancels."))
	return lipgloss.NewStyle().Width(min(m.width, 80)).Render(b.String())
}

// RunConfirm executes a confirmation prompt.
func RunConfirm(ctx context.Context, prompt *Confirm) (bool, error) {
	if prompt == nil {
		return false, errors.New("confirm prompt is nil")
	}
	program := tea.NewProgram(prompt, tea.WithContext(ctx))
	final, err := program.Run()
	if err != nil {
		return false, err
	}
	result, ok := final.(*Confirm)
	if !ok {
		return false, errors.New("unexpected prompt result")
	}
	return result.Confirmed(), nil
}
