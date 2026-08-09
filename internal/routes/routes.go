// Package routes ประกอบ route เข้ากับ handler — ที่เดียวที่ดูแล้วรู้ว่า endpoint ไหนต้อง login
package routes

import (
	"github.com/labstack/echo/v4"

	"github.com/thitiphongD/blog-user-api/internal/auth"
	"github.com/thitiphongD/blog-user-api/internal/handler"
	"github.com/thitiphongD/blog-user-api/internal/middleware"
)

type Deps struct {
	JWT    *auth.JWT
	Health *handler.HealthHandler
	Auth   *handler.AuthHandler
	User   *handler.UserHandler
	Blog   *handler.BlogHandler
}

func Register(e *echo.Echo, d Deps) {
	protected := middleware.JWT(d.JWT)

	e.GET("/health", d.Health.Health)

	v1 := e.Group("/api/v1")

	authGroup := v1.Group("/auth")
	authGroup.POST("/register", d.Auth.Register)
	authGroup.POST("/login", d.Auth.Login)
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
