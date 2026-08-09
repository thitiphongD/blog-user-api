package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/thitiphongD/blog-user-api/internal/handler"
)

// stubPinger แทน *sql.DB — นี่คือเหตุผลที่ health handler รับ interface ไม่ใช่ *gorm.DB
type stubPinger struct {
	err    error
	gotCtx context.Context // เก็บไว้ยืนยันว่า ctx ถูกส่งต่อจริง
	calls  int
}

func (s *stubPinger) PingContext(ctx context.Context) error {
	s.calls++
	s.gotCtx = ctx

	return s.err
}

func newHealthRequest(t *testing.T) (echo.Context, *httptest.ResponseRecorder) {
	t.Helper()

	e := echo.New()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Response().Header().Set(echo.HeaderXRequestID, "test-request-id")

	return c, rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("อ่าน response ไม่ออก: %v (body=%s)", err, rec.Body.String())
	}

	return body
}

func TestHealthOK(t *testing.T) {
	c, rec := newHealthRequest(t)
	pinger := &stubPinger{}

	if err := handler.NewHealthHandler(pinger).Health(c); err != nil {
		t.Fatalf("health: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d อยากได้ 200", rec.Code)
	}

	body := decode(t, rec)

	if body["success"] != true {
		t.Fatalf("success = %v อยากได้ true", body["success"])
	}

	data, ok := body["data"].(map[string]any)
	if !ok || data["database"] != "up" {
		t.Fatalf("data = %v อยากได้ database: up", body["data"])
	}

	if body["request_id"] != "test-request-id" {
		t.Fatalf("request_id = %v ไม่ได้เอามาจาก header", body["request_id"])
	}
	if body["timestamp"] == nil || body["timestamp"] == "" {
		t.Fatal("ไม่มี timestamp ใน envelope")
	}
}

// health ต้อง ping DB จริง ไม่ใช่ตอบ 200 ลอยๆ ไม่งั้น docker healthcheck ก็ไร้ความหมาย
func TestHealthPingsDatabase(t *testing.T) {
	c, _ := newHealthRequest(t)
	pinger := &stubPinger{}

	if err := handler.NewHealthHandler(pinger).Health(c); err != nil {
		t.Fatalf("health: %v", err)
	}

	if pinger.calls != 1 {
		t.Fatalf("เรียก PingContext %d ครั้ง อยากได้ 1", pinger.calls)
	}
	if pinger.gotCtx != c.Request().Context() {
		t.Fatal("ไม่ได้ส่ง context ของ request ลงไป — client ตัดสายแล้ว ping จะไม่ยกเลิกตาม")
	}
}

func TestHealthFailsWhenDatabaseDown(t *testing.T) {
	c, rec := newHealthRequest(t)
	pinger := &stubPinger{err: errors.New("dial tcp 127.0.0.1:5432: connect: connection refused")}

	if err := handler.NewHealthHandler(pinger).Health(c); err != nil {
		t.Fatalf("health: %v", err)
	}

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d อยากได้ 500 — DB ล่มแล้วยังตอบ healthy ไม่ได้", rec.Code)
	}

	body := decode(t, rec)

	if body["success"] != false {
		t.Fatalf("success = %v อยากได้ false", body["success"])
	}
	if body["message"] != "Internal server error" {
		t.Fatalf("message = %v", body["message"])
	}

	// 500 ห้ามคาย error จริงออกไป ต้องให้ตาม request_id ใน log เอา
	if strings.Contains(rec.Body.String(), "connection refused") {
		t.Fatalf("error จริงหลุดออก response: %s", rec.Body.String())
	}
}
