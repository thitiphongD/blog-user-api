package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	"gorm.io/gorm"

	"github.com/thitiphongD/blog-user-api/internal/auth"
	"github.com/thitiphongD/blog-user-api/internal/config"
	"github.com/thitiphongD/blog-user-api/internal/database"
	"github.com/thitiphongD/blog-user-api/internal/handler"
	"github.com/thitiphongD/blog-user-api/internal/middleware"
	"github.com/thitiphongD/blog-user-api/internal/migrate"
	"github.com/thitiphongD/blog-user-api/internal/repository"
	"github.com/thitiphongD/blog-user-api/internal/response"
	"github.com/thitiphongD/blog-user-api/internal/routes"
	"github.com/thitiphongD/blog-user-api/internal/service"
	"github.com/thitiphongD/blog-user-api/internal/validator"
)

// @title						Blog User API
// @version					1.0
// @description				REST API สำหรับ blog — อ่านได้ทุกคน แก้ได้เฉพาะเจ้าของ
// @BasePath					/
// @securityDefinitions.apikey	BearerAuth
// @in							header
// @name						Authorization
// @description				ใส่ "Bearer " นำหน้า token ที่ได้จาก /api/v1/auth/login

const shutdownTimeout = 10 * time.Second

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	if err := run(); err != nil {
		slog.Error("server stopped with error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// ctx ตัวนี้ตายเมื่อได้ SIGINT/SIGTERM ใช้คุมทั้งตอน boot และตอน shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	db, err := database.Connect(ctx, cfg.DB, cfg.App.Env)
	if err != nil {
		return err
	}

	if err := migrate.Up(cfg.DB.URL()); err != nil {
		return err
	}
	slog.Info("migration up to date")

	e := newEcho(cfg, db)

	go func() {
		if err := e.Start(":" + cfg.App.Port); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("start server", "error", err)
			stop()
		}
	}()
	slog.Info("server started", "port", cfg.App.Port, "env", cfg.App.Env)

	<-ctx.Done()
	slog.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := e.Shutdown(shutdownCtx); err != nil {
		return err
	}

	if sqlDB, err := db.DB(); err == nil {
		_ = sqlDB.Close()
	}

	slog.Info("bye")

	return nil
}

func newEcho(cfg *config.Config, db *gorm.DB) *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.Validator = validator.New()
	e.HTTPErrorHandler = response.ErrorHandler

	// Echo ไม่ตั้ง timeout ให้ ไม่ตั้งเองคือเปิดรับ slowloris
	e.Server.ReadTimeout = cfg.Server.ReadTimeout
	e.Server.WriteTimeout = cfg.Server.WriteTimeout
	e.Server.IdleTimeout = cfg.Server.IdleTimeout
	e.Server.ReadHeaderTimeout = cfg.Server.ReadHeaderTimeout

	// Recover อยู่ชั้นในกว่า Logger ตั้งใจ — panic ถูกจับก่อน Logger เลยยัง log 500 ได้ตามปกติ
	e.Use(echomw.RequestID())
	e.Use(middleware.RequestContext())
	e.Use(middleware.Logger())
	e.Use(echomw.Recover())
	e.Use(echomw.CORS())

	jwt := auth.NewJWT(cfg.JWT.Secret, cfg.JWT.Expire)

	userRepo := repository.NewUserRepository(db)
	blogRepo := repository.NewBlogRepository(db)
	tx := repository.NewTxManager(db)

	routes.Register(e, routes.Deps{
		JWT:    jwt,
		Health: handler.NewHealthHandler(db),
		Auth:   handler.NewAuthHandler(service.NewAuthService(userRepo, jwt)),
		User:   handler.NewUserHandler(service.NewUserService(userRepo)),
		Blog:   handler.NewBlogHandler(service.NewBlogService(blogRepo, tx)),
	})

	return e
}
