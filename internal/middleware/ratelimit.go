package middleware

import (
	"time"

	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	"golang.org/x/time/rate"
)

// RateLimitAuth จำกัดจำนวน request ต่อนาทีต่อ IP สำหรับ endpoint ที่เป็นเป้าของการเดารหัส
// (login กับ register) — ไม่ได้เอาไปครอบ refresh เพราะ token สุ่ม 256 bit เดาไม่ได้
// และ client ปกติยิง refresh บ่อยกว่ามาก
//
// เก็บสถานะในหน่วยความจำของ process — พอสำหรับตอนรัน instance เดียว
// ถ้าขยายเป็นหลาย replica เมื่อไหร่ต้องย้ายไปเก็บที่ส่วนกลาง (เช่น Redis) ไม่งั้นโควตาจะคูณตามจำนวน pod
// perMinute <= 0 = ปิด rate limit ไปเลย (ใช้ตอน dev/เทสต์) ไม่ใช่แปลว่า "ห้ามยิงเลยสักครั้ง"
// ซึ่งเป็นสิ่งที่ limiter จะทำถ้าปล่อย burst เป็น 0
func RateLimitAuth(perMinute int) echo.MiddlewareFunc {
	if perMinute <= 0 {
		return func(next echo.HandlerFunc) echo.HandlerFunc { return next }
	}

	store := echomw.NewRateLimiterMemoryStoreWithConfig(echomw.RateLimiterMemoryStoreConfig{
		Rate:      rate.Limit(float64(perMinute) / 60),
		Burst:     perMinute,
		ExpiresIn: 10 * time.Minute,
	})

	return echomw.RateLimiterWithConfig(echomw.RateLimiterConfig{
		Store: store,
		IdentifierExtractor: func(c echo.Context) (string, error) {
			return c.RealIP(), nil
		},
	})
}
