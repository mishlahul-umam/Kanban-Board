# Kanban Board

Mini Kanban app: **Go (Fiber)** + **PostgreSQL** backend and **React (Vite)** frontend. Auth via JWT, drag-and-drop columns, task details (assignee, due date, comments), and WebSocket broadcasts when tasks or comments change.

## Prerequisites

- [Go](https://go.dev/) 1.22+ and [Node.js](https://nodejs.org/) 20+ (for local dev without Docker)
- [Docker](https://docs.docker.com/get-docker/) + Docker Compose (for containerized run)
- Or PostgreSQL 14+ running locally (local dev only)

## Docker (recommended)

From the repo root:

```bash
docker compose up --build
```

Open **http://localhost** (port **80**). The `frontend` service serves the SPA and proxies `/auth`, `/boards`, `/columns`, `/tasks`, and `/ws` to the Go API.

Optional: copy [`env.docker.example`](env.docker.example) to `.env` beside `docker-compose.yml`. Useful variables: `JWT_SECRET`, `POSTGRES_PASSWORD`, `FRONTEND_PORT` (host port mapped to the web UI, default **80**), and `CORS_ORIGIN` (must match the URL you use in the browser, e.g. `http://localhost:3000` if `FRONTEND_PORT=3000`).

Data is stored in the `pgdata` named volume.

## Local development (no Docker)

### Database

Create a database (example):

```sql
CREATE DATABASE kanban_dev;
```

### Backend

```bash
cd backend
copy .env.example .env   # Windows; use cp on Unix
# Edit .env: DATABASE_URL, JWT_SECRET
go run ./cmd/server
```

Default server: `http://localhost:8080`

Migrations run automatically on startup (embedded SQL in `internal/db/migrations`).

### Environment variables

| Variable       | Description                                      |
|----------------|--------------------------------------------------|
| `PORT`         | HTTP port (default `8080`)                       |
| `DATABASE_URL` | PostgreSQL URL, e.g. `postgres://user:pass@localhost:5432/kanban_dev?sslmode=disable` |
| `JWT_SECRET`   | Secret for signing JWTs                          |
| `CORS_ORIGIN`  | Allowed browser origin (default `http://localhost:5173`) |

### Frontend

```bash
cd frontend
copy .env.example .env
# Optional: VITE_API_URL=http://localhost:8080
npm install
npm run dev
```

Open `http://localhost:5173`.

If `VITE_API_URL` is unset, the Vite dev server proxies `/auth`, `/boards`, `/columns`, `/tasks`, and `/ws` to `localhost:8080`.

## WebSocket

- **Local dev (Vite):** `ws://localhost:8080/ws/boards/:boardId?token=<JWT>` (direct to Fiber).
- **Docker:** same host as the UI, path `/ws/...` (proxied by nginx to the backend).

The server broadcasts JSON events (`task_moved`, `task_updated`, `comment_created`, etc.) to connections on that board. The board page subscribes and refetches board data on each message.

## API overview

- `POST /auth/register`, `POST /auth/login`, `GET /auth/me`
- `GET/POST /boards`, `GET/PATCH/DELETE /boards/:id`
- `POST /boards/:id/columns`, `PATCH/DELETE /columns/:id`
- `POST /columns/:id/tasks`, `PATCH/DELETE /tasks/:id`, `PATCH /tasks/:id/move`
- `GET/POST /tasks/:id/comments`

## Project layout

- `backend/` — Fiber app, pgx pool, JWT, migrations, WebSocket hub
- `frontend/` — React, TanStack Query, dnd-kit, Tailwind
- `docker-compose.yml` — Postgres + API + nginx (production build of the UI)
