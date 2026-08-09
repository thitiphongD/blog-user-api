// Package migrate รัน migration ที่ embed มากับ binary
//
// ไฟล์ SQL ต้องอยู่ใต้ package นี้ เพราะ //go:embed ห้ามมี ".." และ embed ไฟล์นอก
// directory ของ package ตัวเองไม่ได้
package migrate

import (
	"embed"
	"errors"
	"fmt"
	"log/slog"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres" // ลงทะเบียน driver ให้ scheme postgres://
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Up รัน migration ที่ยังไม่ได้รันทั้งหมด ไม่มีอะไรให้รันก็ไม่ถือว่า error
func Up(dbURL string) error {
	src, err := iofs.New(migrations, "migrations")
	if err != nil {
		return fmt.Errorf("load migrations: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", src, dbURL)
	if err != nil {
		return fmt.Errorf("init migrate: %w", err)
	}
	defer func() {
		if srcErr, dbErr := m.Close(); srcErr != nil || dbErr != nil {
			slog.Warn("close migrate", "source", srcErr, "database", dbErr)
		}
	}()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("run migration: %w", err)
	}

	return nil
}
