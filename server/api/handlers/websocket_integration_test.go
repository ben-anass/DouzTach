package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ben-anass/DouzTach/internal/auth"
	"github.com/ben-anass/DouzTach/internal/database"
	"github.com/ben-anass/DouzTach/server/chat"
	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/joho/godotenv"
	"golang.org/x/net/websocket"
)

func TestWebSocketHTTPIntegration(t *testing.T) {
	if os.Getenv("DOUZTACH_WEBSOCKET_E2E") != "1" {
		t.Skip("set DOUZTACH_WEBSOCKET_E2E=1 to run against the isolated PostgreSQL test database")
	}
	if !strings.HasPrefix(os.Getenv("POSTGRES_DB"), "douztach_e2e_") {
		t.Fatal("WebSocket integration tests require an isolated POSTGRES_DB named douztach_e2e_*")
	}
	if err := godotenv.Load(filepath.Join("..", "..", "..", ".env")); err != nil && os.Getenv("POSTGRES_USER") == "" {
		t.Fatalf("load test database environment: %v", err)
	}

	workspaceID := os.Getenv("DOUZTACH_E2E_WORKSPACE_ID")
	channelID := os.Getenv("DOUZTACH_E2E_CHANNEL_ID")
	token := os.Getenv("DOUZTACH_E2E_TOKEN")
	if workspaceID == "" || channelID == "" || token == "" {
		t.Fatal("workspace, channel, and token E2E variables are required")
	}
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "default_jwt_secret_key_change_in_production"
	}
	jwtService := auth.NewJWTService(secret)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	pool, err := database.NewPool(ctx)
	if err != nil {
		t.Fatalf("connect isolated database: %v", err)
	}
	defer pool.Close()

	handler := NewChatHandler(chat.NewService(pool))
	router := chi.NewRouter()
	router.Use(jwtService.Middleware)
	router.Handle("/api/v1/workspaces/{workspaceID}/chat/ws", handler.WebSocketHandler())
	router.Post("/api/v1/workspaces/{workspaceID}/chat/channels/{channelID}/messages", handler.SendMessage)
	router.Get("/api/v1/workspaces/{workspaceID}/chat/channels/{channelID}/messages", handler.ListMessages)
	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") +
		"/api/v1/workspaces/" + url.PathEscape(workspaceID) +
		"/chat/ws?channel_id=" + url.QueryEscape(channelID)
	dial := func(bearer string) (*websocket.Conn, error) {
		parsed, err := url.Parse(wsURL)
		if err != nil {
			return nil, err
		}
		origin, _ := url.Parse(server.URL)
		config, err := websocket.NewConfig(parsed.String(), origin.String())
		if err != nil {
			return nil, err
		}
		if bearer != "" {
			config.Header.Set("Authorization", "Bearer "+bearer)
		}
		return websocket.DialConfig(config)
	}

	if conn, err := dial(""); err == nil {
		conn.Close()
		t.Fatal("missing bearer token unexpectedly opened a WebSocket")
	}
	if conn, err := dial("invalid-token"); err == nil {
		conn.Close()
		t.Fatal("invalid bearer token unexpectedly opened a WebSocket")
	}
	expired, err := jwt.NewWithClaims(jwt.SigningMethodHS256, auth.Claims{
		UserID: "00000000-0000-0000-0000-000000000001",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Minute)),
		},
	}).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign expired token: %v", err)
	}
	if conn, err := dial(expired); err == nil {
		conn.Close()
		t.Fatal("expired bearer token unexpectedly opened a WebSocket")
	}

	memberToken := token
	outsideToken, err := jwtService.GenerateToken("00000000-0000-0000-0000-000000000001")
	if err != nil {
		t.Fatalf("generate non-member token: %v", err)
	}
	outside, err := dial(outsideToken)
	if err != nil {
		t.Fatalf("non-member handshake should finish before application authorization closes it: %v", err)
	}
	_ = outside.SetDeadline(time.Now().Add(2 * time.Second))
	var ignored map[string]interface{}
	if err := websocket.JSON.Receive(outside, &ignored); err == nil {
		t.Fatal("non-member remained connected")
	}
	outside.Close()
	if hasSubscriber(handler.hub, channelID) {
		t.Fatal("non-member was registered with the chat hub")
	}

	// A member cannot use a valid workspace membership to subscribe to an
	// unrelated channel identifier.
	badURL := "ws" + strings.TrimPrefix(server.URL, "http") +
		"/api/v1/workspaces/" + url.PathEscape(workspaceID) +
		"/chat/ws?channel_id=00000000-0000-0000-0000-000000000099"
	badParsed, _ := url.Parse(badURL)
	origin, _ := url.Parse(server.URL)
	badConfig, _ := websocket.NewConfig(badParsed.String(), origin.String())
	badConfig.Header.Set("Authorization", "Bearer "+memberToken)
	badChannel, err := websocket.DialConfig(badConfig)
	if err != nil {
		t.Fatalf("channel authorization handshake: %v", err)
	}
	_ = badChannel.SetDeadline(time.Now().Add(2 * time.Second))
	if err := websocket.JSON.Receive(badChannel, &ignored); err == nil {
		t.Fatal("member remained connected to an unknown channel")
	}
	badChannel.Close()
	if hasSubscriber(handler.hub, "00000000-0000-0000-0000-000000000099") {
		t.Fatal("unknown channel subscriber was registered")
	}
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		body := bytes.NewReader([]byte(`{"content":"unauthorized channel message"}`))
		req, err := http.NewRequest(method,
			server.URL+"/api/v1/workspaces/"+workspaceID+"/chat/channels/00000000-0000-0000-0000-000000000099/messages",
			body)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+memberToken)
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("%s unauthorized channel request: %v", method, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("%s unauthorized channel status = %d, want %d", method, resp.StatusCode, http.StatusNotFound)
		}
	}

	first, err := dial(memberToken)
	if err != nil {
		t.Fatalf("authorized member connection: %v", err)
	}
	defer first.Close()
	second, err := dial(memberToken)
	if err != nil {
		t.Fatalf("second authorized connection: %v", err)
	}
	defer second.Close()
	if !waitForSubscribers(handler.hub, channelID, 2) {
		t.Fatal("both authorized clients were not registered")
	}

	postMessage := func(content string) {
		t.Helper()
		body, _ := json.Marshal(map[string]string{"content": content})
		req, err := http.NewRequest(http.MethodPost,
			server.URL+"/api/v1/workspaces/"+workspaceID+"/chat/channels/"+channelID+"/messages",
			bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+memberToken)
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("post chat message: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("post chat status = %d, want %d", resp.StatusCode, http.StatusCreated)
		}
	}
	type message struct {
		Content string `json:"content"`
	}
	postMessage("authorized delivery")
	for i, conn := range []*websocket.Conn{first, second} {
		_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
		var got message
		if err := websocket.JSON.Receive(conn, &got); err != nil {
			t.Fatalf("client %d receive: %v", i+1, err)
		}
		if got.Content != "authorized delivery" {
			t.Fatalf("client %d received %q", i+1, got.Content)
		}
	}

	first.Close()
	if !waitForSubscribers(handler.hub, channelID, 1) {
		t.Fatal("disconnect did not remove the first subscriber")
	}
	reconnected, err := dial(memberToken)
	if err != nil {
		t.Fatalf("reconnect: %v", err)
	}
	defer reconnected.Close()
	if !waitForSubscribers(handler.hub, channelID, 2) {
		t.Fatal("reconnected client was not registered")
	}
	postMessage("reconnect delivery")
	_ = second.SetDeadline(time.Now().Add(3 * time.Second))
	_ = reconnected.SetDeadline(time.Now().Add(3 * time.Second))
	for i, conn := range []*websocket.Conn{second, reconnected} {
		var got message
		if err := websocket.JSON.Receive(conn, &got); err != nil {
			t.Fatalf("reconnected delivery client %d: %v", i+1, err)
		}
		if got.Content != "reconnect delivery" {
			t.Fatalf("client %d received %q", i+1, got.Content)
		}
	}
	second.Close()
	reconnected.Close()
	if !waitForSubscribers(handler.hub, channelID, 0) {
		t.Fatal("disconnect cleanup left subscribers registered")
	}
	shutdownClient, err := dial(memberToken)
	if err != nil {
		t.Fatalf("connect before chat shutdown: %v", err)
	}
	if !waitForSubscribers(handler.hub, channelID, 1) {
		t.Fatal("shutdown client was not registered")
	}
	handler.Close()
	if !waitForSubscribers(handler.hub, channelID, 0) {
		t.Fatal("chat shutdown left subscribers registered")
	}
	_ = shutdownClient.SetDeadline(time.Now().Add(2 * time.Second))
	if err := websocket.JSON.Receive(shutdownClient, &ignored); err == nil {
		t.Fatal("chat shutdown left the WebSocket open")
	}
	shutdownClient.Close()
	server.Close()
	if conn, err := dial(memberToken); err == nil {
		conn.Close()
		t.Fatal("connection unexpectedly succeeded after server shutdown")
	}
}

func hasSubscriber(hub *ChatHub, channelID string) bool {
	hub.mu.RLock()
	defer hub.mu.RUnlock()
	return len(hub.subscribers[channelID]) > 0
}

func waitForSubscribers(hub *ChatHub, channelID string, count int) bool {
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		hub.mu.RLock()
		got := len(hub.subscribers[channelID])
		hub.mu.RUnlock()
		if got == count {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}
