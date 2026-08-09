package response

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/thitiphongD/blog-user-api/internal/apperr"
	"github.com/thitiphongD/blog-user-api/internal/validator"
)

// ErrorHandler แปลง error เป็น HTTP ที่เดียวทั้งระบบ — handler แค่ return err ขึ้นมา
// ไม่ต้องไล่ map status เอง
func ErrorHandler(err error, c echo.Context) {
	if c.Response().Committed {
		return
	}

	if writeErr := writeMapped(err, c); writeErr != nil {
		slog.Error("write error response failed",
			"error", writeErr,
			"request_id", c.Response().Header().Get(echo.HeaderXRequestID),
		)
	}
}

func writeMapped(err error, c echo.Context) error {
	var validationErr *validator.Error
	if errors.As(err, &validationErr) {
		return Validation(c, validationErr.Fields)
	}

	switch {
	case errors.Is(err, apperr.ErrNotFound):
		return NotFound(c, err.Error())
	case errors.Is(err, apperr.ErrEmailTaken):
		return Conflict(c, "Email already exists")
	case errors.Is(err, apperr.ErrInvalidCredential):
		return Unauthorized(c, "Invalid email or password")
	case errors.Is(err, apperr.ErrUnauthorized):
		return Unauthorized(c, "Unauthorized")
	case errors.Is(err, apperr.ErrInvalidRefresh):
		return Unauthorized(c, "Invalid or expired refresh token")
	case errors.Is(err, apperr.ErrForbidden):
		return Forbidden(c, "Permission denied")
	}

	var httpErr *echo.HTTPError
	if errors.As(err, &httpErr) {
		return writeEchoError(httpErr, c)
	}

	// เหลือจากนี้คือของที่ไม่ได้ตั้งใจ — log ไว้ฝั่ง server แล้วคืน 500 ลอยๆ
	// ห้ามคาย error จริงออกไป ตามด้วย request_id ใน response เอา
	slog.Error("unhandled error",
		"error", err,
		"path", c.Request().URL.Path,
		"method", c.Request().Method,
		"request_id", c.Response().Header().Get(echo.HeaderXRequestID),
	)

	return InternalServerError(c)
}

func writeEchoError(httpErr *echo.HTTPError, c echo.Context) error {
	message := http.StatusText(httpErr.Code)
	if msg, ok := httpErr.Message.(string); ok && msg != "" {
		message = msg
	}

	switch httpErr.Code {
	case http.StatusNotFound:
		return NotFound(c, message)
	case http.StatusMethodNotAllowed, http.StatusBadRequest:
		return BadRequest(c, message)
	case http.StatusUnauthorized:
		return Unauthorized(c, message)
	case http.StatusTooManyRequests:
		// echo คืน 429 มาจาก rate limiter — ถ้าไม่ดักตรงนี้จะตกไป default แล้วกลายเป็น 500
		return TooManyRequests(c, "Too many requests, please try again later")
	default:
		return InternalServerError(c)
	}
}
