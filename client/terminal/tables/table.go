// Package tables provides reusable, terminal-width-aware tables. Cells are
// rendered as plain strings (they may contain ANSI styling) and are truncated
// to their column width so the table never wraps unpredictably.
package tables

import (
	"strings"

	"github.com/ben-anass/DouzTach/client/terminal/components"
	"github.com/ben-anass/DouzTach/client/terminal/styles"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Column describes one table column.
type Column struct {
	Title    string
	Weight   int
	MinWidth int
	MaxWidth int
	Align    lipgloss.Position
}

// Table is a collection of columns and pre-rendered row cells.
type Table struct {
	Columns []Column
	Rows    [][]string
	// PreStyled means row cells already contain ANSI styling; the table will
	// not wrap them in the default text style.
	PreStyled bool

	// hasSelection and selected drive the optional cursor gutter. The zero
	// value means "no selection", so callers opt in with WithSelection.
	hasSelection bool
	selected     int
}

// WithSelection highlights one row with a cursor marker. Interactive screens
// use it so keyboard navigation is visible; a negative index clears it.
func (t Table) WithSelection(index int) Table {
	t.hasSelection = index >= 0
	t.selected = index
	return t
}

// gutterWidth returns the number of cells reserved for the cursor marker.
func (t Table) gutterWidth() int {
	if t.hasSelection && len(t.Rows) > 0 {
		return 2
	}
	return 0
}

// Widths computes the column widths for the available terminal width (the
// cursor gutter is not included; use Render for the full layout). It never
// returns a width below 3 cells, and it keeps the total (including single-space
// gutters) within width whenever width is large enough.
func (t Table) Widths(width int) []int {
	n := len(t.Columns)
	if n == 0 {
		return nil
	}
	widths := make([]int, n)
	if width <= 0 {
		for i, col := range t.Columns {
			widths[i] = max(3, col.MinWidth)
		}
		return widths
	}

	gutters := n - 1
	available := width - gutters
	if available < n*3 {
		// Extremely narrow terminal: divide evenly with a floor of 3. The
		// rendered table may exceed the terminal, but content stays readable.
		each := max(3, available/n)
		for i := range widths {
			widths[i] = each
		}
		return widths
	}

	minTotal := 0
	weightTotal := 0
	for i, col := range t.Columns {
		minimum := max(3, col.MinWidth)
		widths[i] = minimum
		minTotal += minimum
		if col.Weight > 0 {
			weightTotal += col.Weight
		}
	}

	if available <= minTotal {
		// Shrink proportionally to minimums.
		remaining := available
		for i := range widths {
			widths[i] = max(3, widths[i]*available/minTotal)
			remaining -= widths[i]
		}
		for i := 0; remaining > 0 && i < n; i++ {
			widths[i]++
			remaining--
		}
		for remaining < 0 {
			for i := n - 1; remaining < 0 && i >= 0; i-- {
				if widths[i] > 3 {
					widths[i]--
					remaining++
				}
			}
		}
		return widths
	}

	extra := available - minTotal
	if weightTotal == 0 {
		// No flexible columns: distribute evenly.
		for i := range widths {
			widths[i] += extra / n
		}
		for i := 0; i < extra%n; i++ {
			widths[i]++
		}
		return widths
	}

	// Distribute extra cells by weight, respecting MaxWidth.
	for pass := 0; pass < 2 && extra > 0; pass++ {
		distributed := 0
		for i, col := range t.Columns {
			if col.Weight <= 0 {
				continue
			}
			share := extra * col.Weight / weightTotal
			if share == 0 {
				continue
			}
			if col.MaxWidth > 0 && widths[i]+share > col.MaxWidth {
				share = max(0, col.MaxWidth-widths[i])
			}
			widths[i] += share
			distributed += share
		}
		extra -= distributed
		if distributed == 0 {
			break
		}
	}
	for i := 0; extra > 0 && i < n; i++ {
		if t.Columns[i].Weight <= 0 {
			continue
		}
		if t.Columns[i].MaxWidth > 0 && widths[i] >= t.Columns[i].MaxWidth {
			continue
		}
		widths[i]++
		extra--
	}
	return widths
}

// Render draws the header, divider, and rows. An empty table renders only its
// header; callers should show an empty state for zero rows.
func (t Table) Render(s styles.Styles, width int) string {
	if len(t.Columns) == 0 {
		return ""
	}
	gutter := t.gutterWidth()
	tableWidth := width
	if width > 0 && gutter > 0 {
		tableWidth = max(3*len(t.Columns), width-gutter)
	}
	widths := t.Widths(tableWidth)

	gap := strings.Repeat(" ", gutter)
	header := gap + t.renderRow(s, widths, columnTitles(t.Columns), true)
	dividerWidth := gutter
	for _, w := range widths {
		dividerWidth += w
	}
	dividerWidth += len(widths) - 1
	if width > 0 {
		dividerWidth = min(dividerWidth, width)
	}
	lines := []string{header, s.DividerString(max(1, dividerWidth))}
	for i, row := range t.Rows {
		marker := gap
		if gutter > 0 && i == t.selected {
			marker = s.Info.Render("▸") + " "
		}
		lines = append(lines, marker+t.renderRow(s, widths, row, false))
	}
	return strings.Join(lines, "\n")
}

func (t Table) renderRow(s styles.Styles, widths []int, cells []string, header bool) string {
	parts := make([]string, len(widths))
	for i, width := range widths {
		value := ""
		if i < len(cells) {
			value = cells[i]
		}
		value = components.Truncate(value, width)
		style := s.Text
		if header {
			style = s.CardTitle
		}
		if t.Columns[i].Align == lipgloss.Right {
			value = components.PadLeft(value, width)
		} else {
			value = components.PadRight(value, width)
		}
		if t.PreStyled && !header {
			parts[i] = value
			continue
		}
		parts[i] = style.Render(value)
	}
	return strings.Join(parts, " ")
}

func columnTitles(columns []Column) []string {
	titles := make([]string, len(columns))
	for i, col := range columns {
		titles[i] = col.Title
	}
	return titles
}

// Width returns the rendered width of a table for the given terminal width.
func (t Table) Width(width int) int {
	gutter := t.gutterWidth()
	tableWidth := width
	if width > 0 && gutter > 0 {
		tableWidth = max(3*len(t.Columns), width-gutter)
	}
	widths := t.Widths(tableWidth)
	total := gutter + len(widths) - 1
	for _, w := range widths {
		total += w
	}
	if width > 0 && total > width {
		return width
	}
	return total
}

// StringWidth is a small helper exposed for tests and callers that need to
// measure rendered ANSI content.
func StringWidth(value string) int { return ansi.StringWidth(value) }
