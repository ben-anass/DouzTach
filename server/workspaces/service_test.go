package workspaces

import (
	"context"
	"testing"
)

func TestCreateWorkspaceValidation(t *testing.T) {
	service := &Service{}

	tests := []struct {
		name      string
		ownerID   string
		workspace string
		kind      string
		wantError string
	}{
		{
			name:      "empty workspace name",
			ownerID:   "033dc0e5-b1be-4b63-af96-a74ae5595ebf",
			workspace: "",
			kind:      "team",
			wantError: "workspace name is required",
		},
		{
			name:      "invalid workspace kind",
			ownerID:   "033dc0e5-b1be-4b63-af96-a74ae5595ebf",
			workspace: "My Team",
			kind:      "invalid",
			wantError: "invalid workspace kind",
		},
		{
			name:      "invalid owner ID",
			ownerID:   "invalid-id",
			workspace: "My Team",
			kind:      "team",
			wantError: "invalid owner user ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := service.CreateWorkspace(
				context.Background(),
				tt.ownerID,
				tt.workspace,
				"",
				"",
				tt.kind,
			)

			if err == nil {
				t.Fatal("expected an error, got nil")
			}

			if err.Error() != tt.wantError {
				t.Fatalf("expected error %q, got %q", tt.wantError, err.Error())
			}
		})
	}
}
