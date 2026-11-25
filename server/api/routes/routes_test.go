package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ben-anass/DouzTach/internal/auth"
	"github.com/ben-anass/DouzTach/server/api/handlers"
	"github.com/ben-anass/DouzTach/server/users"
)

func TestProtectedScrumRoutesRejectMissingAuth(t *testing.T) {
	jwtService := auth.NewJWTService("test-secret-key-that-is-at-least-32-bytes")
	router := NewRouter(
		&handlers.AuthHandler{},
		&handlers.WorkspaceHandler{},
		&handlers.ScrumHandler{},
		&handlers.ActivityHandler{},
		&handlers.NotificationHandler{},
		&handlers.ChatHandler{},
		jwtService,
		&users.Service{},
	)

	tests := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/workspaces/00000000-0000-0000-0000-000000000001/epics"},
		{http.MethodPost, "/api/v1/workspaces/00000000-0000-0000-0000-000000000001/stories"},
		{http.MethodPut, "/api/v1/workspaces/00000000-0000-0000-0000-000000000001/stories/00000000-0000-0000-0000-000000000002"},
		{http.MethodPost, "/api/v1/workspaces/00000000-0000-0000-0000-000000000001/tasks"},
		{http.MethodPut, "/api/v1/workspaces/00000000-0000-0000-0000-000000000001/tasks/00000000-0000-0000-0000-000000000002"},
		{http.MethodPost, "/api/v1/workspaces/00000000-0000-0000-0000-000000000001/sprints/00000000-0000-0000-0000-000000000002/start"},
		{http.MethodGet, "/api/v1/workspaces/00000000-0000-0000-0000-000000000001/activity"},
		{http.MethodGet, "/api/v1/notifications"},
		{http.MethodPost, "/api/v1/notifications/read-all"},
		{http.MethodGet, "/api/v1/workspaces/00000000-0000-0000-0000-000000000001/chat/channels"},
		{http.MethodPost, "/api/v1/workspaces/00000000-0000-0000-0000-000000000001/chat/channels/00000000-0000-0000-0000-000000000002/messages"},
	}
	for _, test := range tests {
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.path, nil)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("%s %s status = %d, want %d", test.method, test.path, response.Code, http.StatusUnauthorized)
			}
		})
	}
}
