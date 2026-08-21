package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"kanban/backend/internal/db"
)

func TestMain(m *testing.M) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn != "" {
		if err := db.Migrate(dsn); err != nil {
			fmt.Fprintf(os.Stderr, "migrate test db: %v\n", err)
			os.Exit(1)
		}
	}
	os.Exit(m.Run())
}

func testStore(t *testing.T) *Store {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping DB-backed test")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect to test db: %v", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		t.Fatalf("ping test db: %v", err)
	}
	t.Cleanup(pool.Close)
	return New(pool)
}

// setupBoard creates a fresh owner user + board (with its 3 auto-seeded
// columns, sorted by position) for a single test. Cleanup deletes the
// owner, which cascades to the board, its columns, and its tasks.
func setupBoard(t *testing.T, s *Store) (ownerID, boardID uuid.UUID, columnIDs []uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	email := "movetask-test-" + uuid.New().String() + "@example.com"
	u, err := s.CreateUser(ctx, email, "test-hash", "Test Owner")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = s.pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, u.ID)
	})

	b, err := s.CreateBoard(ctx, u.ID, "MoveTask test board")
	if err != nil {
		t.Fatalf("create board: %v", err)
	}

	detail, err := s.GetBoardDetail(ctx, u.ID, b.ID)
	if err != nil {
		t.Fatalf("get board detail: %v", err)
	}
	cols := detail.Columns
	sort.Slice(cols, func(i, j int) bool { return cols[i].Position < cols[j].Position })
	ids := make([]uuid.UUID, len(cols))
	for i, c := range cols {
		ids[i] = c.ID
	}
	return u.ID, b.ID, ids
}

// createTask adds a task to columnID and fails the test if it cannot.
func createTask(t *testing.T, s *Store, userID, columnID uuid.UUID, title string) TaskOut {
	t.Helper()
	task, err := s.CreateTask(context.Background(), userID, columnID, title, nil, nil, nil)
	if err != nil {
		t.Fatalf("create task %q: %v", title, err)
	}
	return task
}

// columnTaskTitles returns the titles of columnID's tasks in stored order.
func columnTaskTitles(t *testing.T, s *Store, userID, boardID, columnID uuid.UUID) []string {
	t.Helper()
	detail, err := s.GetBoardDetail(context.Background(), userID, boardID)
	if err != nil {
		t.Fatalf("get board detail: %v", err)
	}
	for _, c := range detail.Columns {
		if c.ID != columnID {
			continue
		}
		titles := make([]string, len(c.Tasks))
		for i, task := range c.Tasks {
			titles[i] = task.Title
		}
		return titles
	}
	t.Fatalf("column %s not found on board", columnID)
	return nil
}

// findTask locates a task on the board by id.
func findTask(t *testing.T, s *Store, userID, boardID, taskID uuid.UUID) TaskOut {
	t.Helper()
	detail, err := s.GetBoardDetail(context.Background(), userID, boardID)
	if err != nil {
		t.Fatalf("get board detail: %v", err)
	}
	for _, c := range detail.Columns {
		for _, task := range c.Tasks {
			if task.ID == taskID {
				return task
			}
		}
	}
	t.Fatalf("task %s not found on board", taskID)
	return TaskOut{}
}

func assertTitles(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("task order: got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("task order: got %v, want %v", got, want)
		}
	}
}

func TestMoveTask_ReorderDown(t *testing.T) {
	t.Parallel()
	s := testStore(t)
	ctx := context.Background()
	ownerID, boardID, cols := setupBoard(t, s)

	t1 := createTask(t, s, ownerID, cols[0], "A")
	createTask(t, s, ownerID, cols[0], "B")
	createTask(t, s, ownerID, cols[0], "C")

	if err := s.MoveTask(ctx, ownerID, t1.ID, cols[0], 2); err != nil {
		t.Fatalf("move task: %v", err)
	}
	assertTitles(t, columnTaskTitles(t, s, ownerID, boardID, cols[0]), []string{"B", "C", "A"})
}

func TestMoveTask_ReorderUp(t *testing.T) {
	t.Parallel()
	s := testStore(t)
	ctx := context.Background()
	ownerID, boardID, cols := setupBoard(t, s)

	createTask(t, s, ownerID, cols[0], "A")
	createTask(t, s, ownerID, cols[0], "B")
	t3 := createTask(t, s, ownerID, cols[0], "C")

	if err := s.MoveTask(ctx, ownerID, t3.ID, cols[0], 0); err != nil {
		t.Fatalf("move task: %v", err)
	}
	assertTitles(t, columnTaskTitles(t, s, ownerID, boardID, cols[0]), []string{"C", "A", "B"})
}

func TestMoveTask_CrossColumn(t *testing.T) {
	t.Parallel()
	s := testStore(t)
	ctx := context.Background()
	ownerID, boardID, cols := setupBoard(t, s)

	t1 := createTask(t, s, ownerID, cols[0], "A")
	createTask(t, s, ownerID, cols[0], "B")
	createTask(t, s, ownerID, cols[1], "C")

	if err := s.MoveTask(ctx, ownerID, t1.ID, cols[1], 1); err != nil {
		t.Fatalf("move task: %v", err)
	}

	assertTitles(t, columnTaskTitles(t, s, ownerID, boardID, cols[0]), []string{"B"})
	assertTitles(t, columnTaskTitles(t, s, ownerID, boardID, cols[1]), []string{"C", "A"})

	moved := findTask(t, s, ownerID, boardID, t1.ID)
	if moved.ColumnID != cols[1] {
		t.Fatalf("moved task column: got %s, want %s", moved.ColumnID, cols[1])
	}
	if moved.Position != 1 {
		t.Fatalf("moved task position: got %d, want 1", moved.Position)
	}
}

func TestMoveTask_ClampsPositionBeyondLength(t *testing.T) {
	t.Parallel()
	s := testStore(t)
	ctx := context.Background()
	ownerID, boardID, cols := setupBoard(t, s)

	t1 := createTask(t, s, ownerID, cols[0], "A")
	createTask(t, s, ownerID, cols[0], "B")

	if err := s.MoveTask(ctx, ownerID, t1.ID, cols[1], 999); err != nil {
		t.Fatalf("move task with out-of-range position: %v", err)
	}

	assertTitles(t, columnTaskTitles(t, s, ownerID, boardID, cols[1]), []string{"A"})
	moved := findTask(t, s, ownerID, boardID, t1.ID)
	if moved.Position != 0 {
		t.Fatalf("clamped position: got %d, want 0", moved.Position)
	}
}

func TestMoveTask_TaskNotFound(t *testing.T) {
	t.Parallel()
	s := testStore(t)

	err := s.MoveTask(context.Background(), uuid.New(), uuid.New(), uuid.New(), 0)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("got %v, want ErrNotFound", err)
	}
}

func TestMoveTask_DestColumnNotFound(t *testing.T) {
	t.Parallel()
	s := testStore(t)
	ctx := context.Background()
	ownerID, _, cols := setupBoard(t, s)

	task := createTask(t, s, ownerID, cols[0], "A")

	err := s.MoveTask(ctx, ownerID, task.ID, uuid.New(), 0)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("got %v, want ErrNotFound", err)
	}
}

func TestMoveTask_ColumnsMustBeSameBoard(t *testing.T) {
	t.Parallel()
	s := testStore(t)
	ctx := context.Background()
	ownerA, _, colsA := setupBoard(t, s)
	_, _, colsB := setupBoard(t, s)

	task := createTask(t, s, ownerA, colsA[0], "A")

	err := s.MoveTask(ctx, ownerA, task.ID, colsB[0], 0)
	if !errors.Is(err, ErrCrossBoardMove) {
		t.Fatalf("got %v, want ErrCrossBoardMove", err)
	}
}

func TestMoveTask_Forbidden(t *testing.T) {
	t.Parallel()
	s := testStore(t)
	ctx := context.Background()
	ownerID, _, cols := setupBoard(t, s)

	task := createTask(t, s, ownerID, cols[0], "A")

	email := "movetask-outsider-" + uuid.New().String() + "@example.com"
	outsider, err := s.CreateUser(ctx, email, "test-hash", "Outsider")
	if err != nil {
		t.Fatalf("create outsider: %v", err)
	}
	t.Cleanup(func() {
		_, _ = s.pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, outsider.ID)
	})

	err = s.MoveTask(ctx, outsider.ID, task.ID, cols[0], 0)
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("got %v, want ErrForbidden", err)
	}
}

func TestMoveTask_InvalidPosition(t *testing.T) {
	t.Parallel()
	s := testStore(t)

	err := s.MoveTask(context.Background(), uuid.New(), uuid.New(), uuid.New(), -1)
	if !errors.Is(err, ErrInvalidPosition) {
		t.Fatalf("got %v, want ErrInvalidPosition", err)
	}
}
