package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
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
		tok, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fiber.ErrUnauthorized
			}
			return []byte(secret), nil
		})
		if err != nil || !tok.Valid {
			return fiber.ErrUnauthorized
		}
		claims, ok := tok.Claims.(jwt.MapClaims)
		if !ok {
			return fiber.ErrUnauthorized
		}
		sub, _ := claims["sub"].(string)
		uid, err := uuid.Parse(sub)
		if err != nil {
			return fiber.ErrUnauthorized
		}
		c.Locals(LocalsUserID, uid)
		return c.Next()
	}
}
