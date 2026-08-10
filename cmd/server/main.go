package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"slices"
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

	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get sql.DB: %w", err)
	}
	defer func() { _ = sqlDB.Close() }()

	if err := migrate.Up(cfg.DB.URL()); err != nil {
		return err
	}
	slog.Info("migration up to date")

	e, authService := newEcho(cfg, db, sqlDB)
	startTokenPruner(ctx, authService, cfg.JWT.RefreshPruneInterval)

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

	slog.Info("bye")

	return nil
}

func newEcho(cfg *config.Config, db *gorm.DB, pinger handler.Pinger) (*echo.Echo, *service.AuthService) {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.Validator = validator.New()
	e.HTTPErrorHandler = response.ErrorHandler

	// ไม่ปักอันนี้ c.RealIP() จะอ่าน X-Forwarded-For ก่อน RemoteAddr เสมอ ซึ่งแปลว่า
	// rate limit ที่ครอบ login/register หนีได้ด้วยการสุ่ม header ใหม่ทุก request
	// และ remote_ip ใน access log ก็ปลอมได้ตามใจ — ไม่มี proxy อยู่หน้า service นี้
	// วันไหนมีค่อยเปลี่ยนเป็น ExtractIPFromXFFHeader พร้อมระบุ range ที่เชื่อถือได้
	e.IPExtractor = echo.ExtractIPDirect()

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

	// echomw.CORS() เปล่าๆ = AllowOrigins ["*"] ตายตัว ให้ตั้งผ่าน env แทน
	// token อยู่ใน header ไม่ใช่ cookie ความเสี่ยงเลยไม่สูงเท่าเคสที่ใช้ cookie
	// แต่ปล่อยเปิดหมดตอน deploy จริงไม่มีเหตุผลรองรับ
	if slices.Contains(cfg.CORS.AllowedOrigins, "*") {
		slog.Warn("CORS เปิดให้ทุก origin — ตั้ง CORS_ALLOWED_ORIGINS ก่อน deploy จริง",
			"env", cfg.App.Env)
	}

	e.Use(echomw.CORSWithConfig(echomw.CORSConfig{AllowOrigins: cfg.CORS.AllowedOrigins}))

	jwt := auth.NewJWT(cfg.JWT.Secret, cfg.JWT.AccessTTL)

	userRepo := repository.NewUserRepository(db)
	blogRepo := repository.NewBlogRepository(db)
	refreshRepo := repository.NewRefreshTokenRepository(db)
	commentRepo := repository.NewCommentRepository(db)
	tx := repository.NewTxManager(db)

	authService := service.NewAuthService(userRepo, refreshRepo, tx, jwt, cfg.JWT.RefreshTTL)

	routes.Register(e, routes.Deps{
		JWT:                    jwt,
		AuthRateLimitPerMinute: cfg.RateLimit.AuthPerMinute,
		Health:                 handler.NewHealthHandler(pinger),
		Auth:                   handler.NewAuthHandler(authService),
		User:                   handler.NewUserHandler(service.NewUserService(userRepo)),
		Blog:                   handler.NewBlogHandler(service.NewBlogService(blogRepo, tx)),
		Comment:                handler.NewCommentHandler(service.NewCommentService(commentRepo, blogRepo, tx)),
	})

	return e, authService
}

// startTokenPruner กวาด refresh token ที่หมดอายุทิ้งเป็นระยะ — ตารางนี้ได้แถวใหม่ทุกครั้ง
// ที่มีคน login หรือ refresh และไม่มีอะไรลบให้ ปล่อยไว้คือโตไปเรื่อยๆ ไม่มีเพดาน
// ctx ตัวเดียวกับที่ใช้คุม shutdown ปิดเครื่องเมื่อไหร่ก็จบตาม
func startTokenPruner(ctx context.Context, auth *service.AuthService, every time.Duration) {
	if every <= 0 {
		slog.Info("ปิดการกวาด refresh token ที่หมดอายุ")

		return
	}

	go func() {
		ticker := time.NewTicker(every)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				deleted, err := auth.PruneExpiredTokens(ctx)
				if err != nil {
					slog.Error("กวาด refresh token ไม่สำเร็จ", "error", err)

					continue
				}
				if deleted > 0 {
					slog.Info("กวาด refresh token ที่หมดอายุแล้ว", "deleted", deleted)
				}
			}
		}
	}()
}
