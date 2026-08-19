package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("not found")
var ErrForbidden = errors.New("forbidden")
var ErrColumnHasTasks = errors.New("column has tasks")
var ErrCannotRemoveOwner = errors.New("cannot remove board owner")

type Store struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

type User struct {
	ID          uuid.UUID `json:"id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
}

func (s *Store) CreateUser(ctx context.Context, email, passwordHash, displayName string) (User, error) {
	var u User
	err := s.pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, display_name)
		VALUES ($1, $2, $3)
		RETURNING id, email, display_name
	`, email, passwordHash, displayName).Scan(&u.ID, &u.Email, &u.DisplayName)
	return u, err
}

func (s *Store) UserByEmail(ctx context.Context, email string) (User, string, error) {
	var u User
	var hash string
	err := s.pool.QueryRow(ctx, `
		SELECT id, email, display_name, password_hash FROM users WHERE email = $1
	`, email).Scan(&u.ID, &u.Email, &u.DisplayName, &hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, "", ErrNotFound
	}
	return u, hash, err
}

func (s *Store) UserByID(ctx context.Context, id uuid.UUID) (User, error) {
	var u User
	err := s.pool.QueryRow(ctx, `
		SELECT id, email, display_name FROM users WHERE id = $1
	`, id).Scan(&u.ID, &u.Email, &u.DisplayName)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}

func (s *Store) HasBoardAccess(ctx context.Context, userID, boardID uuid.UUID) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM boards b
			WHERE b.id = $1 AND (
				b.owner_id = $2 OR EXISTS (
					SELECT 1 FROM board_members bm
					WHERE bm.board_id = b.id AND bm.user_id = $2
				)
			)
		)
	`, boardID, userID).Scan(&ok)
	return ok, err
}

type BoardSummary struct {
	ID        uuid.UUID `json:"id"`
	Title     string    `json:"title"`
	OwnerID   uuid.UUID `json:"owner_id"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Store) ListBoards(ctx context.Context, userID uuid.UUID) ([]BoardSummary, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT b.id, b.title, b.owner_id, b.created_at
		FROM boards b
		LEFT JOIN board_members bm ON bm.board_id = b.id
		WHERE b.owner_id = $1 OR bm.user_id = $1
		ORDER BY b.created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BoardSummary
	for rows.Next() {
		var b BoardSummary
		if err := rows.Scan(&b.ID, &b.Title, &b.OwnerID, &b.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (s *Store) CreateBoard(ctx context.Context, ownerID uuid.UUID, title string) (BoardSummary, error) {
	var b BoardSummary
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return BoardSummary{}, err
	}
	defer tx.Rollback(ctx)

	err = tx.QueryRow(ctx, `
		INSERT INTO boards (owner_id, title) VALUES ($1, $2)
		RETURNING id, title, owner_id, created_at
	`, ownerID, title).Scan(&b.ID, &b.Title, &b.OwnerID, &b.CreatedAt)
	if err != nil {
		return BoardSummary{}, err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO board_members (board_id, user_id) VALUES ($1, $2)
	`, b.ID, ownerID)
	if err != nil {
		return BoardSummary{}, err
	}
	cols := []struct {
		title string
		pos   int
	}{
		{"To Do", 0},
		{"In Progress", 1},
		{"Done", 2},
	}
	for _, c := range cols {
		_, err = tx.Exec(ctx, `
			INSERT INTO columns (board_id, title, position) VALUES ($1, $2, $3)
		`, b.ID, c.title, c.pos)
		if err != nil {
			return BoardSummary{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return BoardSummary{}, err
	}
	return b, nil
}

type TaskOut struct {
	ID          uuid.UUID  `json:"id"`
	ColumnID    uuid.UUID  `json:"column_id"`
	Title       string     `json:"title"`
	Description *string    `json:"description,omitempty"`
	Position    int        `json:"position"`
	AssigneeID  *uuid.UUID `json:"assignee_id,omitempty"`
	DueAt       *time.Time `json:"due_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type ColumnOut struct {
	ID       uuid.UUID `json:"id"`
	BoardID  uuid.UUID `json:"board_id"`
	Title    string    `json:"title"`
	Position int       `json:"position"`
	Tasks    []TaskOut `json:"tasks"`
}

type BoardDetail struct {
	BoardSummary
	Columns []ColumnOut `json:"columns"`
	Members []User      `json:"members"`
}

func (s *Store) GetBoardDetail(ctx context.Context, userID, boardID uuid.UUID) (BoardDetail, error) {
	ok, err := s.HasBoardAccess(ctx, userID, boardID)
	if err != nil {
		return BoardDetail{}, err
	}
	if !ok {
		return BoardDetail{}, ErrForbidden
	}
	var b BoardDetail
	err = s.pool.QueryRow(ctx, `
		SELECT id, title, owner_id, created_at FROM boards WHERE id = $1
	`, boardID).Scan(&b.ID, &b.Title, &b.OwnerID, &b.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return BoardDetail{}, ErrNotFound
		}
		return BoardDetail{}, err
	}

	rows, err := s.pool.Query(ctx, `
		SELECT id, board_id, title, position FROM columns WHERE board_id = $1 ORDER BY position ASC
	`, boardID)
	if err != nil {
		return BoardDetail{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var col ColumnOut
		if err := rows.Scan(&col.ID, &col.BoardID, &col.Title, &col.Position); err != nil {
			return BoardDetail{}, err
		}
		col.Tasks = []TaskOut{}
		b.Columns = append(b.Columns, col)
	}
	if err := rows.Err(); err != nil {
		return BoardDetail{}, err
	}

	taskRows, err := s.pool.Query(ctx, `
		SELECT t.id, t.column_id, t.title, t.description, t.position, t.assignee_id, t.due_at, t.created_at, t.updated_at
		FROM tasks t
		JOIN columns c ON c.id = t.column_id
		WHERE c.board_id = $1
		ORDER BY t.column_id, t.position ASC
	`, boardID)
	if err != nil {
		return BoardDetail{}, err
	}
	defer taskRows.Close()
	colIndex := make(map[uuid.UUID]int)
	for i := range b.Columns {
		colIndex[b.Columns[i].ID] = i
	}
	for taskRows.Next() {
		var t TaskOut
		if err := taskRows.Scan(&t.ID, &t.ColumnID, &t.Title, &t.Description, &t.Position, &t.AssigneeID, &t.DueAt, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return BoardDetail{}, err
		}
		if idx, ok := colIndex[t.ColumnID]; ok {
			b.Columns[idx].Tasks = append(b.Columns[idx].Tasks, t)
		}
	}
	if err := taskRows.Err(); err != nil {
		return BoardDetail{}, err
	}

	mrows, err := s.pool.Query(ctx, `
		SELECT u.id, u.email, u.display_name
		FROM users u
		JOIN board_members bm ON bm.user_id = u.id
		WHERE bm.board_id = $1
		ORDER BY u.display_name
	`, boardID)
	if err != nil {
		return BoardDetail{}, err
	}
	defer mrows.Close()
	for mrows.Next() {
		var u User
		if err := mrows.Scan(&u.ID, &u.Email, &u.DisplayName); err != nil {
			return BoardDetail{}, err
		}
		b.Members = append(b.Members, u)
	}
	return b, mrows.Err()
}

func (s *Store) UpdateBoard(ctx context.Context, userID, boardID uuid.UUID, title string) error {
	ok, err := s.HasBoardAccess(ctx, userID, boardID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrForbidden
	}
	ct, err := s.pool.Exec(ctx, `UPDATE boards SET title = $1 WHERE id = $2`, title, boardID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteBoard(ctx context.Context, userID, boardID uuid.UUID) error {
	var owner uuid.UUID
	err := s.pool.QueryRow(ctx, `SELECT owner_id FROM boards WHERE id = $1`, boardID).Scan(&owner)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if owner != userID {
		return ErrForbidden
	}
	_, err = s.pool.Exec(ctx, `DELETE FROM boards WHERE id = $1`, boardID)
	return err
}

func (s *Store) columnBoardID(ctx context.Context, columnID uuid.UUID) (uuid.UUID, error) {
	var bid uuid.UUID
	err := s.pool.QueryRow(ctx, `SELECT board_id FROM columns WHERE id = $1`, columnID).Scan(&bid)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, ErrNotFound
	}
	return bid, err
}

func (s *Store) CreateColumn(ctx context.Context, userID, boardID uuid.UUID, title string, position *int) (ColumnOut, error) {
	ok, err := s.HasBoardAccess(ctx, userID, boardID)
	if err != nil {
		return ColumnOut{}, err
	}
	if !ok {
		return ColumnOut{}, ErrForbidden
	}
	var pos int
	if position != nil {
		pos = *position
	} else {
		err = s.pool.QueryRow(ctx, `
			SELECT COALESCE(MAX(position), -1) + 1 FROM columns WHERE board_id = $1
		`, boardID).Scan(&pos)
		if err != nil {
			return ColumnOut{}, err
		}
	}
	var col ColumnOut
	err = s.pool.QueryRow(ctx, `
		INSERT INTO columns (board_id, title, position) VALUES ($1, $2, $3)
		RETURNING id, board_id, title, position
	`, boardID, title, pos).Scan(&col.ID, &col.BoardID, &col.Title, &col.Position)
	col.Tasks = []TaskOut{}
	return col, err
}

func (s *Store) UpdateColumn(ctx context.Context, userID, columnID uuid.UUID, title *string, position *int) error {
	bid, err := s.columnBoardID(ctx, columnID)
	if err != nil {
		return err
	}
	ok, err := s.HasBoardAccess(ctx, userID, bid)
	if err != nil {
		return err
	}
	if !ok {
		return ErrForbidden
	}
	if title != nil && position != nil {
		ct, err := s.pool.Exec(ctx, `UPDATE columns SET title = $1, position = $2 WHERE id = $3`, *title, *position, columnID)
		if err != nil {
			return err
		}
		if ct.RowsAffected() == 0 {
			return ErrNotFound
		}
		return nil
	}
	if title != nil {
		ct, err := s.pool.Exec(ctx, `UPDATE columns SET title = $1 WHERE id = $2`, *title, columnID)
		if err != nil {
			return err
		}
		if ct.RowsAffected() == 0 {
			return ErrNotFound
		}
		return nil
	}
	if position != nil {
		ct, err := s.pool.Exec(ctx, `UPDATE columns SET position = $1 WHERE id = $2`, *position, columnID)
		if err != nil {
			return err
		}
		if ct.RowsAffected() == 0 {
			return ErrNotFound
		}
		return nil
	}
	return nil
}

func (s *Store) DeleteColumn(ctx context.Context, userID, columnID uuid.UUID) error {
	bid, err := s.columnBoardID(ctx, columnID)
	if err != nil {
		return err
	}
	ok, err := s.HasBoardAccess(ctx, userID, bid)
	if err != nil {
		return err
	}
	if !ok {
		return ErrForbidden
	}
	var n int
	_ = s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM tasks WHERE column_id = $1`, columnID).Scan(&n)
	if n > 0 {
		return ErrColumnHasTasks
	}
	ct, err := s.pool.Exec(ctx, `DELETE FROM columns WHERE id = $1`, columnID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) CreateTask(ctx context.Context, userID, columnID uuid.UUID, title string, description *string, assigneeID *uuid.UUID, dueAt *time.Time) (TaskOut, error) {
	bid, err := s.columnBoardID(ctx, columnID)
	if err != nil {
		return TaskOut{}, err
	}
	ok, err := s.HasBoardAccess(ctx, userID, bid)
	if err != nil {
		return TaskOut{}, err
	}
	if !ok {
		return TaskOut{}, ErrForbidden
	}
	if assigneeID != nil {
		var mem bool
		err = s.pool.QueryRow(ctx, `
			SELECT EXISTS (SELECT 1 FROM board_members WHERE board_id = $1 AND user_id = $2)
		`, bid, *assigneeID).Scan(&mem)
		if err != nil {
			return TaskOut{}, err
		}
		if !mem {
			var owner uuid.UUID
			_ = s.pool.QueryRow(ctx, `SELECT owner_id FROM boards WHERE id = $1`, bid).Scan(&owner)
			if owner != *assigneeID {
				return TaskOut{}, errors.New("assignee must be a board member")
			}
		}
	}
	var pos int
	err = s.pool.QueryRow(ctx, `
		SELECT COALESCE(MAX(position), -1) + 1 FROM tasks WHERE column_id = $1
	`, columnID).Scan(&pos)
	if err != nil {
		return TaskOut{}, err
	}
	var t TaskOut
	err = s.pool.QueryRow(ctx, `
		INSERT INTO tasks (column_id, title, description, position, assignee_id, due_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, column_id, title, description, position, assignee_id, due_at, created_at, updated_at
	`, columnID, title, description, pos, assigneeID, dueAt).Scan(
		&t.ID, &t.ColumnID, &t.Title, &t.Description, &t.Position, &t.AssigneeID, &t.DueAt, &t.CreatedAt, &t.UpdatedAt,
	)
	return t, err
}

func (s *Store) TaskBoardID(ctx context.Context, taskID uuid.UUID) (uuid.UUID, error) {
	var bid uuid.UUID
	err := s.pool.QueryRow(ctx, `
		SELECT c.board_id FROM tasks t JOIN columns c ON c.id = t.column_id WHERE t.id = $1
	`, taskID).Scan(&bid)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, ErrNotFound
	}
	return bid, err
}

type TaskPatch struct {
	Title         *string
	Description   *string
	AssigneeID    *uuid.UUID
	ClearAssignee bool
	DueAt         *time.Time
	ClearDueAt    bool
}

func (s *Store) UpdateTask(ctx context.Context, userID, taskID uuid.UUID, p TaskPatch) (TaskOut, error) {
	bid, err := s.TaskBoardID(ctx, taskID)
	if err != nil {
		return TaskOut{}, err
	}
	ok, err := s.HasBoardAccess(ctx, userID, bid)
	if err != nil {
		return TaskOut{}, err
	}
	if !ok {
		return TaskOut{}, ErrForbidden
	}
	if p.AssigneeID != nil {
		var mem bool
		err = s.pool.QueryRow(ctx, `
			SELECT EXISTS (SELECT 1 FROM board_members WHERE board_id = $1 AND user_id = $2)
		`, bid, *p.AssigneeID).Scan(&mem)
		if err != nil {
			return TaskOut{}, err
		}
		if !mem {
			var owner uuid.UUID
			_ = s.pool.QueryRow(ctx, `SELECT owner_id FROM boards WHERE id = $1`, bid).Scan(&owner)
			if owner != *p.AssigneeID {
				return TaskOut{}, errors.New("assignee must be a board member")
			}
		}
	}
	var t TaskOut
	err = s.pool.QueryRow(ctx, `
		SELECT id, column_id, title, description, position, assignee_id, due_at, created_at, updated_at
		FROM tasks WHERE id = $1
	`, taskID).Scan(
		&t.ID, &t.ColumnID, &t.Title, &t.Description, &t.Position, &t.AssigneeID, &t.DueAt, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return TaskOut{}, ErrNotFound
		}
		return TaskOut{}, err
	}
	if p.Title != nil {
		t.Title = *p.Title
	}
	if p.Description != nil {
		t.Description = p.Description
	}
	if p.ClearAssignee {
		t.AssigneeID = nil
	} else if p.AssigneeID != nil {
		t.AssigneeID = p.AssigneeID
	}
	if p.ClearDueAt {
		t.DueAt = nil
	} else if p.DueAt != nil {
		t.DueAt = p.DueAt
	}
	err = s.pool.QueryRow(ctx, `
		UPDATE tasks SET
			title = $2,
			description = $3,
			assignee_id = $4,
			due_at = $5,
			updated_at = now()
		WHERE id = $1
		RETURNING id, column_id, title, description, position, assignee_id, due_at, created_at, updated_at
	`, taskID, t.Title, t.Description, t.AssigneeID, t.DueAt).Scan(
		&t.ID, &t.ColumnID, &t.Title, &t.Description, &t.Position, &t.AssigneeID, &t.DueAt, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return TaskOut{}, err
	}
	return t, nil
}

func (s *Store) DeleteTask(ctx context.Context, userID, taskID uuid.UUID) error {
	bid, err := s.TaskBoardID(ctx, taskID)
	if err != nil {
		return err
	}
	ok, err := s.HasBoardAccess(ctx, userID, bid)
	if err != nil {
		return err
	}
	if !ok {
		return ErrForbidden
	}
	ct, err := s.pool.Exec(ctx, `DELETE FROM tasks WHERE id = $1`, taskID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) MoveTask(ctx context.Context, userID, taskID uuid.UUID, destColumnID uuid.UUID, newPosition int) error {
	if newPosition < 0 {
		return errors.New("invalid position")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var srcColumnID uuid.UUID
	var oldPos int
	var srcBoard, dstBoard uuid.UUID
	err = tx.QueryRow(ctx, `
		SELECT t.column_id, t.position, c.board_id
		FROM tasks t
		JOIN columns c ON c.id = t.column_id
		WHERE t.id = $1
		FOR UPDATE OF t
	`, taskID).Scan(&srcColumnID, &oldPos, &srcBoard)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}

	err = tx.QueryRow(ctx, `SELECT board_id FROM columns WHERE id = $1 FOR UPDATE`, destColumnID).Scan(&dstBoard)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if srcBoard != dstBoard {
		return errors.New("columns must belong to same board")
	}

	ok, err := s.hasBoardAccessTx(ctx, tx, userID, srcBoard)
	if err != nil {
		return err
	}
	if !ok {
		return ErrForbidden
	}

	if srcColumnID == destColumnID {
		var ids []uuid.UUID
		rows, err := tx.Query(ctx, `SELECT id FROM tasks WHERE column_id = $1 ORDER BY position ASC`, srcColumnID)
		if err != nil {
			return err
		}
		for rows.Next() {
			var id uuid.UUID
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			ids = append(ids, id)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		idx := -1
		for i, id := range ids {
			if id == taskID {
				idx = i
				break
			}
		}
		if idx < 0 {
			return ErrNotFound
		}
		ids = append(ids[:idx], ids[idx+1:]...)
		if newPosition > len(ids) {
			newPosition = len(ids)
		}
		ids = append(ids[:newPosition], append([]uuid.UUID{taskID}, ids[newPosition:]...)...)
		for i, id := range ids {
			_, err = tx.Exec(ctx, `UPDATE tasks SET position = $1, updated_at = now() WHERE id = $2`, i, id)
			if err != nil {
				return err
			}
		}
		return tx.Commit(ctx)
	}

	var oldIDs []uuid.UUID
	rows, err := tx.Query(ctx, `SELECT id FROM tasks WHERE column_id = $1 ORDER BY position ASC`, srcColumnID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		oldIDs = append(oldIDs, id)
	}
	rows.Close()
	var newOld []uuid.UUID
	for _, id := range oldIDs {
		if id != taskID {
			newOld = append(newOld, id)
		}
	}
	for i, id := range newOld {
		_, err = tx.Exec(ctx, `UPDATE tasks SET position = $1, updated_at = now() WHERE id = $2`, i, id)
		if err != nil {
			return err
		}
	}

	var destIDs []uuid.UUID
	rows, err = tx.Query(ctx, `SELECT id FROM tasks WHERE column_id = $1 ORDER BY position ASC`, destColumnID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		destIDs = append(destIDs, id)
	}
	rows.Close()
	if newPosition > len(destIDs) {
		newPosition = len(destIDs)
	}
	destIDs = append(destIDs[:newPosition], append([]uuid.UUID{taskID}, destIDs[newPosition:]...)...)
	for i, id := range destIDs {
		_, err = tx.Exec(ctx, `UPDATE tasks SET column_id = $1, position = $2, updated_at = now() WHERE id = $3`, destColumnID, i, id)
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *Store) hasBoardAccessTx(ctx context.Context, tx pgx.Tx, userID, boardID uuid.UUID) (bool, error) {
	var ok bool
	err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM boards b
			WHERE b.id = $1 AND (
				b.owner_id = $2 OR EXISTS (
					SELECT 1 FROM board_members bm
					WHERE bm.board_id = b.id AND bm.user_id = $2
				)
			)
		)
	`, boardID, userID).Scan(&ok)
	return ok, err
}

type CommentOut struct {
	ID        uuid.UUID `json:"id"`
	TaskID    uuid.UUID `json:"task_id"`
	UserID    uuid.UUID `json:"user_id"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
	User      *User     `json:"user,omitempty"`
}

func (s *Store) ListComments(ctx context.Context, userID, taskID uuid.UUID) ([]CommentOut, error) {
	bid, err := s.TaskBoardID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	ok, err := s.HasBoardAccess(ctx, userID, bid)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrForbidden
	}
	rows, err := s.pool.Query(ctx, `
		SELECT c.id, c.task_id, c.user_id, c.body, c.created_at, u.email, u.display_name
		FROM comments c
		JOIN users u ON u.id = c.user_id
		WHERE c.task_id = $1
		ORDER BY c.created_at ASC
	`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CommentOut
	for rows.Next() {
		var co CommentOut
		var email, dn string
		if err := rows.Scan(&co.ID, &co.TaskID, &co.UserID, &co.Body, &co.CreatedAt, &email, &dn); err != nil {
			return nil, err
		}
		co.User = &User{ID: co.UserID, Email: email, DisplayName: dn}
		out = append(out, co)
	}
	return out, rows.Err()
}

func (s *Store) AddComment(ctx context.Context, userID, taskID uuid.UUID, body string) (CommentOut, error) {
	bid, err := s.TaskBoardID(ctx, taskID)
	if err != nil {
		return CommentOut{}, err
	}
	ok, err := s.HasBoardAccess(ctx, userID, bid)
	if err != nil {
		return CommentOut{}, err
	}
	if !ok {
		return CommentOut{}, ErrForbidden
	}
	var co CommentOut
	err = s.pool.QueryRow(ctx, `
		INSERT INTO comments (task_id, user_id, body) VALUES ($1, $2, $3)
		RETURNING id, task_id, user_id, body, created_at
	`, taskID, userID, body).Scan(&co.ID, &co.TaskID, &co.UserID, &co.Body, &co.CreatedAt)
	if err != nil {
		return CommentOut{}, err
	}
	u, err := s.UserByID(ctx, userID)
	if err != nil {
		return co, nil
	}
	co.User = &u
	return co, nil
}

func (s *Store) AddBoardMember(ctx context.Context, ownerID, boardID, memberUserID uuid.UUID) error {
	var owner uuid.UUID
	err := s.pool.QueryRow(ctx, `SELECT owner_id FROM boards WHERE id = $1`, boardID).Scan(&owner)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if owner != ownerID {
		return ErrForbidden
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO board_members (board_id, user_id) VALUES ($1, $2)
		ON CONFLICT DO NOTHING
	`, boardID, memberUserID)
	return err
}

func (s *Store) RemoveBoardMember(ctx context.Context, ownerID, boardID, memberUserID uuid.UUID) error {
	var owner uuid.UUID
	err := s.pool.QueryRow(ctx, `SELECT owner_id FROM boards WHERE id = $1`, boardID).Scan(&owner)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if owner != ownerID {
		return ErrForbidden
	}
	if memberUserID == owner {
		return ErrCannotRemoveOwner
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	ct, err := tx.Exec(ctx, `
		DELETE FROM board_members WHERE board_id = $1 AND user_id = $2
	`, boardID, memberUserID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	// Leaving the board is not the same as deleting the user, so the
	// assignee_id FK does not clear itself here.
	_, err = tx.Exec(ctx, `
		UPDATE tasks SET assignee_id = NULL, updated_at = now()
		WHERE assignee_id = $1
		  AND column_id IN (SELECT id FROM columns WHERE board_id = $2)
	`, memberUserID, boardID)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
