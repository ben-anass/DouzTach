package handlers

import (
	"fmt"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ben-anass/DouzTach/internal/repository"
	"golang.org/x/net/websocket"
)

func TestChatHubSubscriberLifecycle(t *testing.T) {
	hub := NewChatHub()
	var conn *websocket.Conn

	hub.Register("channel-1", conn)
	hub.mu.RLock()
	count := len(hub.subscribers["channel-1"])
	hub.mu.RUnlock()
	if count != 1 {
		t.Fatalf("subscriber count = %d, want 1", count)
	}

	hub.Unregister("channel-1", conn)
	hub.mu.RLock()
	_, exists := hub.subscribers["channel-1"]
	hub.mu.RUnlock()
	if exists {
		t.Fatal("empty channel subscription should be removed")
	}
}

func TestChatHubConcurrentBroadcastAndClose(t *testing.T) {
	hub := NewChatHub()
	server := httptest.NewServer(websocket.Handler(func(conn *websocket.Conn) {
		hub.Register("channel-1", conn)
		defer hub.Unregister("channel-1", conn)
		var incoming map[string]interface{}
		for websocket.JSON.Receive(conn, &incoming) == nil {
		}
	}))
	defer server.Close()

	endpoint := "ws" + strings.TrimPrefix(server.URL, "http")
	origin, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	config, err := websocket.NewConfig(endpoint, origin.String())
	if err != nil {
		t.Fatal(err)
	}
	client, err := websocket.DialConfig(config)
	if err != nil {
		t.Fatalf("dial test WebSocket: %v", err)
	}
	defer client.Close()

	waitForChatSubscribers(t, hub, 1)

	const messages = 32
	var broadcasters sync.WaitGroup
	for i := 0; i < messages; i++ {
		broadcasters.Add(1)
		go func(i int) {
			defer broadcasters.Done()
			hub.Broadcast("channel-1", repository.ChatMessage{Content: fmt.Sprint(i)})
		}(i)
	}
	broadcasters.Wait()

	_ = client.SetDeadline(time.Now().Add(3 * time.Second))
	received := make(map[string]bool, messages)
	for i := 0; i < messages; i++ {
		var message repository.ChatMessage
		if err := websocket.JSON.Receive(client, &message); err != nil {
			t.Fatalf("receive concurrent message %d: %v", i+1, err)
		}
		received[message.Content] = true
	}
	if len(received) != messages {
		t.Fatalf("received %d distinct messages, want %d", len(received), messages)
	}

	hub.Close()
	waitForChatSubscribers(t, hub, 0)
	if err := websocket.JSON.Receive(client, &map[string]interface{}{}); err == nil {
		t.Fatal("client remained connected after hub shutdown")
	}
}

func waitForChatSubscribers(t *testing.T, hub *ChatHub, count int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		hub.mu.RLock()
		got := len(hub.subscribers["channel-1"])
		hub.mu.RUnlock()
		if got == count {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("subscriber count did not reach %d", count)
}
