package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"kanban/backend/internal/authjwt"
)

const LocalsUserID = "userID"

func JWT(secret string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		h := c.Get("Authorization")
		const prefix = "Bearer "
		if len(h) < len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
			return fiber.ErrUnauthorized
		}
		tokenStr := strings.TrimSpace(h[len(prefix):])
		if tokenStr == "" {
			return fiber.ErrUnauthorized
		}
		uid, err := authjwt.Parse(tokenStr, secret)
		if err != nil {
			return fiber.ErrUnauthorized
		}
		c.Locals(LocalsUserID, uid)
		return c.Next()
	}
}
