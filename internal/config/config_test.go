package config_test

import (
	"strings"
	"testing"
	"time"

	"github.com/thitiphongD/blog-user-api/internal/config"
)

// Load อ่าน .env ของโปรเจกต์ด้วยถ้ามี แต่ godotenv ไม่ override env ที่ตั้งไว้แล้ว
// t.Setenv เลยชนะเสมอ
func TestLoadDefaults(t *testing.T) {
	t.Setenv("JWT_SECRET", "secret")
	t.Setenv("APP_PORT", "")
	t.Setenv("DB_MAX_OPEN_CONNS", "")
	t.Setenv("SERVER_READ_TIMEOUT", "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if cfg.App.Port != "8080" {
		t.Fatalf("port = %s อยากได้ default 8080", cfg.App.Port)
	}
	if cfg.DB.MaxOpenConns != 25 {
		t.Fatalf("max open conns = %d อยากได้ default 25", cfg.DB.MaxOpenConns)
	}
	if cfg.Server.ReadTimeout != 10*time.Second {
		t.Fatalf("read timeout = %v อยากได้ default 10s", cfg.Server.ReadTimeout)
	}
	if cfg.JWT.Expire != 24*time.Hour {
		t.Fatalf("jwt expire = %v อยากได้ default 24h", cfg.JWT.Expire)
	}
}

// ไม่มี JWT_SECRET ต้องไม่ยอมสตาร์ท ไม่ใช่ปล่อยให้รันด้วย secret ว่าง
func TestLoadRequiresJWTSecret(t *testing.T) {
	t.Setenv("JWT_SECRET", "")

	if _, err := config.Load(); err == nil {
		t.Fatal("ยอมสตาร์ททั้งที่ไม่มี JWT_SECRET")
	}
}

func TestLoadReadsEnv(t *testing.T) {
	t.Setenv("JWT_SECRET", "secret")
	t.Setenv("APP_PORT", "9999")
	t.Setenv("DB_MAX_IDLE_CONNS", "3")
	t.Setenv("DB_CONN_MAX_LIFETIME", "90s")
	t.Setenv("JWT_EXPIRE_HOURS", "1")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if cfg.App.Port != "9999" {
		t.Fatalf("port = %s", cfg.App.Port)
	}
	if cfg.DB.MaxIdleConns != 3 {
		t.Fatalf("max idle conns = %d", cfg.DB.MaxIdleConns)
	}
	if cfg.DB.ConnMaxLifetime != 90*time.Second {
		t.Fatalf("conn max lifetime = %v", cfg.DB.ConnMaxLifetime)
	}
	if cfg.JWT.Expire != time.Hour {
		t.Fatalf("jwt expire = %v", cfg.JWT.Expire)
	}
}

// ค่าที่ parse ไม่ได้ต้องถอยไปใช้ default ไม่ใช่กลายเป็น 0 แล้วพังตอน runtime
// (pool = 0 คือ unlimited, timeout = 0 คือไม่มี timeout — สองอันนั้นอันตรายกว่าค่าผิด)
func TestLoadFallsBackOnGarbage(t *testing.T) {
	t.Setenv("JWT_SECRET", "secret")
	t.Setenv("DB_MAX_OPEN_CONNS", "เยอะๆ")
	t.Setenv("SERVER_WRITE_TIMEOUT", "นานหน่อย")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if cfg.DB.MaxOpenConns != 25 {
		t.Fatalf("max open conns = %d อยากได้ 25 (ไม่ใช่ 0 = unlimited)", cfg.DB.MaxOpenConns)
	}
	if cfg.Server.WriteTimeout != 10*time.Second {
		t.Fatalf("write timeout = %v อยากได้ 10s (ไม่ใช่ 0 = ไม่มี timeout)", cfg.Server.WriteTimeout)
	}
}

func TestDSNAndURL(t *testing.T) {
	db := config.DBConfig{
		Host: "postgres", Port: "5432", User: "u", Password: "p",
		Name: "blog", SSLMode: "disable",
	}

	dsn := db.DSN()
	if !strings.Contains(dsn, "TimeZone=UTC") {
		t.Fatalf("DSN ไม่ได้ปัก TimeZone=UTC: %s", dsn)
	}
	if !strings.Contains(dsn, "host=postgres") || !strings.Contains(dsn, "dbname=blog") {
		t.Fatalf("DSN ประกอบผิด: %s", dsn)
	}

	want := "postgres://u:p@postgres:5432/blog?sslmode=disable"
	if got := db.URL(); got != want {
		t.Fatalf("URL = %s อยากได้ %s", got, want)
	}
}
