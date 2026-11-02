package prompts

import (
	"context"
	"errors"
	"strings"

	"github.com/ben-anass/DouzTach/client/terminal/styles"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Option is a selectable value.
type Option struct {
	Label       string
	Value       string
	Description string
	Disabled    bool
}

// Select is a keyboard-driven single-choice prompt.
type Select struct {
	Title   string
	Options []Option
	Styles  styles.Styles

	cursor    int
	chosen    int
	cancelled bool
	width     int
}

// NewSelect builds a select prompt.
func NewSelect(s styles.Styles, title string, options []Option) *Select {
	return &Select{
		Title:   title,
		Options: options,
		Styles:  s,
		chosen:  -1,
		width:   80,
	}
}

// NewSelectSelected builds a select prompt whose cursor starts on the option
// matching value. It is used when editing an existing choice (a task type or a
// status) so the current value is preselected instead of the first option.
func NewSelectSelected(s styles.Styles, title string, options []Option, value string) *Select {
	prompt := NewSelect(s, title, options)
	prompt.SelectValue(value)
	return prompt
}

// SelectValue moves the cursor to the first enabled option with the given
// value. An unknown value leaves the cursor untouched.
func (m *Select) SelectValue(value string) {
	for i, option := range m.Options {
		if option.Disabled {
			continue
		}
		if option.Value == value {
			m.cursor = i
			return
		}
	}
}

// Init implements tea.Model.
func (m *Select) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (m *Select) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc", "q":
			m.cancelled = true
			return m, tea.Quit
		case "up", "k":
			m.move(-1)
		case "down", "j":
			m.move(1)
		case "home", "g":
			m.cursor = m.firstEnabled()
		case "end", "G":
			m.cursor = m.lastEnabled()
		case "enter", " ":
			if len(m.Options) > 0 && !m.Options[m.cursor].Disabled {
				m.chosen = m.cursor
				return m, tea.Quit
			}
		}
	}
	return m, nil
}

func (m *Select) move(delta int) {
	if len(m.Options) == 0 {
		return
	}
	for attempts := 0; attempts < len(m.Options); attempts++ {
		m.cursor += delta
		if m.cursor < 0 {
			m.cursor = len(m.Options) - 1
		}
		if m.cursor >= len(m.Options) {
			m.cursor = 0
		}
		if !m.Options[m.cursor].Disabled {
			return
		}
	}
}

func (m *Select) firstEnabled() int {
	for i, option := range m.Options {
		if !option.Disabled {
			return i
		}
	}
	return 0
}

func (m *Select) lastEnabled() int {
	for i := len(m.Options) - 1; i >= 0; i-- {
		if !m.Options[i].Disabled {
			return i
		}
	}
	return max(0, len(m.Options)-1)
}

// Selected returns the chosen option when the prompt completed.
func (m *Select) Selected() (Option, bool) {
	if m.chosen < 0 || m.chosen >= len(m.Options) {
		return Option{}, false
	}
	return m.Options[m.chosen], true
}

// Cancelled reports whether the user aborted the prompt.
func (m *Select) Cancelled() bool { return m.cancelled }

// View implements tea.Model.
func (m *Select) View() string {
	var b strings.Builder
	if m.Title != "" {
		b.WriteString(m.Styles.SectionTitle.Render(m.Title))
		b.WriteString("\n\n")
	}
	if len(m.Options) == 0 {
		b.WriteString(m.Styles.Muted.Render("No options available."))
		return b.String()
	}
	start, end := m.visibleRange()
	if start > 0 {
		b.WriteString(m.Styles.Muted.Render("  ↑ more"))
		b.WriteString("\n")
	}
	for i := start; i < end; i++ {
		option := m.Options[i]
		label := option.Label
		if option.Disabled {
			label += " (unavailable)"
		}
		switch {
		case option.Disabled:
			b.WriteString(m.Styles.Muted.Render("  " + label))
		case i == m.cursor:
			b.WriteString(m.Styles.Selected.Render("› " + label))
		default:
			b.WriteString(m.Styles.Text.Render("  " + label))
		}
		if option.Description != "" {
			b.WriteString("\n")
			b.WriteString(m.Styles.Muted.Render("    " + option.Description))
		}
		b.WriteString("\n")
	}
	if end < len(m.Options) {
		b.WriteString(m.Styles.Muted.Render("  ↓ more"))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(m.Styles.Help.Render("↑/↓ move  ·  enter select  ·  esc cancel"))
	return lipgloss.NewStyle().Width(min(m.width, 80)).Render(b.String())
}

func (m *Select) visibleRange() (int, int) {
	const maxVisible = 10
	if len(m.Options) <= maxVisible {
		return 0, len(m.Options)
	}
	start := m.cursor - maxVisible/2
	if start < 0 {
		start = 0
	}
	if start+maxVisible > len(m.Options) {
		start = len(m.Options) - maxVisible
	}
	return start, start + maxVisible
}

// RunSelect executes a select prompt.
func RunSelect(ctx context.Context, prompt *Select) (Option, error) {
	if prompt == nil {
		return Option{}, errors.New("select prompt is nil")
	}
	if len(prompt.Options) == 0 {
		return Option{}, errors.New("no options available")
	}
	program := tea.NewProgram(prompt, tea.WithContext(ctx))
	final, err := program.Run()
	if err != nil {
		return Option{}, err
	}
	result, ok := final.(*Select)
	if !ok {
		return Option{}, errors.New("unexpected prompt result")
	}
	if result.Cancelled() {
		return Option{}, ErrCancelled
	}
	option, ok := result.Selected()
	if !ok {
		return Option{}, errors.New("no option selected")
	}
	return option, nil
}
