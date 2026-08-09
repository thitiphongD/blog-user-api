package middleware

import (
	"log/slog"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/thitiphongD/blog-user-api/internal/logging"
)

// RequestContext ยัด request id ที่ echomw.RequestID ออกให้ ลงใน context.Context
// service กับ repository จะได้ log แล้วมี request_id ติดไปด้วยโดยไม่ต้องรู้จัก echo
func RequestContext() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := c.Request()
			id := c.Response().Header().Get(echo.HeaderXRequestID)

			c.SetRequest(req.WithContext(logging.WithRequestID(req.Context(), id)))

			return next(c)
		}
	}
}

// Logger เขียน access log ด้วย slog — ใช้แทน echomw.Logger() เพื่อให้ log ทั้งระบบ
// เป็น handler เดียวกันหมด ไม่ใช่ JSON คนละฟอร์แมตปนกัน
func Logger() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()

			err := next(c)
			if err != nil {
				// ปล่อยให้ ErrorHandler เขียน response ก่อน status ถึงจะเป็นค่าจริง
				c.Error(err)
			}

			req := c.Request()
			res := c.Response()

			attrs := []any{
				"method", req.Method,
				"path", req.URL.Path,
				"status", res.Status,
				"latency_ms", float64(time.Since(start).Microseconds()) / 1000,
				"remote_ip", c.RealIP(),
			}
			if err != nil {
				attrs = append(attrs, "error", err.Error())
			}

			logging.FromContext(req.Context()).Log(req.Context(), level(res.Status), "request", attrs...)

			return nil
		}
	}
}

func level(status int) slog.Level {
	switch {
	case status >= 500:
		return slog.LevelError
	case status >= 400:
		return slog.LevelWarn
	default:
		return slog.LevelInfo
	}
}
