package response_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/thitiphongD/blog-user-api/internal/apperr"
	"github.com/thitiphongD/blog-user-api/internal/response"
	"github.com/thitiphongD/blog-user-api/internal/validator"
)

func newContext(t *testing.T) (echo.Context, *httptest.ResponseRecorder) {
	t.Helper()

	e := echo.New()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Response().Header().Set(echo.HeaderXRequestID, "req-1")

	return c, rec
}

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()

	var body map[string]any
	if rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("อ่าน response ไม่ออก: %v (%s)", err, rec.Body.String())
		}
	}

	return body
}

func handle(t *testing.T, err error) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()

	c, rec := newContext(t)
	response.ErrorHandler(err, c)

	return rec, decodeBody(t, rec)
}

func TestErrorHandlerMapsDomainErrors(t *testing.T) {
	cases := []struct {
		name    string
		err     error
		status  int
		message string
	}{
		{"not found", apperr.NotFound("Blog"), http.StatusNotFound, "Blog not found"},
		{"email ซ้ำ", apperr.ErrEmailTaken, http.StatusConflict, "Email already exists"},
		{"login ผิด", apperr.ErrInvalidCredential, http.StatusUnauthorized, "Invalid email or password"},
		{"ไม่มีสิทธิ์", apperr.ErrForbidden, http.StatusForbidden, "Permission denied"},
		{"ไม่ได้ login", apperr.ErrUnauthorized, http.StatusUnauthorized, "Unauthorized"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec, body := handle(t, tc.err)

			if rec.Code != tc.status {
				t.Fatalf("status = %d อยากได้ %d", rec.Code, tc.status)
			}
			if body["message"] != tc.message {
				t.Fatalf("message = %v อยากได้ %q", body["message"], tc.message)
			}
			if body["success"] != false {
				t.Fatalf("success = %v", body["success"])
			}
			if body["request_id"] != "req-1" {
				t.Fatalf("request_id = %v", body["request_id"])
			}
		})
	}
}

// error ที่ถูก wrap ด้วย fmt.Errorf ต้องยัง map ถูก — นี่คือเหตุผลที่เปิด errorlint ไว้
func TestErrorHandlerUnwrapsWrappedErrors(t *testing.T) {
	rec, body := handle(t, errors.Join(errors.New("ชั้นนอก"), apperr.ErrForbidden))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d — errors.Is ไม่ได้ทะลุชั้นที่ห่อไว้", rec.Code)
	}
	if body["message"] != "Permission denied" {
		t.Fatalf("message = %v", body["message"])
	}
}

func TestErrorHandlerValidation(t *testing.T) {
	rec, body := handle(t, &validator.Error{Fields: map[string]string{"title": "Title is required"}})

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d อยากได้ 422", rec.Code)
	}
	if body["message"] != "Validation failed" {
		t.Fatalf("message = %v", body["message"])
	}

	errs, ok := body["errors"].(map[string]any)
	if !ok || errs["title"] != "Title is required" {
		t.Fatalf("errors = %v", body["errors"])
	}
}

// error ที่ไม่รู้จักต้องกลายเป็น 500 ที่ไม่คายอะไรออกไป แต่ต้อง log ไว้ฝั่ง server พร้อม request_id
func TestErrorHandlerHidesUnexpectedError(t *testing.T) {
	var buf bytes.Buffer
	restore := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	defer slog.SetDefault(restore)

	rec, body := handle(t, errors.New("dial tcp 10.0.0.1:5432: connect: connection refused"))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d อยากได้ 500", rec.Code)
	}
	if body["message"] != "Internal server error" {
		t.Fatalf("message = %v", body["message"])
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("connection refused")) {
		t.Fatalf("error จริงหลุดออก response: %s", rec.Body.String())
	}

	if !bytes.Contains(buf.Bytes(), []byte("connection refused")) {
		t.Fatalf("ไม่ได้ log error จริงไว้ฝั่ง server เลย: %s", buf.String())
	}
	if !bytes.Contains(buf.Bytes(), []byte("req-1")) {
		t.Fatalf("log ไม่มี request_id ตามไม่ได้: %s", buf.String())
	}
}

// route ที่ไม่มีอยู่ echo โยน HTTPError 404 มา ต้องออกมาเป็น envelope เดียวกับที่อื่น
func TestErrorHandlerEchoHTTPError(t *testing.T) {
	rec, body := handle(t, echo.NewHTTPError(http.StatusNotFound, "Not Found"))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d", rec.Code)
	}
	if body["success"] != false || body["timestamp"] == nil {
		t.Fatalf("ไม่ได้อยู่ใน envelope เดียวกัน: %s", rec.Body.String())
	}
}

// เขียน response ไปแล้วห้ามเขียนซ้ำ ไม่งั้น body จะพังเป็น JSON สองก้อนต่อกัน
func TestErrorHandlerSkipsCommittedResponse(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := response.Success(c, map[string]string{"ok": "1"}); err != nil {
		t.Fatalf("write: %v", err)
	}

	response.ErrorHandler(apperr.ErrForbidden, c)

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body พังเพราะเขียนซ้ำ: %s", rec.Body.String())
	}
	if body["status"] != float64(http.StatusOK) {
		t.Fatalf("response แรกถูกทับ: %s", rec.Body.String())
	}
}

// ทุกตัวที่เขียน response ต้องออกมาเป็น envelope หน้าตาเดียวกัน ไม่มีตัวไหนแหกออกไป
func TestWritersShareEnvelope(t *testing.T) {
	cases := []struct {
		name    string
		write   func(echo.Context) error
		status  int
		success bool
		message string
	}{
		{
			"Success", func(c echo.Context) error { return response.Success(c, map[string]string{"k": "v"}) },
			http.StatusOK, true, "Success",
		},
		{
			"SuccessWithMessage",
			func(c echo.Context) error { return response.SuccessWithMessage(c, "Login successfully", nil) },
			http.StatusOK, true, "Login successfully",
		},
		{
			"Created", func(c echo.Context) error { return response.Created(c, "User created successfully", nil) },
			http.StatusCreated, true, "User created successfully",
		},
		{
			"BadRequest", func(c echo.Context) error { return response.BadRequest(c, "Invalid request body") },
			http.StatusBadRequest, false, "Invalid request body",
		},
		{
			"Unauthorized", func(c echo.Context) error { return response.Unauthorized(c, "Unauthorized") },
			http.StatusUnauthorized, false, "Unauthorized",
		},
		{
			"Forbidden", func(c echo.Context) error { return response.Forbidden(c, "Permission denied") },
			http.StatusForbidden, false, "Permission denied",
		},
		{
			"NotFound", func(c echo.Context) error { return response.NotFound(c, "Blog not found") },
			http.StatusNotFound, false, "Blog not found",
		},
		{
			"Conflict", func(c echo.Context) error { return response.Conflict(c, "Email already exists") },
			http.StatusConflict, false, "Email already exists",
		},
		{
			"InternalServerError", response.InternalServerError,
			http.StatusInternalServerError, false, "Internal server error",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, rec := newContext(t)

			if err := tc.write(c); err != nil {
				t.Fatalf("write: %v", err)
			}

			if rec.Code != tc.status {
				t.Fatalf("status = %d อยากได้ %d", rec.Code, tc.status)
			}

			body := decodeBody(t, rec)

			if body["status"] != float64(tc.status) {
				t.Fatalf("status ใน body = %v ไม่ตรงกับ HTTP status", body["status"])
			}
			if body["success"] != tc.success {
				t.Fatalf("success = %v", body["success"])
			}
			if body["message"] != tc.message {
				t.Fatalf("message = %v อยากได้ %q", body["message"], tc.message)
			}
			if body["request_id"] != "req-1" || body["timestamp"] == nil {
				t.Fatalf("envelope ไม่ครบ: %s", rec.Body.String())
			}
		})
	}
}

// data ที่เป็น nil ต้องหายไปเลย ไม่ใช่โผล่มาเป็น "data": null ให้ client ต้องมานั่งเช็ค
func TestNilDataIsOmitted(t *testing.T) {
	c, rec := newContext(t)

	if err := response.SuccessWithMessage(c, "Blog deleted successfully", nil); err != nil {
		t.Fatalf("write: %v", err)
	}

	if _, present := decodeBody(t, rec)["data"]; present {
		t.Fatalf("มี data null โผล่มา: %s", rec.Body.String())
	}
}

func TestSuccessWithPagination(t *testing.T) {
	c, rec := newContext(t)

	err := response.SuccessWithPagination(c, []string{"a", "b"}, response.NewPagination(2, 20, 121))
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	body := decodeBody(t, rec)

	pagination, ok := body["pagination"].(map[string]any)
	if !ok {
		t.Fatalf("ไม่มี pagination: %s", rec.Body.String())
	}
	if pagination["page"] != float64(2) || pagination["limit"] != float64(20) {
		t.Fatalf("pagination = %v", pagination)
	}
	if pagination["total"] != float64(121) || pagination["total_page"] != float64(7) {
		t.Fatalf("total/total_page = %v — 121 หาร 20 ต้องปัดขึ้นเป็น 7 หน้า", pagination)
	}
}

// HTTPError ของ echo (route ไม่มี, method ไม่ตรง) ต้องถูกดึงเข้า envelope เดียวกันทุกตัว
func TestErrorHandlerMapsEchoHTTPErrors(t *testing.T) {
	cases := []struct {
		name string
		err  *echo.HTTPError
		want int
	}{
		{"404", echo.NewHTTPError(http.StatusNotFound, "Not Found"), http.StatusNotFound},
		{"405", echo.NewHTTPError(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed},
		{"400", echo.NewHTTPError(http.StatusBadRequest, "bad"), http.StatusBadRequest},
		{"401", echo.NewHTTPError(http.StatusUnauthorized), http.StatusUnauthorized},
		{"418 ที่ไม่ได้ map ไว้ → 500", echo.NewHTTPError(http.StatusTeapot), http.StatusInternalServerError},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec, body := handle(t, tc.err)

			// 405 ถูก map ลง BadRequest ตามที่ writeEchoError ตั้งใจ
			want := tc.want
			if tc.want == http.StatusMethodNotAllowed {
				want = http.StatusBadRequest
			}

			if rec.Code != want {
				t.Fatalf("status = %d อยากได้ %d", rec.Code, want)
			}
			if body["success"] != false || body["timestamp"] == nil {
				t.Fatalf("ไม่ได้อยู่ใน envelope เดียวกัน: %s", rec.Body.String())
			}
		})
	}
}

// เขียน response ไม่ออก (client ตัดสายไปแล้ว) ต้อง log ไว้ ไม่ใช่เงียบหรือ panic
func TestErrorHandlerLogsWhenWriteFails(t *testing.T) {
	var buf bytes.Buffer
	restore := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	defer slog.SetDefault(restore)

	e := echo.New()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	c := e.NewContext(req, &brokenWriter{header: http.Header{}})

	response.ErrorHandler(apperr.ErrForbidden, c)

	if !bytes.Contains(buf.Bytes(), []byte("write error response failed")) {
		t.Fatalf("เขียน response ไม่ออกแล้วไม่ log อะไรเลย: %s", buf.String())
	}
}

type brokenWriter struct {
	header http.Header
}

func (w *brokenWriter) Header() http.Header { return w.header }

func (w *brokenWriter) Write([]byte) (int, error) {
	return 0, errors.New("client ตัดสายไปแล้ว")
}

func (w *brokenWriter) WriteHeader(int) {}

func TestTooManyRequestsWriter(t *testing.T) {
	c, rec := newContext(t)

	if err := response.TooManyRequests(c, "Too many requests, please try again later"); err != nil {
		t.Fatalf("write: %v", err)
	}

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d อยากได้ 429", rec.Code)
	}

	body := decodeBody(t, rec)
	if body["success"] != false || body["request_id"] != "req-1" {
		t.Fatalf("envelope ไม่ครบ: %s", rec.Body.String())
	}
}

// rate limiter ของ echo โยน 429 มาเป็น HTTPError — ถ้าไม่ดักจะตกไป default แล้วกลายเป็น 500
func TestErrorHandlerMaps429(t *testing.T) {
	rec, body := handle(t, echo.NewHTTPError(http.StatusTooManyRequests))

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d อยากได้ 429", rec.Code)
	}
	if body["message"] != "Too many requests, please try again later" {
		t.Fatalf("message = %v", body["message"])
	}
}

func TestErrorHandlerMapsInvalidRefresh(t *testing.T) {
	rec, body := handle(t, apperr.ErrInvalidRefresh)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d อยากได้ 401", rec.Code)
	}
	if body["message"] != "Invalid or expired refresh token" {
		t.Fatalf("message = %v", body["message"])
	}
}
