package handlers

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"kanban/backend/internal/store"
)

type createTaskBody struct {
	Title       string     `json:"title"`
	Description *string    `json:"description"`
	AssigneeID  *uuid.UUID `json:"assignee_id"`
	DueAt       *time.Time `json:"due_at"`
}

func (h *Handlers) CreateTask(c *fiber.Ctx) error {
	uid, err := parseUserID(c)
	if err != nil {
		return fiber.ErrUnauthorized
	}
	cid, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return h.sendError(c, fiber.StatusBadRequest, "invalid column id")
	}
	var body createTaskBody
	if err := c.BodyParser(&body); err != nil || body.Title == "" {
		return h.sendError(c, fiber.StatusBadRequest, "title required")
	}
	t, err := h.Store.CreateTask(c.Context(), uid, cid, body.Title, body.Description, body.AssigneeID, body.DueAt)
	if err == store.ErrForbidden {
		return h.sendError(c, fiber.StatusForbidden, "access denied")
	}
	if err != nil {
		if err.Error() == "assignee must be a board member" {
			return h.sendError(c, fiber.StatusBadRequest, err.Error())
		}
		return h.sendError(c, fiber.StatusInternalServerError, "failed")
	}
	bid, brdErr := h.Store.TaskBoardID(c.Context(), t.ID)
	if h.Hub != nil && brdErr == nil {
		h.Hub.Broadcast(bid, "task_created", map[string]any{"task_id": t.ID.String()})
	}
	return c.Status(fiber.StatusCreated).JSON(t)
}

type patchTaskBody struct {
	Title         *string    `json:"title"`
	Description   *string    `json:"description"`
	AssigneeID    *uuid.UUID `json:"assignee_id"`
	ClearAssignee bool       `json:"clear_assignee"`
	DueAt         *time.Time `json:"due_at"`
	ClearDueAt    bool       `json:"clear_due_at"`
}

func (h *Handlers) PatchTask(c *fiber.Ctx) error {
	uid, err := parseUserID(c)
	if err != nil {
		return fiber.ErrUnauthorized
	}
	tid, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return h.sendError(c, fiber.StatusBadRequest, "invalid task id")
	}
	var body patchTaskBody
	if err := c.BodyParser(&body); err != nil {
		return h.sendError(c, fiber.StatusBadRequest, "invalid body")
	}
	p := store.TaskPatch{
		Title: body.Title, Description: body.Description,
		AssigneeID: body.AssigneeID, ClearAssignee: body.ClearAssignee,
		DueAt: body.DueAt, ClearDueAt: body.ClearDueAt,
	}
	t, err := h.Store.UpdateTask(c.Context(), uid, tid, p)
	if err == store.ErrForbidden {
		return h.sendError(c, fiber.StatusForbidden, "access denied")
	}
	if err == store.ErrNotFound {
		return h.sendError(c, fiber.StatusNotFound, "task not found")
	}
	if err != nil {
		if err.Error() == "assignee must be a board member" {
			return h.sendError(c, fiber.StatusBadRequest, err.Error())
		}
		return h.sendError(c, fiber.StatusInternalServerError, "failed")
	}
	bid, _ := h.Store.TaskBoardID(c.Context(), tid)
	if h.Hub != nil {
		h.Hub.Broadcast(bid, "task_updated", map[string]any{"task_id": tid.String()})
	}
	return c.JSON(t)
}

type moveTaskBody struct {
	ColumnID uuid.UUID `json:"column_id"`
	Position int       `json:"position"`
}

func (h *Handlers) MoveTask(c *fiber.Ctx) error {
	uid, err := parseUserID(c)
	if err != nil {
		return fiber.ErrUnauthorized
	}
	tid, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return h.sendError(c, fiber.StatusBadRequest, "invalid task id")
	}
	var body moveTaskBody
	if err := c.BodyParser(&body); err != nil {
		return h.sendError(c, fiber.StatusBadRequest, "invalid body")
	}
	err = h.Store.MoveTask(c.Context(), uid, tid, body.ColumnID, body.Position)
	if err == store.ErrForbidden {
		return h.sendError(c, fiber.StatusForbidden, "access denied")
	}
	if err == store.ErrNotFound {
		return h.sendError(c, fiber.StatusNotFound, "task not found")
	}
	if err != nil {
		if err.Error() == "columns must belong to same board" || err.Error() == "invalid position" {
			return h.sendError(c, fiber.StatusBadRequest, err.Error())
		}
		return h.sendError(c, fiber.StatusInternalServerError, "failed")
	}
	bid, _ := h.Store.TaskBoardID(c.Context(), tid)
	if h.Hub != nil {
		h.Hub.Broadcast(bid, "task_moved", map[string]any{
			"task_id": tid.String(), "column_id": body.ColumnID.String(), "position": body.Position,
		})
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *Handlers) DeleteTask(c *fiber.Ctx) error {
	uid, err := parseUserID(c)
	if err != nil {
		return fiber.ErrUnauthorized
	}
	tid, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return h.sendError(c, fiber.StatusBadRequest, "invalid task id")
	}
	bid, _ := h.Store.TaskBoardID(c.Context(), tid)
	err = h.Store.DeleteTask(c.Context(), uid, tid)
	if err == store.ErrForbidden {
		return h.sendError(c, fiber.StatusForbidden, "access denied")
	}
	if err == store.ErrNotFound {
		return h.sendError(c, fiber.StatusNotFound, "task not found")
	}
	if err != nil {
		return h.sendError(c, fiber.StatusInternalServerError, "failed")
	}
	if h.Hub != nil && bid != uuid.Nil {
		h.Hub.Broadcast(bid, "task_deleted", map[string]any{"task_id": tid.String()})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

type commentBody struct {
	Body string `json:"body"`
}

func (h *Handlers) ListComments(c *fiber.Ctx) error {
	uid, err := parseUserID(c)
	if err != nil {
		return fiber.ErrUnauthorized
	}
	tid, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return h.sendError(c, fiber.StatusBadRequest, "invalid task id")
	}
	list, err := h.Store.ListComments(c.Context(), uid, tid)
	if err == store.ErrForbidden {
		return h.sendError(c, fiber.StatusForbidden, "access denied")
	}
	if err == store.ErrNotFound {
		return h.sendError(c, fiber.StatusNotFound, "task not found")
	}
	if err != nil {
		return h.sendError(c, fiber.StatusInternalServerError, "failed")
	}
	return c.JSON(list)
}

func (h *Handlers) CreateComment(c *fiber.Ctx) error {
	uid, err := parseUserID(c)
	if err != nil {
		return fiber.ErrUnauthorized
	}
	tid, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return h.sendError(c, fiber.StatusBadRequest, "invalid task id")
	}
	var body commentBody
	if err := c.BodyParser(&body); err != nil || body.Body == "" {
		return h.sendError(c, fiber.StatusBadRequest, "body required")
	}
	co, err := h.Store.AddComment(c.Context(), uid, tid, body.Body)
	if err == store.ErrForbidden {
		return h.sendError(c, fiber.StatusForbidden, "access denied")
	}
	if err == store.ErrNotFound {
		return h.sendError(c, fiber.StatusNotFound, "task not found")
	}
	if err != nil {
		return h.sendError(c, fiber.StatusInternalServerError, "failed")
	}
	bid, _ := h.Store.TaskBoardID(c.Context(), tid)
	if h.Hub != nil {
		h.Hub.Broadcast(bid, "comment_created", map[string]any{"task_id": tid.String(), "comment_id": co.ID.String()})
	}
	return c.Status(fiber.StatusCreated).JSON(co)
}
