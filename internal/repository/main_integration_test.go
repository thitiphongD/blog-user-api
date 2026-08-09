//go:build integration

// เทสต์ชุดนี้ต้องมี postgres จริง เพราะของที่อยากพิสูจน์คือพฤติกรรมของ SQL/GORM เอง
// (soft delete, unique violation, ILIKE, ORDER BY, transaction) ซึ่ง mock พิสูจน์ให้ไม่ได้
//
//	make test-integration
package repository_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"gorm.io/gorm"

	"github.com/thitiphongD/blog-user-api/internal/config"
	"github.com/thitiphongD/blog-user-api/internal/database"
	"github.com/thitiphongD/blog-user-api/internal/migrate"
)

var testDB *gorm.DB

func TestMain(m *testing.M) {
	cfg := config.DBConfig{
		Host:            env("TEST_DB_HOST", "localhost"),
		Port:            env("TEST_DB_PORT", "5432"),
		User:            env("TEST_DB_USER", "postgres"),
		Password:        env("TEST_DB_PASSWORD", "postgres"),
		Name:            env("TEST_DB_NAME", "blog_user_api_test"),
		SSLMode:         env("TEST_DB_SSLMODE", "disable"),
		MaxOpenConns:    5,
		MaxIdleConns:    2,
		ConnMaxLifetime: 0,
	}

	// ใช้ database.Connect ตัวจริง ไม่ใช่ gorm.Open เอง เพราะ TranslateError: true
	// อยู่ในนั้น ถ้าใครลบทิ้ง เทสต์ email ซ้ำต้องแดง
	db, err := database.Connect(context.Background(), cfg, "test")
	if err != nil {
		fmt.Fprintf(os.Stderr, "ต่อ postgres ไม่ได้: %v\nสั่ง make test-integration หรือยกเอง\n", err)
		os.Exit(1)
	}

	// migration ตัวจริง — schema ที่เทสต์วิ่งบนคืออันเดียวกับที่ deploy
	if err := migrate.Up(cfg.URL()); err != nil {
		fmt.Fprintf(os.Stderr, "รัน migration ไม่ผ่าน: %v\n", err)
		os.Exit(1)
	}

	testDB = db

	os.Exit(m.Run())
}

// reset ล้างข้อมูลก่อนทุกเทสต์ เพื่อไม่ให้ลำดับการรันมีผลต่อผลลัพธ์
func reset(t *testing.T) {
	t.Helper()

	if err := testDB.Exec("TRUNCATE blogs, users RESTART IDENTITY CASCADE").Error; err != nil {
		t.Fatalf("ล้างตาราง: %v", err)
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	return fallback
}
