package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ben-anass/DouzTach/internal/auth"
	"github.com/ben-anass/DouzTach/server/activity"
)

func TestScrumHandlersRejectMalformedOptionalValues(t *testing.T) {
	jwtService := auth.NewJWTService("test-secret-key-that-is-at-least-32-bytes")
	token, err := jwtService.GenerateToken("00000000-0000-0000-0000-000000000001")
	if err != nil {
		t.Fatal(err)
	}
	handler := NewScrumHandler(nil)

	tests := []struct {
		name   string
		path   string
		body   string
		handle http.HandlerFunc
	}{
		{
			name:   "epic date",
			path:   "/api/v1/workspaces/workspace-1/epics",
			body:   `{"title":"Epic","start_date":"not-a-date"}`,
			handle: handler.CreateEpic,
		},
		{
			name:   "story reference",
			path:   "/api/v1/workspaces/workspace-1/stories",
			body:   `{"title":"Story","epic_id":"not-a-uuid"}`,
			handle: handler.CreateUserStory,
		},
		{
			name:   "task reference",
			path:   "/api/v1/workspaces/workspace-1/tasks",
			body:   `{"title":"Task","item_type":"task","assignee_id":"not-a-uuid"}`,
			handle: handler.CreateWorkItem,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, tt.path, strings.NewReader(tt.body))
			request.Header.Set("Authorization", "Bearer "+token)
			response := httptest.NewRecorder()
			jwtService.Middleware(tt.handle).ServeHTTP(response, request)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusBadRequest, response.Body)
			}
		})
	}
}

func TestScrumReadOnlyRoleErrorMapsToForbidden(t *testing.T) {
	response := httptest.NewRecorder()
	writeScrumError(response, activity.ErrReadOnly)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusForbidden)
	}
}
