package handlers

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"kanban/backend/internal/store"
)

type createBoardBody struct {
	Title string `json:"title"`
}

func (h *Handlers) ListBoards(c *fiber.Ctx) error {
	uid, err := parseUserID(c)
	if err != nil {
		return fiber.ErrUnauthorized
	}
	boards, err := h.Store.ListBoards(c.Context(), uid)
	if err != nil {
		return h.sendError(c, fiber.StatusInternalServerError, "failed to list boards")
	}
	return c.JSON(boards)
}

func (h *Handlers) CreateBoard(c *fiber.Ctx) error {
	uid, err := parseUserID(c)
	if err != nil {
		return fiber.ErrUnauthorized
	}
	var body createBoardBody
	if err := c.BodyParser(&body); err != nil || body.Title == "" {
		return h.sendError(c, fiber.StatusBadRequest, "title required")
	}
	b, err := h.Store.CreateBoard(c.Context(), uid, body.Title)
	if err != nil {
		return h.sendError(c, fiber.StatusInternalServerError, "could not create board")
	}
	return c.Status(fiber.StatusCreated).JSON(b)
}

func (h *Handlers) GetBoard(c *fiber.Ctx) error {
	uid, err := parseUserID(c)
	if err != nil {
		return fiber.ErrUnauthorized
	}
	bid, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return h.sendError(c, fiber.StatusBadRequest, "invalid board id")
	}
	d, err := h.Store.GetBoardDetail(c.Context(), uid, bid)
	if errors.Is(err, store.ErrForbidden) {
		return h.sendError(c, fiber.StatusForbidden, "access denied")
	}
	if errors.Is(err, store.ErrNotFound) {
		return h.sendError(c, fiber.StatusNotFound, "board not found")
	}
	if err != nil {
		return h.sendError(c, fiber.StatusInternalServerError, "failed")
	}
	return c.JSON(d)
}

type patchBoardBody struct {
	Title string `json:"title"`
}

func (h *Handlers) PatchBoard(c *fiber.Ctx) error {
	uid, err := parseUserID(c)
	if err != nil {
		return fiber.ErrUnauthorized
	}
	bid, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return h.sendError(c, fiber.StatusBadRequest, "invalid board id")
	}
	var body patchBoardBody
	if err := c.BodyParser(&body); err != nil || body.Title == "" {
		return h.sendError(c, fiber.StatusBadRequest, "title required")
	}
	err = h.Store.UpdateBoard(c.Context(), uid, bid, body.Title)
	if errors.Is(err, store.ErrForbidden) {
		return h.sendError(c, fiber.StatusForbidden, "access denied")
	}
	if errors.Is(err, store.ErrNotFound) {
		return h.sendError(c, fiber.StatusNotFound, "board not found")
	}
	if err != nil {
		return h.sendError(c, fiber.StatusInternalServerError, "failed")
	}
	if h.Hub != nil {
		h.Hub.Broadcast(bid, "board_updated", map[string]any{"title": body.Title})
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *Handlers) DeleteBoard(c *fiber.Ctx) error {
	uid, err := parseUserID(c)
	if err != nil {
		return fiber.ErrUnauthorized
	}
	bid, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return h.sendError(c, fiber.StatusBadRequest, "invalid board id")
	}
	err = h.Store.DeleteBoard(c.Context(), uid, bid)
	if errors.Is(err, store.ErrForbidden) {
		return h.sendError(c, fiber.StatusForbidden, "only owner can delete")
	}
	if errors.Is(err, store.ErrNotFound) {
		return h.sendError(c, fiber.StatusNotFound, "board not found")
	}
	if err != nil {
		return h.sendError(c, fiber.StatusInternalServerError, "failed")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

type inviteMemberBody struct {
	Email string `json:"email"`
}

func (h *Handlers) InviteMember(c *fiber.Ctx) error {
	uid, err := parseUserID(c)
	if err != nil {
		return fiber.ErrUnauthorized
	}
	bid, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return h.sendError(c, fiber.StatusBadRequest, "invalid board id")
	}
	var body inviteMemberBody
	if err := c.BodyParser(&body); err != nil {
		return h.sendError(c, fiber.StatusBadRequest, "invalid body")
	}
	email := strings.TrimSpace(strings.ToLower(body.Email))
	if email == "" {
		return h.sendError(c, fiber.StatusBadRequest, "email required")
	}
	u, _, err := h.Store.UserByEmail(c.Context(), email)
	if errors.Is(err, store.ErrNotFound) {
		return h.sendError(c, fiber.StatusNotFound, "no user registered with that email")
	}
	if err != nil {
		return h.sendError(c, fiber.StatusInternalServerError, "failed")
	}
	err = h.Store.AddBoardMember(c.Context(), uid, bid, u.ID)
	if errors.Is(err, store.ErrForbidden) {
		return h.sendError(c, fiber.StatusForbidden, "only the board owner can invite members")
	}
	if errors.Is(err, store.ErrNotFound) {
		return h.sendError(c, fiber.StatusNotFound, "board not found")
	}
	if err != nil {
		return h.sendError(c, fiber.StatusInternalServerError, "failed")
	}
	if h.Hub != nil {
		h.Hub.Broadcast(bid, "member_added", map[string]any{"user_id": u.ID.String()})
	}
	return c.Status(fiber.StatusCreated).JSON(u)
}

func (h *Handlers) RemoveMember(c *fiber.Ctx) error {
	uid, err := parseUserID(c)
	if err != nil {
		return fiber.ErrUnauthorized
	}
	bid, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return h.sendError(c, fiber.StatusBadRequest, "invalid board id")
	}
	mid, err := uuid.Parse(c.Params("userId"))
	if err != nil {
		return h.sendError(c, fiber.StatusBadRequest, "invalid user id")
	}
	err = h.Store.RemoveBoardMember(c.Context(), uid, bid, mid)
	if errors.Is(err, store.ErrForbidden) {
		return h.sendError(c, fiber.StatusForbidden, "only the board owner can remove members")
	}
	if errors.Is(err, store.ErrNotFound) {
		return h.sendError(c, fiber.StatusNotFound, "member not found")
	}
	if errors.Is(err, store.ErrCannotRemoveOwner) {
		return h.sendError(c, fiber.StatusBadRequest, "cannot remove the board owner")
	}
	if err != nil {
		return h.sendError(c, fiber.StatusInternalServerError, "failed")
	}
	if h.Hub != nil {
		h.Hub.Broadcast(bid, "member_removed", map[string]any{"user_id": mid.String()})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

type createColumnBody struct {
	Title    string `json:"title"`
	Position *int   `json:"position"`
}

func (h *Handlers) CreateColumn(c *fiber.Ctx) error {
	uid, err := parseUserID(c)
	if err != nil {
		return fiber.ErrUnauthorized
	}
	bid, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return h.sendError(c, fiber.StatusBadRequest, "invalid board id")
	}
	var body createColumnBody
	if err := c.BodyParser(&body); err != nil || body.Title == "" {
		return h.sendError(c, fiber.StatusBadRequest, "title required")
	}
	col, err := h.Store.CreateColumn(c.Context(), uid, bid, body.Title, body.Position)
	if errors.Is(err, store.ErrForbidden) {
		return h.sendError(c, fiber.StatusForbidden, "access denied")
	}
	if err != nil {
		return h.sendError(c, fiber.StatusInternalServerError, "failed")
	}
	if h.Hub != nil {
		h.Hub.Broadcast(bid, "column_created", map[string]any{"column_id": col.ID.String()})
	}
	return c.Status(fiber.StatusCreated).JSON(col)
}

type patchColumnBody struct {
	Title    *string `json:"title"`
	Position *int    `json:"position"`
}

func (h *Handlers) PatchColumn(c *fiber.Ctx) error {
	uid, err := parseUserID(c)
	if err != nil {
		return fiber.ErrUnauthorized
	}
	cid, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return h.sendError(c, fiber.StatusBadRequest, "invalid column id")
	}
	var body patchColumnBody
	if err := c.BodyParser(&body); err != nil {
		return h.sendError(c, fiber.StatusBadRequest, "invalid body")
	}
	if body.Title == nil && body.Position == nil {
		return h.sendError(c, fiber.StatusBadRequest, "nothing to update")
	}
	err = h.Store.UpdateColumn(c.Context(), uid, cid, body.Title, body.Position)
	if errors.Is(err, store.ErrForbidden) {
		return h.sendError(c, fiber.StatusForbidden, "access denied")
	}
	if errors.Is(err, store.ErrNotFound) {
		return h.sendError(c, fiber.StatusNotFound, "column not found")
	}
	if err != nil {
		return h.sendError(c, fiber.StatusInternalServerError, "failed")
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *Handlers) DeleteColumn(c *fiber.Ctx) error {
	uid, err := parseUserID(c)
	if err != nil {
		return fiber.ErrUnauthorized
	}
	cid, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return h.sendError(c, fiber.StatusBadRequest, "invalid column id")
	}
	err = h.Store.DeleteColumn(c.Context(), uid, cid)
	if errors.Is(err, store.ErrForbidden) {
		return h.sendError(c, fiber.StatusForbidden, "access denied")
	}
	if errors.Is(err, store.ErrNotFound) {
		return h.sendError(c, fiber.StatusNotFound, "column not found")
	}
	if errors.Is(err, store.ErrColumnHasTasks) {
		return h.sendError(c, fiber.StatusBadRequest, "column has tasks")
	}
	if err != nil {
		return h.sendError(c, fiber.StatusInternalServerError, "failed")
	}
	return c.SendStatus(fiber.StatusNoContent)
}
