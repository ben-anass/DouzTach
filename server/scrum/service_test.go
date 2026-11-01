package scrum

import (
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestValidateEpic(t *testing.T) {
	workspaceID := pgtype.UUID{Valid: true}
	actorID := pgtype.UUID{Valid: true}

	tests := []struct {
		name    string
		status  string
		start   pgtype.Date
		end     pgtype.Date
		wantErr bool
	}{
		{
			name:   "valid epic",
			status: "active",
		},
		{
			name:    "rejects invalid status",
			status:  "in_progress",
			wantErr: true,
		},
		{
			name:    "rejects reversed date range",
			status:  "planned",
			start:   testDate(2026, time.November, 2),
			end:     testDate(2026, time.November, 1),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateEpic(workspaceID, actorID, "Epic title", tt.status, "medium", "", tt.start, tt.end)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateEpic() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("validateEpic() error = %v, want ErrInvalidInput", err)
			}
		})
	}
}

func TestValidateStoryRejectsNegativePoints(t *testing.T) {
	err := validateStory(
		pgtype.UUID{Valid: true},
		pgtype.UUID{Valid: true},
		"Story",
		"to_do",
		"medium",
		pgtype.Int4{Int32: -1, Valid: true},
	)
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("validateStory() error = %v, want ErrInvalidInput", err)
	}
}

func TestValidateWorkItemHierarchy(t *testing.T) {
	workspaceID := pgtype.UUID{Valid: true}
	actorID := pgtype.UUID{Valid: true}

	tests := []struct {
		name     string
		itemType string
		parentID pgtype.UUID
		wantErr  bool
	}{
		{name: "task without parent", itemType: "task"},
		{name: "bug without parent", itemType: "bug"},
		{name: "subtask without parent", itemType: "subtask", wantErr: true},
		{name: "task with parent", itemType: "task", parentID: pgtype.UUID{Valid: true}, wantErr: true},
		{name: "subtask with parent", itemType: "subtask", parentID: pgtype.UUID{Valid: true}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateWorkItem(workspaceID, actorID, "Work item", tt.itemType, "to_do", "medium", pgtype.Int4{}, tt.parentID)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateWorkItem() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func testDate(year int, month time.Month, day int) pgtype.Date {
	return pgtype.Date{
		Time:  time.Date(year, month, day, 0, 0, 0, 0, time.UTC),
		Valid: true,
	}
}
