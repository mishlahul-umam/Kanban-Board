package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"kanban/backend/internal/config"
	"kanban/backend/internal/db"
	"kanban/backend/internal/handlers"
	"kanban/backend/internal/middleware"
	"kanban/backend/internal/store"
	"kanban/backend/internal/ws"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	if err := db.Migrate(cfg.DatabaseURL); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	pool, err := db.NewPool(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db pool: %v", err)
	}
	defer pool.Close()

	app := fiber.New()
	app.Use(recover.New())
	app.Use(logger.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins:     cfg.CORSOrigin,
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization",
		AllowMethods:     "GET,POST,PATCH,DELETE,OPTIONS",
		AllowCredentials: true,
	}))

	hub := ws.NewHub()
	h := &handlers.Handlers{
		Store:     store.New(pool),
		JWTSecret: cfg.JWTSecret,
		Hub:       hub,
	}

	authLimiter := limiter.New(limiter.Config{
		Max:        10,
		Expiration: 1 * time.Minute,
	})
	app.Post("/auth/register", authLimiter, h.Register)
	app.Post("/auth/login", authLimiter, h.Login)

	protected := app.Group("", middleware.JWT(cfg.JWTSecret))
	protected.Get("/auth/me", h.Me)
	protected.Get("/boards", h.ListBoards)
	protected.Post("/boards", h.CreateBoard)
	protected.Get("/boards/:id", h.GetBoard)
	protected.Patch("/boards/:id", h.PatchBoard)
	protected.Delete("/boards/:id", h.DeleteBoard)
	protected.Post("/boards/:id/members", h.InviteMember)
	protected.Delete("/boards/:id/members/:userId", h.RemoveMember)
	protected.Post("/boards/:id/columns", h.CreateColumn)
	protected.Patch("/columns/:id", h.PatchColumn)
	protected.Delete("/columns/:id", h.DeleteColumn)
	protected.Post("/columns/:id/tasks", h.CreateTask)
	protected.Patch("/tasks/:id", h.PatchTask)
	protected.Patch("/tasks/:id/move", h.MoveTask)
	protected.Delete("/tasks/:id", h.DeleteTask)
	protected.Get("/tasks/:id/comments", h.ListComments)
	protected.Post("/tasks/:id/comments", h.CreateComment)

	app.Use("/ws/boards/:id", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})
	app.Get("/ws/boards/:id", websocket.New(h.BoardWebSocket))

	go func() {
		if err := app.Listen(":" + cfg.Port); err != nil {
			log.Printf("listen: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := app.ShutdownWithContext(ctx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}
