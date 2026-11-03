package tables

import (
	"strings"
	"testing"
	"time"

	"github.com/ben-anass/DouzTach/client/api"
	"github.com/ben-anass/DouzTach/client/terminal/styles"
	"github.com/charmbracelet/x/ansi"
)

func TestTaskColumnsAreResponsive(t *testing.T) {
	t.Parallel()
	widths := []int{120, 100, 90, 72, 55, 40, 20}
	last := 100
	for _, width := range widths {
		columns := TaskColumns(width)
		if len(columns) == 0 {
			t.Fatalf("width %d produced no columns", width)
		}
		if len(columns) > last {
			t.Fatalf("width %d produced more columns than a wider layout", width)
		}
		if columns[len(columns)-1].Title != "Title" && len(columns) == 1 {
			t.Fatalf("narrow layout should show the title, got %+v", columns)
		}
		last = len(columns)
	}
}

func TestWidthsNeverExceedAvailableWidth(t *testing.T) {
	t.Parallel()
	table := Table{Columns: TaskColumns(120)}
	for _, width := range []int{30, 36, 52, 70, 88, 104, 140} {
		widths := table.Widths(width)
		total := len(widths) - 1
		for _, w := range widths {
			if w < 3 {
				t.Errorf("width %d: column width %d is too small", width, w)
			}
			total += w
		}
		if width >= 30 && total > width {
			t.Errorf("width %d: table total %d exceeds terminal width", width, total)
		}
	}
}

func TestRenderTruncatesLongTitle(t *testing.T) {
	t.Parallel()
	s := styles.New(false)
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	task := api.Task{
		ID:       "task-1",
		Key:      "DT-1",
		Title:    strings.Repeat("very long title ", 20),
		Status:   api.TaskStatusInProgress,
		Priority: api.PriorityHigh,
		Type:     api.TaskTypeTask,
	}
	rendered := RenderTasks(s, []api.Task{task}, 70, now)
	for _, line := range strings.Split(rendered, "\n") {
		if w := ansi.StringWidth(line); w > 70 {
			t.Fatalf("line exceeds width 70 (%d): %q", w, line)
		}
	}
	if !strings.Contains(rendered, "IN_PROGRESS") {
		t.Fatalf("expected status label in output: %q", rendered)
	}
	if !strings.Contains(rendered, "DT-1") {
		t.Fatalf("expected task key in output: %q", rendered)
	}
}

func TestRenderTasksEmpty(t *testing.T) {
	t.Parallel()
	s := styles.New(true)
	if got := RenderTasks(s, nil, 80, time.Now()); got != "" {
		t.Fatalf("empty tasks should render nothing, got %q", got)
	}
}

func TestOverdueTaskIsMarked(t *testing.T) {
	t.Parallel()
	s := styles.New(true)
	past := time.Now().Add(-48 * time.Hour)
	task := api.Task{
		ID:       "t1",
		Title:    "Late task",
		Status:   api.TaskStatusTodo,
		Priority: api.PriorityUrgent,
		Type:     api.TaskTypeBug,
		DueDate:  &past,
	}
	rendered := RenderTasks(s, []api.Task{task}, 120, time.Now())
	if !strings.Contains(rendered, "!") {
		t.Fatalf("expected overdue marker: %q", rendered)
	}
}

func TestTaskTableStyledCellsDoNotPanic(t *testing.T) {
	t.Parallel()
	s := styles.New(false)
	assignee := &api.User{ID: "u1", Name: "Jane Doe"}
	task := api.Task{
		ID:       "t1",
		Title:    "Styled",
		Status:   api.TaskStatusDone,
		Priority: api.PriorityLow,
		Type:     api.TaskTypeStory,
		Assignee: assignee,
	}
	table := TaskTable(s, []api.Task{task}, 120, time.Now())
	if !table.PreStyled {
		t.Fatal("task table should use pre-styled cells")
	}
	if len(table.Rows) != 1 || len(table.Rows[0]) != len(table.Columns) {
		t.Fatalf("unexpected table shape: %+v", table.Rows)
	}
}

func TestSelectionRendersExactlyOneCursor(t *testing.T) {
	t.Parallel()
	s := styles.New(true)
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	tasks := []api.Task{
		{ID: "t1", Key: "DT-1", Title: "First", Status: api.TaskStatusTodo, Priority: api.PriorityLow, Type: api.TaskTypeTask},
		{ID: "t2", Key: "DT-2", Title: "Second", Status: api.TaskStatusTodo, Priority: api.PriorityLow, Type: api.TaskTypeTask},
	}

	without := RenderTasks(s, tasks, 100, now)
	if strings.Contains(without, "▸") {
		t.Fatalf("a table without selection must not render a cursor: %q", without)
	}

	withCursor := RenderSelectableTasks(s, tasks, 100, now, 1)
	if got := strings.Count(withCursor, "▸"); got != 1 {
		t.Fatalf("expected exactly one cursor marker, got %d: %q", got, withCursor)
	}
	if !strings.Contains(withCursor, "▸ DT-2") {
		t.Fatalf("the cursor should mark the second row: %q", withCursor)
	}
	for _, line := range strings.Split(withCursor, "\n") {
		if w := ansi.StringWidth(line); w > 100 {
			t.Errorf("line exceeds width 100 (%d): %q", w, line)
		}
	}
}

func TestSelectionKeepsRowsAligned(t *testing.T) {
	t.Parallel()
	s := styles.New(true)
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	tasks := []api.Task{
		{ID: "t1", Key: "DT-1", Title: "First", Status: api.TaskStatusTodo, Priority: api.PriorityLow, Type: api.TaskTypeTask},
		{ID: "t2", Key: "DT-2", Title: "Second", Status: api.TaskStatusTodo, Priority: api.PriorityLow, Type: api.TaskTypeTask},
	}
	for _, width := range []int{120, 90, 70, 52, 40} {
		rendered := RenderSelectableTasks(s, tasks, width, now, 0)
		lines := strings.Split(rendered, "\n")
		for _, line := range lines {
			if w := ansi.StringWidth(line); w > width {
				t.Errorf("width %d: line width %d exceeds the terminal: %q", width, w, line)
			}
		}
		// Header, divider, and rows must all start at the same column.
		for _, line := range lines[2:] {
			if ansi.StringWidth(line) < 4 {
				continue
			}
			if strings.HasPrefix(line, "▸ ") {
				continue
			}
			if !strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "▸ ") {
				t.Errorf("width %d: row does not start at the table gutter: %q", width, line)
			}
		}
	}
}

func TestTableWidthIncludesTheCursorGutter(t *testing.T) {
	t.Parallel()
	table := TaskTable(styles.New(true), []api.Task{{
		ID: "t1", Key: "DT-1", Title: "First", Status: api.TaskStatusTodo, Priority: api.PriorityLow, Type: api.TaskTypeTask,
	}}, 100, time.Now()).WithSelection(0)
	if got := table.Width(100); got > 100 {
		t.Fatalf("table width = %d, want <= 100", got)
	}
}

func TestTaskCountLabel(t *testing.T) {
	t.Parallel()
	if got := TaskCountLabel(1); got != "1 task" {
		t.Fatalf("TaskCountLabel(1) = %q", got)
	}
	if got := TaskCountLabel(3); got != "3 tasks" {
		t.Fatalf("TaskCountLabel(3) = %q", got)
	}
}
