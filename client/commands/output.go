package commands

import (
	"fmt"
	"strings"
	"text/tabwriter"
	"time"
	"unicode/utf8"

	"github.com/ben-anass/DouzTach/client/terminal/components"
)

// title prints a section heading.
func (a *App) title(text string) {
	fmt.Fprintln(a.Out, a.Styles.SectionTitle.Render(text))
}

// muted prints secondary information.
func (a *App) muted(format string, args ...any) {
	fmt.Fprintln(a.Out, a.Styles.Muted.Render(fmt.Sprintf(format, args...)))
}

// success prints a confirmation.
func (a *App) success(format string, args ...any) {
	fmt.Fprintln(a.Out, a.Styles.Success.Render(fmt.Sprintf(format, args...)))
}

// warn prints a warning to stderr so it never contaminates piped stdout.
func (a *App) warn(format string, args ...any) {
	fmt.Fprintln(a.Err, a.Styles.Warning.Render("warning: "+fmt.Sprintf(format, args...)))
}

// table renders aligned columns. Cells must be plain text: ANSI styling would
// break tabwriter's width calculation.
func (a *App) table(headers []string, rows [][]string) {
	writer := tabwriter.NewWriter(a.Out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(writer, strings.Join(headers, "\t"))
	separators := make([]string, len(headers))
	for i, header := range headers {
		separators[i] = strings.Repeat("-", max(3, utf8.RuneCountInString(header)))
	}
	fmt.Fprintln(writer, strings.Join(separators, "\t"))
	for _, row := range rows {
		fmt.Fprintln(writer, strings.Join(row, "\t"))
	}
	_ = writer.Flush()
}

// keyValues prints an aligned label/value block.
func (a *App) keyValues(rows [][2]string, labelWidth int) {
	for _, row := range rows {
		label := a.Styles.Muted.Render(components.PadRight(row[0], labelWidth))
		fmt.Fprintln(a.Out, label+a.Styles.Text.Render(row[1]))
	}
}

// spacer prints a blank line between sections. Use it instead of a leading
// newline inside a styled string, which would be padded to the block width.
func (a *App) spacer() {
	fmt.Fprintln(a.Out)
}

// empty prints a neutral empty-collection message.
func (a *App) empty(message string) {
	fmt.Fprintln(a.Out, a.Styles.Muted.Render(message))
}

func cell(value string, limit int) string {
	value = strings.Join(strings.Fields(value), " ")
	if limit > 0 {
		value = components.Truncate(value, limit)
	}
	return value
}

func dash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "—"
	}
	return value
}

func formatDate(value *time.Time) string {
	if value == nil {
		return "—"
	}
	return value.Format("2006-01-02")
}

func formatTimestamp(value time.Time) string {
	if value.IsZero() {
		return "—"
	}
	return value.Format("2006-01-02 15:04")
}

// relativeTime renders a coarse "5m ago" style timestamp for tables.
func relativeTime(value, now time.Time) string {
	if value.IsZero() {
		return "—"
	}
	return components.RelativeTime(value, now)
}

func progressText(done, total int) string {
	if total <= 0 {
		return "—"
	}
	return fmt.Sprintf("%d/%d (%d%%)", done, total, int(float64(done)/float64(total)*100+0.5))
}
