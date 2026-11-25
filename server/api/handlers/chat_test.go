package handlers

import (
	"testing"

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
