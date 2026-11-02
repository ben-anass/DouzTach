package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMiddlewareWithoutToken(t *testing.T) {
	jwtService := NewJWTService("test-secret-key-for-unit-tests")

	protectedHandler := jwtService.Middleware(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	recorder := httptest.NewRecorder()

	protectedHandler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Errorf(
			"expected status %d, got %d",
			http.StatusUnauthorized,
			recorder.Code,
		)
	}
}

func TestMiddlewareWithInvalidToken(t *testing.T) {
	jwtService := NewJWTService("test-secret-key-for-unit-tests")

	protectedHandler := jwtService.Middleware(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	request.Header.Set("Authorization", "Bearer invalid-token")

	recorder := httptest.NewRecorder()

	protectedHandler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Errorf(
			"expected status %d, got %d",
			http.StatusUnauthorized,
			recorder.Code,
		)
	}
}

func TestMiddlewareWithValidToken(t *testing.T) {
	jwtService := NewJWTService("test-secret-key-for-unit-tests")

	expectedUserID := "user-123"

	token, err := jwtService.GenerateToken(expectedUserID)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	protectedHandler := jwtService.Middleware(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := UserIDFromContext(r.Context())

			if !ok {
				t.Error("user ID not found in context")
				return
			}

			if userID != expectedUserID {
				t.Errorf("expected user ID %s, got %s", expectedUserID, userID)
			}

			w.WriteHeader(http.StatusOK)
		}),
	)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	request.Header.Set("Authorization", "Bearer "+token)

	recorder := httptest.NewRecorder()

	protectedHandler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
}
