package handler_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/thitiphongD/blog-user-api/internal/auth"
	"github.com/thitiphongD/blog-user-api/internal/handler"
	"github.com/thitiphongD/blog-user-api/internal/model"
	"github.com/thitiphongD/blog-user-api/internal/response"
	"github.com/thitiphongD/blog-user-api/internal/routes"
	"github.com/thitiphongD/blog-user-api/internal/service"
	"github.com/thitiphongD/blog-user-api/internal/validator"
)

// loginFor พา user ผ่าน flow login จริงเพื่อให้ได้ refresh token ที่ระบบออกให้เอง
func loginFor(t *testing.T, app *app) string {
	t.Helper()

	hashed, err := auth.HashPassword("password123")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}

	userID := uuid.New()
	app.users.findByEmail = func(context.Context, string) (*model.User, error) {
		return &model.User{ID: userID, Email: "daew@example.com", Password: hashed}, nil
	}
	app.users.findByID = func(_ context.Context, id uuid.UUID) (*model.User, error) {
		return &model.User{ID: id, Email: "daew@example.com"}, nil
	}

	rec := app.do(t, http.MethodPost, "/api/v1/auth/login",
		`{"email":"daew@example.com","password":"password123"}`, "")

	data := dataOf(t, assertStatus(t, rec, http.StatusOK))

	token, ok := data["refresh_token"].(string)
	if !ok || token == "" {
		t.Fatalf("login ไม่ได้คืน refresh_token: %v", data)
	}

	return token
}

func TestLoginReturnsRefreshToken(t *testing.T) {
	app := newApp(t)
	refreshToken := loginFor(t, app)

	if len(refreshToken) < 40 {
		t.Fatalf("refresh token สั้นผิดปกติ: %q", refreshToken)
	}
}

func TestRefreshRotates(t *testing.T) {
	app := newApp(t)
	first := loginFor(t, app)

	rec := app.do(t, http.MethodPost, "/api/v1/auth/refresh", `{"refresh_token":"`+first+`"}`, "")

	body := assertStatus(t, rec, http.StatusOK)
	if body["message"] != "Token refreshed" {
		t.Fatalf("message = %v", body["message"])
	}

	data := dataOf(t, body)

	second, _ := data["refresh_token"].(string)
	if second == "" || second == first {
		t.Fatal("ไม่ได้ refresh token ใบใหม่")
	}
	if token, _ := data["token"].(string); token == "" {
		t.Fatal("ไม่ได้ access token ใบใหม่")
	}

	// ใบเก่าต้องใช้ต่อไม่ได้แล้ว
	again := app.do(t, http.MethodPost, "/api/v1/auth/refresh", `{"refresh_token":"`+first+`"}`, "")
	assertStatus(t, again, http.StatusUnauthorized)
}

// ใช้ token เก่าซ้ำ = ระบบต้องตัดทุก session ทิ้ง ใบใหม่ที่เพิ่งได้ก็ต้องใช้ไม่ได้ด้วย
func TestRefreshReuseKillsEverySession(t *testing.T) {
	app := newApp(t)
	first := loginFor(t, app)

	rec := app.do(t, http.MethodPost, "/api/v1/auth/refresh", `{"refresh_token":"`+first+`"}`, "")
	second, _ := dataOf(t, assertStatus(t, rec, http.StatusOK))["refresh_token"].(string)

	// คนร้ายเอาใบเก่ามายิง
	stolen := app.do(t, http.MethodPost, "/api/v1/auth/refresh", `{"refresh_token":"`+first+`"}`, "")
	assertStatus(t, stolen, http.StatusUnauthorized)

	// ใบที่เจ้าของถืออยู่ต้องตายตามไปด้วย
	owner := app.do(t, http.MethodPost, "/api/v1/auth/refresh", `{"refresh_token":"`+second+`"}`, "")
	body := assertStatus(t, owner, http.StatusUnauthorized)

	if body["message"] != "Invalid or expired refresh token" {
		t.Fatalf("message = %v", body["message"])
	}
}

func TestRefreshRejectsUnknownToken(t *testing.T) {
	app := newApp(t)

	rec := app.do(t, http.MethodPost, "/api/v1/auth/refresh", `{"refresh_token":"ไม่เคยมีอยู่จริง"}`, "")

	assertStatus(t, rec, http.StatusUnauthorized)
}

func TestRefreshValidation(t *testing.T) {
	app := newApp(t)

	rec := app.do(t, http.MethodPost, "/api/v1/auth/refresh", `{}`, "")
	body := assertStatus(t, rec, http.StatusUnprocessableEntity)

	if errs, ok := body["errors"].(map[string]any); !ok || errs["refresh_token"] == nil {
		t.Fatalf("errors = %v", body["errors"])
	}

	broken := app.do(t, http.MethodPost, "/api/v1/auth/refresh", `{oops`, "")
	assertStatus(t, broken, http.StatusBadRequest)
}

func TestLogoutThenRefreshFails(t *testing.T) {
	app := newApp(t)
	token := loginFor(t, app)

	rec := app.do(t, http.MethodPost, "/api/v1/auth/logout", `{"refresh_token":"`+token+`"}`, "")

	body := assertStatus(t, rec, http.StatusOK)
	if body["message"] != "Logout successfully" {
		t.Fatalf("message = %v", body["message"])
	}

	again := app.do(t, http.MethodPost, "/api/v1/auth/refresh", `{"refresh_token":"`+token+`"}`, "")
	assertStatus(t, again, http.StatusUnauthorized)
}

func TestLogoutWithUnknownToken(t *testing.T) {
	app := newApp(t)

	rec := app.do(t, http.MethodPost, "/api/v1/auth/logout", `{"refresh_token":"ไม่มีอยู่จริง"}`, "")

	assertStatus(t, rec, http.StatusUnauthorized)
}

// ยิงถี่เกินโควตาต้องได้ 429 ที่อยู่ใน envelope เดียวกับที่อื่น ไม่ใช่ 500
func TestRateLimitReturns429InEnvelope(t *testing.T) {
	e := echo.New()
	e.Validator = validator.New()
	e.HTTPErrorHandler = response.ErrorHandler

	users := &fakeUserRepo{}
	jwt := auth.NewJWT("test-secret", time.Minute)

	routes.Register(e, routes.Deps{
		JWT:                    jwt,
		AuthRateLimitPerMinute: 2,
		Health:                 handler.NewHealthHandler(&stubPinger{}),
		Auth: handler.NewAuthHandler(
			service.NewAuthService(users, &fakeRefreshRepo{}, passthroughTx{}, jwt, time.Hour),
		),
		User: handler.NewUserHandler(service.NewUserService(users)),
		Blog: handler.NewBlogHandler(service.NewBlogService(&fakeBlogRepo{}, passthroughTx{})),
	})

	limited := &app{echo: e, users: users, jwt: jwt}

	var last int
	for range 5 {
		rec := limited.do(t, http.MethodPost, "/api/v1/auth/login", `{oops`, "")
		last = rec.Code

		if last == http.StatusTooManyRequests {
			body := decode(t, rec)
			if body["message"] != "Too many requests, please try again later" {
				t.Fatalf("message = %v", body["message"])
			}
			if body["request_id"] == nil || body["timestamp"] == nil {
				t.Fatalf("429 หลุด envelope: %s", rec.Body.String())
			}

			return
		}
	}

	t.Fatalf("ยิง 5 ครั้งด้วยโควตา 2/นาที แต่ไม่โดนจำกัดเลย (status สุดท้าย %d)", last)
}
