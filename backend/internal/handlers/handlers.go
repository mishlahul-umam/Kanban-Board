package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"kanban/backend/internal/middleware"
	"kanban/backend/internal/store"
	"kanban/backend/internal/ws"
)

type Handlers struct {
	Store     *store.Store
	JWTSecret string
	Hub       *ws.Hub
}

func (h *Handlers) sendError(c *fiber.Ctx, status int, msg string) error {
	return c.Status(status).JSON(fiber.Map{"error": msg})
}

func parseUserID(c *fiber.Ctx) (uuid.UUID, error) {
	v := c.Locals(middleware.LocalsUserID)
	uid, ok := v.(uuid.UUID)
	if !ok {
		return uuid.Nil, fiber.ErrUnauthorized
	}
	return uid, nil
}
