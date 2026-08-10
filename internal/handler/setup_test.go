package handler_test

import (
	"context"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/thitiphongD/blog-user-api/internal/apperr"
	"github.com/thitiphongD/blog-user-api/internal/auth"
	"github.com/thitiphongD/blog-user-api/internal/handler"
	"github.com/thitiphongD/blog-user-api/internal/model"
	"github.com/thitiphongD/blog-user-api/internal/response"
	"github.com/thitiphongD/blog-user-api/internal/routes"
	"github.com/thitiphongD/blog-user-api/internal/service"
	"github.com/thitiphongD/blog-user-api/internal/validator"
)

// handler ผูกกับ service struct ตรงๆ (ไม่ใช่ interface) เลย mock ที่ชั้น repository แทน
// ได้ของแถมคือเทสต์วิ่งผ่าน route + validator + global error handler ของจริง
// การ map error เป็น status เลยถูกเช็คไปด้วย ไม่ใช่เช็คแค่ค่าที่ handler return
type app struct {
	echo     *echo.Echo
	users    *fakeUserRepo
	blogs    *fakeBlogRepo
	comments *fakeCommentRepo
	refresh  *fakeRefreshRepo
	jwt      *auth.JWT
}

func newApp(t *testing.T) *app {
	t.Helper()

	users := &fakeUserRepo{}
	blogs := &fakeBlogRepo{}
	refresh := &fakeRefreshRepo{}
	comments := &fakeCommentRepo{}
	jwt := auth.NewJWT("test-secret", time.Hour)

	e := echo.New()
	e.Validator = validator.New()
	e.HTTPErrorHandler = response.ErrorHandler

	routes.Register(e, routes.Deps{
		JWT: jwt,
		// ตั้งสูงไว้ เทสต์ชุดนี้ไม่ได้ทดสอบ rate limit (มีเทสต์แยกของมันเอง)
		AuthRateLimitPerMinute: 1000,
		Health:                 handler.NewHealthHandler(&stubPinger{}),
		Auth: handler.NewAuthHandler(
			service.NewAuthService(users, refresh, passthroughTx{}, jwt, 7*24*time.Hour),
		),
		User:    handler.NewUserHandler(service.NewUserService(users)),
		Blog:    handler.NewBlogHandler(service.NewBlogService(blogs, passthroughTx{})),
		Comment: handler.NewCommentHandler(service.NewCommentService(comments, blogs, passthroughTx{})),
	})

	return &app{echo: e, users: users, blogs: blogs, comments: comments, refresh: refresh, jwt: jwt}
}

// do ยิง request จริงเข้า echo — token ว่างแปลว่าไม่แนบ Authorization
func (a *app) do(t *testing.T, method, target, body, token string) *httptest.ResponseRecorder {
	t.Helper()

	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}

	req := httptest.NewRequestWithContext(t.Context(), method, target, reader)
	if body != "" {
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	}
	if token != "" {
		req.Header.Set(echo.HeaderAuthorization, "Bearer "+token)
	}

	rec := httptest.NewRecorder()
	a.echo.ServeHTTP(rec, req)

	return rec
}

func (a *app) tokenFor(t *testing.T, userID uuid.UUID) string {
	t.Helper()

	token, _, err := a.jwt.Generate(userID)
	if err != nil {
		t.Fatalf("gen token: %v", err)
	}

	return token
}

func assertStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) map[string]any {
	t.Helper()

	body := decode(t, rec)
	if rec.Code != want {
		t.Fatalf("status = %d อยากได้ %d (body=%s)", rec.Code, want, rec.Body.String())
	}

	if body["status"] != float64(want) {
		t.Fatalf("status ใน body = %v ไม่ตรงกับ HTTP status %d", body["status"], want)
	}
	if body["request_id"] == nil || body["timestamp"] == nil {
		t.Fatalf("envelope ไม่ครบ: %s", rec.Body.String())
	}

	return body
}

func dataOf(t *testing.T, body map[string]any) map[string]any {
	t.Helper()

	data, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatalf("data ไม่ใช่ object: %v", body["data"])
	}

	return data
}

// —— mock ————————————————————————————————————————————————

type fakeUserRepo struct {
	findByEmail func(ctx context.Context, email string) (*model.User, error)
	findByID    func(ctx context.Context, id uuid.UUID) (*model.User, error)
	findAll     func(ctx context.Context, offset, limit int) ([]model.User, error)
	count       func(ctx context.Context) (int64, error)
	create      func(ctx context.Context, user *model.User) error
}

func (m *fakeUserRepo) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	return m.findByEmail(ctx, email)
}

func (m *fakeUserRepo) FindByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	return m.findByID(ctx, id)
}

func (m *fakeUserRepo) FindAll(ctx context.Context, offset, limit int) ([]model.User, error) {
	return m.findAll(ctx, offset, limit)
}

func (m *fakeUserRepo) Count(ctx context.Context) (int64, error) { return m.count(ctx) }

func (m *fakeUserRepo) Create(ctx context.Context, user *model.User) error {
	return m.create(ctx, user)
}

type fakeBlogRepo struct {
	findByID func(ctx context.Context, id uuid.UUID) (*model.Blog, error)
	findAll  func(ctx context.Context, f model.BlogFilter) ([]model.Blog, error)
	count    func(ctx context.Context, f model.BlogFilter) (int64, error)
	create   func(ctx context.Context, blog *model.Blog) error
	update   func(ctx context.Context, blog *model.Blog) error
	delete   func(ctx context.Context, id uuid.UUID) error
}

func (m *fakeBlogRepo) FindByID(ctx context.Context, id uuid.UUID) (*model.Blog, error) {
	return m.findByID(ctx, id)
}

func (m *fakeBlogRepo) FindAll(ctx context.Context, f model.BlogFilter) ([]model.Blog, error) {
	return m.findAll(ctx, f)
}

func (m *fakeBlogRepo) Count(ctx context.Context, f model.BlogFilter) (int64, error) {
	return m.count(ctx, f)
}

func (m *fakeBlogRepo) Create(ctx context.Context, blog *model.Blog) error {
	return m.create(ctx, blog)
}

func (m *fakeBlogRepo) Update(ctx context.Context, blog *model.Blog) error {
	return m.update(ctx, blog)
}

func (m *fakeBlogRepo) Delete(ctx context.Context, id uuid.UUID) error { return m.delete(ctx, id) }

// fakeRefreshRepo เก็บ token ไว้ใน map จำลอง DB — พอสำหรับเทสต์ชั้น HTTP
type fakeRefreshRepo struct {
	tokens  map[string]*model.RefreshToken
	revoked []uuid.UUID
}

func (m *fakeRefreshRepo) Create(_ context.Context, token *model.RefreshToken) error {
	if m.tokens == nil {
		m.tokens = map[string]*model.RefreshToken{}
	}

	token.ID = uuid.New()
	m.tokens[token.TokenHash] = token

	return nil
}

func (m *fakeRefreshRepo) FindByHash(_ context.Context, hash string) (*model.RefreshToken, error) {
	if token, ok := m.tokens[hash]; ok {
		return token, nil
	}

	return nil, apperr.NotFound("Refresh token")
}

// Revoke เลียนแบบ WHERE revoked_at IS NULL ของจริง — ใบที่ถูกเพิกถอนไปแล้วต้องตีกลับ
// ไม่ใช่คืน nil เฉยๆ ไม่งั้น fake จะโกหกว่า race ที่ repository จริงกันไว้ให้นั้นเกิดไม่ได้
func (m *fakeRefreshRepo) Revoke(_ context.Context, id uuid.UUID, at time.Time) error {
	for _, token := range m.tokens {
		if token.ID != id {
			continue
		}

		if token.RevokedAt != nil {
			return apperr.ErrInvalidRefresh
		}

		token.RevokedAt = &at
		m.revoked = append(m.revoked, id)

		return nil
	}

	return apperr.ErrInvalidRefresh
}

func (m *fakeRefreshRepo) DeleteExpired(_ context.Context, before time.Time) (int64, error) {
	var deleted int64

	for hash, token := range m.tokens {
		if token.ExpiresAt.Before(before) {
			delete(m.tokens, hash)
			deleted++
		}
	}

	return deleted, nil
}

func (m *fakeRefreshRepo) RevokeAllForUser(_ context.Context, userID uuid.UUID, at time.Time) error {
	for _, token := range m.tokens {
		if token.UserID == userID {
			token.RevokedAt = &at
		}
	}

	return nil
}

type fakeCommentRepo struct {
	findByID      func(ctx context.Context, id uuid.UUID) (*model.Comment, error)
	findAllByBlog func(ctx context.Context, blogID uuid.UUID, offset, limit int) ([]model.Comment, error)
	countByBlog   func(ctx context.Context, blogID uuid.UUID) (int64, error)
	create        func(ctx context.Context, comment *model.Comment) error
	update        func(ctx context.Context, comment *model.Comment) error
	delete        func(ctx context.Context, id uuid.UUID) error
}

func (m *fakeCommentRepo) FindByID(ctx context.Context, id uuid.UUID) (*model.Comment, error) {
	return m.findByID(ctx, id)
}

func (m *fakeCommentRepo) FindAllByBlog(
	ctx context.Context,
	blogID uuid.UUID,
	offset, limit int,
) ([]model.Comment, error) {
	return m.findAllByBlog(ctx, blogID, offset, limit)
}

func (m *fakeCommentRepo) CountByBlog(ctx context.Context, blogID uuid.UUID) (int64, error) {
	return m.countByBlog(ctx, blogID)
}

func (m *fakeCommentRepo) Create(ctx context.Context, comment *model.Comment) error {
	return m.create(ctx, comment)
}

func (m *fakeCommentRepo) Update(ctx context.Context, comment *model.Comment) error {
	return m.update(ctx, comment)
}

func (m *fakeCommentRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return m.delete(ctx, id)
}

// passthroughTx รัน fn ตรงๆ — transaction จริงถูกเทสต์ที่ชั้น service แล้ว
type passthroughTx struct{}

func (passthroughTx) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}
