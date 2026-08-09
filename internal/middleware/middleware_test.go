package middleware_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"

	"github.com/thitiphongD/blog-user-api/internal/apperr"
	"github.com/thitiphongD/blog-user-api/internal/auth"
	"github.com/thitiphongD/blog-user-api/internal/logging"
	"github.com/thitiphongD/blog-user-api/internal/middleware"
	"github.com/thitiphongD/blog-user-api/internal/response"
)

func newContext(t *testing.T, header string) echo.Context {
	t.Helper()

	e := echo.New()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	if header != "" {
		req.Header.Set(echo.HeaderAuthorization, header)
	}

	return e.NewContext(req, httptest.NewRecorder())
}

func TestJWTPassesUserIDThrough(t *testing.T) {
	j := auth.NewJWT("test-secret", time.Hour)
	id := uuid.New()

	token, _, err := j.Generate(id)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	c := newContext(t, "Bearer "+token)

	var got uuid.UUID
	handler := middleware.JWT(j)(func(c echo.Context) error {
		got, err = middleware.UserID(c)

		return err
	})

	if err := handler(c); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if got != id {
		t.Fatalf("user id = %v อยากได้ %v", got, id)
	}
}

func TestJWTRejects(t *testing.T) {
	j := auth.NewJWT("test-secret", time.Hour)

	valid, _, err := j.Generate(uuid.New())
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	cases := []struct {
		name   string
		header string
	}{
		{"ไม่มี header", ""},
		{"ไม่มีคำว่า Bearer", valid},
		{"Bearer แต่ token ขยะ", "Bearer abc.def.ghi"},
		{"พิมพ์เล็ก", "bearer " + valid},
		{"Basic auth", "Basic dXNlcjpwYXNz"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := newContext(t, tc.header)

			called := false
			handler := middleware.JWT(j)(func(echo.Context) error {
				called = true

				return nil
			})

			err := handler(c)
			if !errors.Is(err, apperr.ErrUnauthorized) {
				t.Fatalf("อยากได้ ErrUnauthorized ได้ %v", err)
			}
			if called {
				t.Fatal("หลุดไปถึง handler ทั้งที่ไม่ควรผ่าน")
			}
		})
	}
}

// เรียก UserID บน route ที่ไม่ได้ผ่าน JWT ต้องได้ error ไม่ใช่ uuid.Nil เงียบๆ
func TestUserIDWithoutMiddleware(t *testing.T) {
	c := newContext(t, "")

	if _, err := middleware.UserID(c); !errors.Is(err, apperr.ErrUnauthorized) {
		t.Fatalf("อยากได้ ErrUnauthorized ได้ %v", err)
	}
}

// request_id ต้องเดินไปกับ context.Context ด้วย ไม่ใช่อยู่แค่ใน echo.Context
// ไม่งั้น service กับ repository log แล้วตามไม่ได้
func TestRequestContextCarriesRequestIDIntoContext(t *testing.T) {
	e := echo.New()
	e.Use(echomw.RequestID())
	e.Use(middleware.RequestContext())

	var got string
	e.GET("/", func(c echo.Context) error {
		got = logging.RequestID(c.Request().Context())

		return c.NoContent(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))

	if got == "" {
		t.Fatal("context ไม่มี request id")
	}
	if header := rec.Header().Get(echo.HeaderXRequestID); got != header {
		t.Fatalf("request id ใน context (%s) ไม่ตรงกับใน header (%s)", got, header)
	}
}

func TestLoggerWritesAccessLog(t *testing.T) {
	var buf bytes.Buffer
	restore := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	defer slog.SetDefault(restore)

	e := echo.New()
	e.Use(echomw.RequestID())
	e.Use(middleware.RequestContext())
	e.Use(middleware.Logger())
	e.GET("/blogs", func(c echo.Context) error { return c.NoContent(http.StatusOK) })

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/blogs", nil))

	line := lastLine(t, &buf)

	if line["msg"] != "request" {
		t.Fatalf("msg = %v", line["msg"])
	}
	if line["path"] != "/blogs" || line["method"] != http.MethodGet {
		t.Fatalf("log ไม่ครบ: %v", line)
	}
	if line["request_id"] == nil || line["request_id"] == "" {
		t.Fatalf("access log ไม่มี request_id: %v", line)
	}
	if line["level"] != "INFO" {
		t.Fatalf("level = %v อยากได้ INFO", line["level"])
	}
}

// level ต้องผูกกับ status ไม่งั้น grep หา error ใน log ไม่เจอของจริง
func TestLoggerLevelFollowsStatus(t *testing.T) {
	cases := []struct {
		name   string
		status int
		want   string
	}{
		{"4xx เป็น WARN", http.StatusBadRequest, "WARN"},
		{"5xx เป็น ERROR", http.StatusInternalServerError, "ERROR"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			restore := slog.Default()
			slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
			defer slog.SetDefault(restore)

			e := echo.New()
			e.Use(middleware.Logger())
			e.GET("/", func(c echo.Context) error { return c.NoContent(tc.status) })

			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))

			if line := lastLine(t, &buf); line["level"] != tc.want {
				t.Fatalf("level = %v อยากได้ %s", line["level"], tc.want)
			}
		})
	}
}

func lastLine(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()

	lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	if len(lines) == 0 || len(lines[0]) == 0 {
		t.Fatal("ไม่มี log ออกมาเลย")
	}

	var line map[string]any
	if err := json.Unmarshal(lines[len(lines)-1], &line); err != nil {
		t.Fatalf("อ่าน log ไม่ออก: %v (%s)", err, buf.String())
	}

	return line
}

// handler คืน error → Logger ต้องส่งต่อให้ ErrorHandler เขียน response ก่อน
// แล้วค่อย log ด้วย status จริง พร้อมแนบข้อความ error ไว้ให้ตามได้
func TestLoggerRecordsHandlerError(t *testing.T) {
	var buf bytes.Buffer
	restore := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	defer slog.SetDefault(restore)

	e := echo.New()
	e.HTTPErrorHandler = response.ErrorHandler
	e.Use(echomw.RequestID())
	e.Use(middleware.RequestContext())
	e.Use(middleware.Logger())
	e.GET("/", func(echo.Context) error { return apperr.ErrForbidden })

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))

	line := lastLine(t, &buf)

	if line["error"] != apperr.ErrForbidden.Error() {
		t.Fatalf("ไม่ได้แนบ error ไว้ใน log: %v", line)
	}
	if line["status"] != float64(http.StatusForbidden) {
		t.Fatalf("status ใน log = %v — log ก่อนที่ response จะถูกเขียนจริง", line["status"])
	}
	if line["level"] != "WARN" {
		t.Fatalf("level = %v อยากได้ WARN", line["level"])
	}
}

// perMinute <= 0 ต้องแปลว่า "ปิด rate limit" ไม่ใช่ "ห้ามยิงเลยสักครั้ง"
// ซึ่งเป็นสิ่งที่ limiter จะทำถ้าปล่อย burst เป็น 0
func TestRateLimitDisabledWhenZero(t *testing.T) {
	for _, perMinute := range []int{0, -1} {
		t.Run(strconv.Itoa(perMinute), func(t *testing.T) {
			e := echo.New()
			e.Use(middleware.RateLimitAuth(perMinute))
			e.GET("/", func(c echo.Context) error { return c.NoContent(http.StatusOK) })

			for i := range 20 {
				rec := httptest.NewRecorder()
				e.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))

				if rec.Code != http.StatusOK {
					t.Fatalf("ครั้งที่ %d ได้ status %d — ปิดอยู่แต่ดันบล็อก", i+1, rec.Code)
				}
			}
		})
	}
}

func TestRateLimitBlocksAfterQuota(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = response.ErrorHandler
	e.Use(middleware.RateLimitAuth(3))
	e.GET("/", func(c echo.Context) error { return c.NoContent(http.StatusOK) })

	codes := make([]int, 0, 5)
	for range 5 {
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
		codes = append(codes, rec.Code)
	}

	for i := range 3 {
		if codes[i] != http.StatusOK {
			t.Fatalf("ยังไม่เกินโควตาแต่ครั้งที่ %d โดนบล็อก: %v", i+1, codes)
		}
	}
	if codes[3] != http.StatusTooManyRequests || codes[4] != http.StatusTooManyRequests {
		t.Fatalf("เกินโควตาแล้วไม่โดนบล็อก: %v", codes)
	}
}
