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

func handle(t *testing.T, err error) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()

	e := echo.New()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Response().Header().Set(echo.HeaderXRequestID, "req-1")

	response.ErrorHandler(err, c)

	var body map[string]any
	if rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("อ่าน response ไม่ออก: %v (%s)", err, rec.Body.String())
		}
	}

	return rec, body
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
