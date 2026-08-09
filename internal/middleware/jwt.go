// Package middleware เก็บ middleware ของ echo — logic จริงอยู่ใน package อื่น ตัวนี้แค่ต่อสาย
package middleware

import (
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/thitiphongD/blog-user-api/internal/apperr"
	"github.com/thitiphongD/blog-user-api/internal/auth"
)

const userIDKey = "user_id"

const bearerPrefix = "Bearer "

// JWT ตรวจ token แล้วยัด user id ลง context ให้ handler หยิบผ่าน UserID()
func JWT(j *auth.JWT) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			header := c.Request().Header.Get(echo.HeaderAuthorization)
			if !strings.HasPrefix(header, bearerPrefix) {
				return apperr.ErrUnauthorized
			}

			userID, err := j.Verify(strings.TrimPrefix(header, bearerPrefix))
			if err != nil {
				return apperr.ErrUnauthorized
			}

			c.Set(userIDKey, userID)

			return next(c)
		}
	}
}

// UserID หยิบ user id ที่ middleware JWT ใส่ไว้ — เรียกได้เฉพาะ route ที่ผ่าน JWT มาแล้ว
func UserID(c echo.Context) (uuid.UUID, error) {
	id, ok := c.Get(userIDKey).(uuid.UUID)
	if !ok {
		return uuid.Nil, apperr.ErrUnauthorized
	}

	return id, nil
}
