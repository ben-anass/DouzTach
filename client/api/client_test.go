package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestNewClientValidation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		baseURL string
		wantErr bool
	}{
		{name: "empty", baseURL: "", wantErr: true},
		{name: "bad scheme", baseURL: "ftp://example.com", wantErr: true},
		{name: "credentials", baseURL: "https://user:pass@example.com", wantErr: true},
		{name: "query", baseURL: "https://example.com?x=1", wantErr: true},
		{name: "valid http", baseURL: "http://localhost:8080", wantErr: false},
		{name: "valid with prefix", baseURL: "https://example.com/api/v1", wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			client, err := NewClient(tt.baseURL)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got client %v", client)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if client.BaseURL() == "" {
				t.Fatal("expected non-empty base URL")
			}
		})
	}
}

func TestClientGetWithAuthAndQuery(t *testing.T) {
	t.Parallel()
	var gotPath, gotQuery, gotAuth, gotAgent string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		gotAuth = r.Header.Get("Authorization")
		gotAgent = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"t1","title":"Write tests"}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL+"/api/v1",
		WithTokenProvider(func() string { return "secret-token" }),
		WithUserAgent("douztech-test"),
	)
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	}
	query := url.Values{"status": {"TODO"}}
	if err := client.Get(context.Background(), "/tasks", query, &out); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if gotPath != "/api/v1/tasks" {
		t.Fatalf("path = %q, want /api/v1/tasks", gotPath)
	}
	if gotQuery != "status=TODO" {
		t.Fatalf("query = %q", gotQuery)
	}
	if gotAuth != "Bearer secret-token" {
		t.Fatalf("auth = %q", gotAuth)
	}
	if gotAgent != "douztech-test" {
		t.Fatalf("user agent = %q", gotAgent)
	}
	if out.ID != "t1" || out.Title != "Write tests" {
		t.Fatalf("unexpected response: %+v", out)
	}
}

func TestClientPostJSON(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("content type = %q", ct)
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode body: %v", err)
		}
		if body["title"] != "New task" {
			t.Errorf("body = %+v", body)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"task-1"}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		ID string `json:"id"`
	}
	err = client.Post(context.Background(), "/tasks", map[string]string{"title": "New task"}, &out)
	if err != nil {
		t.Fatalf("Post: %v", err)
	}
	if out.ID != "task-1" {
		t.Fatalf("id = %q", out.ID)
	}
}

func TestClientStructuredError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-ID", "req-42")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"code":"INVALID_CREDENTIALS","message":"Email or password is incorrect"}}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	err = client.Get(context.Background(), "/auth/me", nil, &struct{}{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !IsUnauthorized(err) {
		t.Fatalf("expected unauthorized, got %v", err)
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.Code != "INVALID_CREDENTIALS" {
		t.Errorf("code = %q", apiErr.Code)
	}
	if apiErr.Message != "Email or password is incorrect" {
		t.Errorf("message = %q", apiErr.Message)
	}
	if apiErr.RequestID != "req-42" {
		t.Errorf("request id = %q", apiErr.RequestID)
	}
	if strings.Contains(err.Error(), "secret") {
		t.Errorf("error leaked unexpected content: %v", err)
	}
}

func TestClientPlainTextError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("database is unavailable"))
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	err = client.Get(context.Background(), "/tasks", nil, &struct{}{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !IsServerError(err) {
		t.Fatalf("expected server error, got %v", err)
	}
	if !strings.Contains(err.Error(), "database is unavailable") {
		t.Fatalf("missing server message: %v", err)
	}
}

func TestClientEmptyAndNoContentResponses(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/empty":
			w.WriteHeader(http.StatusOK)
		case "/no-content":
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Get(context.Background(), "/empty", nil, &struct{}{}); !errors.Is(err, ErrEmptyResponse) {
		t.Fatalf("expected ErrEmptyResponse, got %v", err)
	}
	if err := client.Get(context.Background(), "/no-content", nil, &struct{}{}); err != nil {
		t.Fatalf("no content should succeed: %v", err)
	}
}

func TestClientResponseLimit(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", 100)))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, WithMaxResponseBytes(10))
	if err != nil {
		t.Fatal(err)
	}
	err = client.Get(context.Background(), "/large", nil, &struct{}{})
	if !errors.Is(err, ErrResponseTooLarge) {
		t.Fatalf("expected ErrResponseTooLarge, got %v", err)
	}
}

func TestClientTimeout(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, WithTimeout(20*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err = client.Get(ctx, "/slow", nil, &struct{}{})
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestClientRejectsUnsafePaths(t *testing.T) {
	t.Parallel()
	client, err := NewClient("https://example.com/api")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"", "https://evil.example/x", "//evil.example/x", "/../admin", "/tasks?x=1", "/tasks#frag"} {
		if err := client.Get(context.Background(), path, nil, nil); err == nil {
			t.Errorf("path %q should be rejected", path)
		}
	}
}

func TestClientPing(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Ping(context.Background())
	if err != nil {
		t.Fatalf("Ping: %v", err)
	}
	if result.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", result.StatusCode)
	}
}

func TestClientRejectsTrailingJSON(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"a":1}{"b":2}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := client.Get(context.Background(), "/trailing", nil, &out); err == nil {
		t.Fatal("expected trailing JSON error")
	}
}

func TestServiceStatusSummary(t *testing.T) {
	t.Parallel()
	tests := []struct {
		status ServiceStatus
		want   string
	}{
		{ServiceStatus{}, "Not configured"},
		{ServiceStatus{Configured: true}, "API unreachable"},
		{ServiceStatus{Configured: true, Reachable: true}, "API reachable, contract pending"},
		{ServiceStatus{Configured: true, Reachable: true, ContractReady: true}, "Connected"},
		{ServiceStatus{Configured: true, ContractReady: true}, "Contract ready, API unreachable"},
	}
	for _, tt := range tests {
		if got := tt.status.Summary(); got != tt.want {
			t.Errorf("Summary() = %q, want %q", got, tt.want)
		}
	}
}

func TestPendingServicesReturnContractError(t *testing.T) {
	t.Parallel()
	services := newPendingServices(nil)
	if services.Ready() {
		t.Fatal("pending services must not be ready")
	}
	_, err := services.Auth.Login(context.Background(), LoginRequest{})
	if !IsContractPending(err) {
		t.Fatalf("expected contract pending, got %v", err)
	}
	_, err = services.Tasks.List(context.Background(), TaskFilter{})
	if !IsContractPending(err) {
		t.Fatalf("expected contract pending, got %v", err)
	}
	if err := services.RequireReady(); !IsContractPending(err) {
		t.Fatalf("RequireReady: %v", err)
	}
}

func TestNewServicesWithoutURL(t *testing.T) {
	t.Parallel()
	services, err := NewServices("", time.Second, nil)
	if err != nil {
		t.Fatalf("NewServices: %v", err)
	}
	if services.Status.Configured {
		t.Fatal("expected not configured")
	}
	if services.Ready() {
		t.Fatal("expected contract pending")
	}
}

func TestNewServicesWithURL(t *testing.T) {
	t.Parallel()
	services, err := NewServices("http://localhost:8080", time.Second, func() string { return "token" })
	if err != nil {
		t.Fatalf("NewServices: %v", err)
	}
	if !services.Status.Configured {
		t.Fatal("expected configured")
	}
	if services.Ready() {
		t.Fatal("contract must remain pending until implemented")
	}
	if services.Client == nil {
		t.Fatal("expected client")
	}
}

func ExampleAPIError() {
	err := &APIError{StatusCode: 404, Method: "GET", Path: "/tasks/1", Message: "task not found"}
	fmt.Println(err)
	// Output: GET /tasks/1: 404 Not Found: task not found
}
