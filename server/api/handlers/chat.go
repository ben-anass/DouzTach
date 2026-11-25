package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"sync"

	"github.com/ben-anass/DouzTach/internal/auth"
	"github.com/ben-anass/DouzTach/internal/repository"
	"github.com/ben-anass/DouzTach/server/activity"
	"github.com/ben-anass/DouzTach/server/chat"
	"github.com/go-chi/chi/v5"
	"golang.org/x/net/websocket"
)

type ChatHandler struct {
	chatService *chat.Service
	hub         *ChatHub
}

func NewChatHandler(service *chat.Service) *ChatHandler {
	return &ChatHandler{
		chatService: service,
		hub:         NewChatHub(),
	}
}

type CreateChannelRequest struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
}

func (h *ChatHandler) CreateChannel(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	userIDStr, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	workspaceID := parseUUID(chi.URLParam(r, "workspaceID"))
	userID := parseUUID(userIDStr)

	var req CreateChannelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	channel, err := h.chatService.CreateChannel(r.Context(), workspaceID, userID, req.Name, req.Kind)
	if err != nil {
		writeChatError(w, err)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(channel)
}

func (h *ChatHandler) ListChannels(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	userIDStr, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	workspaceID := parseUUID(chi.URLParam(r, "workspaceID"))
	userID := parseUUID(userIDStr)

	channels, err := h.chatService.ListChannels(r.Context(), workspaceID, userID)
	if err != nil {
		writeChatError(w, err)
		return
	}
	json.NewEncoder(w).Encode(channels)
}

type SendMessageRequest struct {
	Content string `json:"content"`
}

func (h *ChatHandler) SendMessage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	userIDStr, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	workspaceID := parseUUID(chi.URLParam(r, "workspaceID"))
	channelID := parseUUID(chi.URLParam(r, "channelID"))
	userID := parseUUID(userIDStr)

	var req SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	msg, err := h.chatService.PostMessage(r.Context(), workspaceID, channelID, userID, req.Content)
	if err != nil {
		writeChatError(w, err)
		return
	}

	// Broadcast via WebSocket
	h.hub.Broadcast(channelID.String(), msg)

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(msg)
}

func (h *ChatHandler) ListMessages(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	userIDStr, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	workspaceID := parseUUID(chi.URLParam(r, "workspaceID"))
	channelID := parseUUID(chi.URLParam(r, "channelID"))
	userID := parseUUID(userIDStr)

	limit := int32(50)
	if l := r.URL.Query().Get("limit"); l != "" {
		val, err := strconv.Atoi(l)
		if err != nil || val < 1 || val > 100 {
			http.Error(w, `{"error":"limit must be between 1 and 100"}`, http.StatusBadRequest)
			return
		}
		limit = int32(val)
	}

	messages, err := h.chatService.ListMessages(r.Context(), workspaceID, channelID, userID, limit, 0)
	if err != nil {
		writeChatError(w, err)
		return
	}
	json.NewEncoder(w).Encode(messages)
}

// WebSocket handler
func (h *ChatHandler) WebSocketHandler() http.Handler {
	return websocket.Handler(func(ws *websocket.Conn) {
		defer ws.Close()
		userIDStr, ok := auth.UserIDFromContext(ws.Request().Context())
		if !ok {
			return
		}
		workspaceID := parseUUID(chi.URLParam(ws.Request(), "workspaceID"))
		channelID := parseUUID(ws.Request().URL.Query().Get("channel_id"))
		userID := parseUUID(userIDStr)
		if _, err := h.chatService.ListMessages(ws.Request().Context(), workspaceID, channelID, userID, 1, 0); err != nil {
			return
		}
		channelIDStr := ws.Request().URL.Query().Get("channel_id")
		if channelIDStr == "" {
			return
		}

		h.hub.Register(channelIDStr, ws)
		defer h.hub.Unregister(channelIDStr, ws)

		// Read loop to keep alive
		var msg map[string]interface{}
		for {
			if err := websocket.JSON.Receive(ws, &msg); err != nil {
				break
			}
		}
	})
}

// ChatHub manages websocket subscriptions
type ChatHub struct {
	mu          sync.RWMutex
	subscribers map[string]map[*websocket.Conn]*chatSubscriber
	closed      bool
}

type chatSubscriber struct {
	conn    *websocket.Conn
	writeMu sync.Mutex
}

func NewChatHub() *ChatHub {
	return &ChatHub{
		subscribers: make(map[string]map[*websocket.Conn]*chatSubscriber),
	}
}

func (hub *ChatHub) Register(channelID string, conn *websocket.Conn) {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	if hub.closed {
		_ = conn.Close()
		return
	}
	if hub.subscribers[channelID] == nil {
		hub.subscribers[channelID] = make(map[*websocket.Conn]*chatSubscriber)
	}
	hub.subscribers[channelID][conn] = &chatSubscriber{conn: conn}
}

func (hub *ChatHub) Unregister(channelID string, conn *websocket.Conn) {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	if conns := hub.subscribers[channelID]; conns != nil {
		delete(conns, conn)
		if len(conns) == 0 {
			delete(hub.subscribers, channelID)
		}
	}
}

func (hub *ChatHub) Broadcast(channelID string, msg repository.ChatMessage) {
	hub.mu.RLock()
	subscribers := make([]*chatSubscriber, 0, len(hub.subscribers[channelID]))
	for _, subscriber := range hub.subscribers[channelID] {
		subscribers = append(subscribers, subscriber)
	}
	hub.mu.RUnlock()

	for _, subscriber := range subscribers {
		subscriber.writeMu.Lock()
		err := websocket.JSON.Send(subscriber.conn, msg)
		subscriber.writeMu.Unlock()
		if err != nil {
			hub.Unregister(channelID, subscriber.conn)
			_ = subscriber.conn.Close()
		}
	}
}

func (hub *ChatHub) Close() {
	hub.mu.Lock()
	if hub.closed {
		hub.mu.Unlock()
		return
	}
	hub.closed = true
	var conns []*websocket.Conn
	for _, subscribers := range hub.subscribers {
		for _, subscriber := range subscribers {
			conns = append(conns, subscriber.conn)
		}
	}
	hub.subscribers = make(map[string]map[*websocket.Conn]*chatSubscriber)
	hub.mu.Unlock()

	for _, conn := range conns {
		_ = conn.Close()
	}
}

func (h *ChatHandler) Close() {
	h.hub.Close()
}

func writeChatError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	message := "internal server error"
	switch {
	case errors.Is(err, activity.ErrNotMember), errors.Is(err, activity.ErrReadOnly):
		status = http.StatusForbidden
		message = "forbidden"
	case errors.Is(err, chat.ErrNotFound):
		status = http.StatusNotFound
		message = "not found"
	case errors.Is(err, chat.ErrInvalidInput):
		status = http.StatusBadRequest
		message = err.Error()
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
