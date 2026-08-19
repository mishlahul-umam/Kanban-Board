package handlers

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
	"kanban/backend/internal/authjwt"
	"kanban/backend/internal/store"
)

type registerBody struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
}

type loginBody struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *Handlers) Register(c *fiber.Ctx) error {
	var body registerBody
	if err := c.BodyParser(&body); err != nil {
		return h.sendError(c, fiber.StatusBadRequest, "invalid body")
	}
	body.Email = strings.TrimSpace(strings.ToLower(body.Email))
	body.DisplayName = strings.TrimSpace(body.DisplayName)
	if body.Email == "" || len(body.Password) < 8 || body.DisplayName == "" {
		return h.sendError(c, fiber.StatusBadRequest, "email, password (min 8), display_name required")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
	if err != nil {
		return h.sendError(c, fiber.StatusInternalServerError, "could not hash password")
	}
	u, err := h.Store.CreateUser(c.Context(), body.Email, string(hash), body.DisplayName)
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			return h.sendError(c, fiber.StatusConflict, "email already registered")
		}
		return h.sendError(c, fiber.StatusInternalServerError, "could not create user")
	}
	token, err := authjwt.Sign(u.ID, h.JWTSecret, 7*24*time.Hour)
	if err != nil {
		return h.sendError(c, fiber.StatusInternalServerError, "could not sign token")
	}
	return c.JSON(fiber.Map{
		"token": token,
		"user":  u,
	})
}

func (h *Handlers) Login(c *fiber.Ctx) error {
	var body loginBody
	if err := c.BodyParser(&body); err != nil {
		return h.sendError(c, fiber.StatusBadRequest, "invalid body")
	}
	body.Email = strings.TrimSpace(strings.ToLower(body.Email))
	if body.Email == "" || body.Password == "" {
		return h.sendError(c, fiber.StatusBadRequest, "email and password required")
	}
	u, hash, err := h.Store.UserByEmail(c.Context(), body.Email)
	if err == store.ErrNotFound {
		return h.sendError(c, fiber.StatusUnauthorized, "invalid credentials")
	}
	if err != nil {
		return h.sendError(c, fiber.StatusInternalServerError, "login failed")
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(body.Password)) != nil {
		return h.sendError(c, fiber.StatusUnauthorized, "invalid credentials")
	}
	token, err := authjwt.Sign(u.ID, h.JWTSecret, 7*24*time.Hour)
	if err != nil {
		return h.sendError(c, fiber.StatusInternalServerError, "could not sign token")
	}
	return c.JSON(fiber.Map{
		"token": token,
		"user":  u,
	})
}

func (h *Handlers) Me(c *fiber.Ctx) error {
	uid, err := parseUserID(c)
	if err != nil {
		return fiber.ErrUnauthorized
	}
	u, err := h.Store.UserByID(c.Context(), uid)
	if err == store.ErrNotFound {
		return h.sendError(c, fiber.StatusNotFound, "user not found")
	}
	if err != nil {
		return h.sendError(c, fiber.StatusInternalServerError, "failed")
	}
	return c.JSON(u)
}
