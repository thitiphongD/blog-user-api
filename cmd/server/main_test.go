package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/thitiphongD/blog-user-api/internal/config"
)

func testConfig() *config.Config {
	return &config.Config{
		App:       config.AppConfig{Port: "8080", Env: "test"},
		JWT:       config.JWTConfig{Secret: "secret", AccessTTL: time.Minute, RefreshTTL: time.Hour},
		RateLimit: config.RateLimitConfig{AuthPerMinute: 10},
	}
}

// rate limit ที่ครอบ login/register คีย์ด้วย c.RealIP() ถ้าไม่ปัก IPExtractor เอง
// echo จะอ่าน X-Forwarded-For ก่อน RemoteAddr = สุ่ม header ใหม่ทุก request แล้วหนีโควตาได้
// (remote_ip ใน access log ก็ปลอมได้ด้วยเหตุผลเดียวกัน)
func TestRealIPIgnoresSpoofableHeaders(t *testing.T) {
	e, _ := newEcho(testConfig(), nil, nil)

	cases := []struct {
		name   string
		header string
		value  string
	}{
		{"X-Forwarded-For", echo.HeaderXForwardedFor, "203.0.113.9"},
		{"X-Real-IP", echo.HeaderXRealIP, "203.0.113.9"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/auth/login", nil)
			req.RemoteAddr = "10.1.2.3:54321"
			req.Header.Set(tc.header, tc.value)

			got := e.NewContext(req, httptest.NewRecorder()).RealIP()
			if got != "10.1.2.3" {
				t.Fatalf("RealIP เชื่อ %s: ได้ %q อยากได้ 10.1.2.3", tc.name, got)
			}
		})
	}
}
