package ws

import (
	"encoding/json"
	"sync"

	"github.com/gofiber/contrib/websocket"
	"github.com/google/uuid"
)

type Hub struct {
	mu   sync.RWMutex
	subs map[string]map[*websocket.Conn]struct{}
}

func NewHub() *Hub {
	return &Hub{subs: make(map[string]map[*websocket.Conn]struct{})}
}

func boardKey(id uuid.UUID) string {
	return id.String()
}

func (h *Hub) Register(boardID uuid.UUID, c *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	k := boardKey(boardID)
	if h.subs[k] == nil {
		h.subs[k] = make(map[*websocket.Conn]struct{})
	}
	h.subs[k][c] = struct{}{}
}

func (h *Hub) Unregister(boardID uuid.UUID, c *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	k := boardKey(boardID)
	if m, ok := h.subs[k]; ok {
		delete(m, c)
		if len(m) == 0 {
			delete(h.subs, k)
		}
	}
}

type Event struct {
	Type string         `json:"type"`
	Data map[string]any `json:"data,omitempty"`
}

func (h *Hub) Broadcast(boardID uuid.UUID, typ string, data map[string]any) {
	payload, err := json.Marshal(Event{Type: typ, Data: data})
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	k := boardKey(boardID)
	m := h.subs[k]
	for c := range m {
		_ = c.WriteMessage(websocket.TextMessage, payload)
	}
}
