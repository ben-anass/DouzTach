// Package components contains reusable terminal UI building blocks shared by
// the dashboard, tables, prompts, and screens.
package components

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"
)

// Truncate shortens a string to width terminal cells. It is ANSI-aware and
// accounts for wide characters.
func Truncate(value string, width int) string {
	if width <= 0 {
		return ""
	}
	return ansi.Truncate(value, width, "…")
}

// PadRight pads a string with spaces to width terminal cells.
func PadRight(value string, width int) string {
	current := ansi.StringWidth(value)
	if current >= width {
		return value
	}
	return value + strings.Repeat(" ", width-current)
}

// PadLeft pads a string with spaces on the left to width terminal cells.
func PadLeft(value string, width int) string {
	current := ansi.StringWidth(value)
	if current >= width {
		return value
	}
	return strings.Repeat(" ", width-current) + value
}

// RightAlign places left and right on the same line, separated by spaces. If
// they do not fit, left is truncated and right is preserved.
func RightAlign(left, right string, width int) string {
	if width <= 0 {
		return ""
	}
	rightWidth := ansi.StringWidth(right)
	leftWidth := ansi.StringWidth(left)
	if rightWidth >= width {
		return Truncate(right, width)
	}
	if leftWidth+rightWidth+1 > width {
		left = Truncate(left, width-rightWidth-1)
		leftWidth = ansi.StringWidth(left)
	}
	gap := width - leftWidth - rightWidth
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

// Wrap wraps text to width, preserving ANSI sequences.
func Wrap(value string, width int) string {
	if width <= 0 {
		return value
	}
	return ansi.Wrap(value, width, " -/")
}

// Clamp limits rendered content to width and height. If lines are removed, the
// last visible line receives an ellipsis.
func Clamp(value string, width, height int) string {
	if height <= 0 {
		return ""
	}
	lines := strings.Split(value, "\n")
	truncated := len(lines) > height
	if truncated {
		lines = lines[:height]
	}
	for i := range lines {
		if width > 0 {
			lines[i] = ansi.Truncate(lines[i], width, "…")
		}
	}
	if truncated && len(lines) > 0 {
		lines[len(lines)-1] = ansi.Truncate(lines[len(lines)-1], max(1, width-1), "") + "…"
	}
	return strings.Join(lines, "\n")
}

// FormatDate renders a date pointer compactly.
func FormatDate(value *time.Time) string {
	if value == nil || value.IsZero() {
		return "—"
	}
	return value.Format("Jan 2, 2006")
}

// FormatDateTime renders a timestamp compactly.
func FormatDateTime(value time.Time) string {
	if value.IsZero() {
		return "—"
	}
	return value.Format("Jan 2, 2006 15:04")
}

// RelativeTime renders a coarse relative timestamp such as "5m ago".
func RelativeTime(value, now time.Time) string {
	if value.IsZero() {
		return "—"
	}
	if value.After(now) {
		remaining := value.Sub(now)
		if remaining < time.Minute {
			return "in a moment"
		}
		if remaining < time.Hour {
			return fmt.Sprintf("in %dm", int(remaining.Minutes()))
		}
		if remaining < 24*time.Hour {
			return fmt.Sprintf("in %dh", int(remaining.Hours()))
		}
		return "in " + fmt.Sprintf("%dd", int(remaining.Hours()/24))
	}
	elapsed := now.Sub(value)
	switch {
	case elapsed < time.Minute:
		return "just now"
	case elapsed < time.Hour:
		return fmt.Sprintf("%dm ago", int(elapsed.Minutes()))
	case elapsed < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(elapsed.Hours()))
	case elapsed < 7*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(elapsed.Hours()/24))
	default:
		return value.Format("Jan 2")
	}
}

// Initials extracts up to two initials from a display name.
func Initials(name string) string {
	fields := strings.Fields(strings.TrimSpace(name))
	if len(fields) == 0 {
		return "?"
	}
	first := []rune(fields[0])
	if len(first) == 0 {
		return "?"
	}
	if len(fields) == 1 {
		return strings.ToUpper(string(first[0]))
	}
	second := []rune(fields[len(fields)-1])
	if len(second) == 0 {
		return strings.ToUpper(string(first[0]))
	}
	return strings.ToUpper(string(first[0]) + string(second[0]))
}

// Plural returns singular when n == 1, otherwise plural.
func Plural(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}

// IntString formats an integer without importing strconv at call sites.
func IntString(n int) string { return strconv.Itoa(n) }
