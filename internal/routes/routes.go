// Package routes ประกอบ route เข้ากับ handler — ที่เดียวที่ดูแล้วรู้ว่า endpoint ไหนต้อง login
package routes

import (
	"github.com/labstack/echo/v4"
	echoswagger "github.com/swaggo/echo-swagger"

	_ "github.com/thitiphongD/blog-user-api/docs" // spec ที่ swag gen ไว้ ต้อง import ถึงจะโผล่
	"github.com/thitiphongD/blog-user-api/internal/auth"
	"github.com/thitiphongD/blog-user-api/internal/handler"
	"github.com/thitiphongD/blog-user-api/internal/middleware"
)

type Deps struct {
	JWT                    *auth.JWT
	AuthRateLimitPerMinute int
	Health                 *handler.HealthHandler
	Auth                   *handler.AuthHandler
	User                   *handler.UserHandler
	Blog                   *handler.BlogHandler
}

func Register(e *echo.Echo, d Deps) {
	protected := middleware.JWT(d.JWT)

	e.GET("/health", d.Health.Health)
	e.GET("/swagger/*", echoswagger.WrapHandler)

	v1 := e.Group("/api/v1")

	// rate limit เฉพาะ login กับ register ซึ่งเป็นเป้าของการเดารหัส
	// refresh/logout ไม่ต้อง — token สุ่ม 256 bit เดาไม่ได้อยู่แล้ว และ client ปกติยิง refresh บ่อย
	// จะโดนจำกัดไปด้วยเปล่าๆ ส่วน /blogs ที่เป็น public ยังไม่มีเหตุผลให้จำกัด
	guessGuard := middleware.RateLimitAuth(d.AuthRateLimitPerMinute)

	authGroup := v1.Group("/auth")
	authGroup.POST("/register", d.Auth.Register, guessGuard)
	authGroup.POST("/login", d.Auth.Login, guessGuard)
	authGroup.POST("/refresh", d.Auth.Refresh)
	authGroup.POST("/logout", d.Auth.Logout)
	authGroup.GET("/me", d.Auth.Me, protected)

	// /users ปิดทั้งกลุ่ม ไม่เปิด email ชาวบ้าน
	users := v1.Group("/users", protected)
	users.GET("", d.User.List)
	users.GET("/:id", d.User.Get)

	// /blogs อ่านได้ไม่ต้อง login เขียนต้อง login
	blogs := v1.Group("/blogs")
	blogs.GET("", d.Blog.List)
	blogs.GET("/:id", d.Blog.Get)
	blogs.POST("", d.Blog.Create, protected)
	blogs.PUT("/:id", d.Blog.Update, protected)
	blogs.DELETE("/:id", d.Blog.Delete, protected)
}
