package handlers

import (
	"context"
	"log"

	"github.com/gofiber/contrib/websocket"
	"github.com/google/uuid"
	"kanban/backend/internal/authjwt"
)

func (h *Handlers) BoardWebSocket(c *websocket.Conn) {
	bid, err := uuid.Parse(c.Params("id"))
	if err != nil {
		_ = c.Close()
		return
	}
	token := c.Query("token")
	uid, err := authjwt.Parse(token, h.JWTSecret)
	if err != nil {
		_ = c.Close()
		return
	}
	ok, err := h.Store.HasBoardAccess(context.Background(), uid, bid)
	if err != nil || !ok {
		_ = c.Close()
		return
	}
	if h.Hub == nil {
		_ = c.Close()
		return
	}
	h.Hub.Register(bid, c)
	defer h.Hub.Unregister(bid, c)
	for {
		if _, _, err := c.ReadMessage(); err != nil {
			log.Printf("ws read: %v", err)
			break
		}
	}
}
