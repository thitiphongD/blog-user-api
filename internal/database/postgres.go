package database

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/thitiphongD/blog-user-api/internal/config"
)

const (
	connectRetries = 10
	connectBackoff = 2 * time.Second
)

// Connect ต่อ postgres พร้อม retry — compose healthcheck กันไว้ชั้นนึงแล้ว
// แต่ postgres ผ่าน pg_isready ไม่ได้แปลว่าพร้อมรับ connection ทันทีเสมอไป
func Connect(ctx context.Context, cfg config.DBConfig, env string) (*gorm.DB, error) {
	gormCfg := &gorm.Config{
		Logger: gormLogger(env),
		// แปลง error ของ driver เป็น error กลางของ GORM (เช่น ErrDuplicatedKey)
		// repository จะได้ไม่ต้องไปแกะ error code ของ postgres เอง
		TranslateError: true,
	}

	var lastErr error
	for attempt := 1; attempt <= connectRetries; attempt++ {
		db, err := gorm.Open(postgres.Open(cfg.DSN()), gormCfg)
		if err == nil {
			if err = tune(db, cfg); err != nil {
				return nil, err
			}
			return db, nil
		}

		lastErr = err
		slog.Warn("connect postgres failed, retrying",
			"attempt", attempt, "max", connectRetries, "error", err)

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(connectBackoff):
		}
	}

	return nil, fmt.Errorf("connect postgres after %d attempts: %w", connectRetries, lastErr)
}

// tune ตั้ง connection pool — default ของ database/sql คือ open conns ไม่จำกัด
func tune(db *gorm.DB, cfg config.DBConfig) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get sql.DB: %w", err)
	}

	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	return nil
}

func gormLogger(env string) logger.Interface {
	dev := env == "development"

	level := logger.Warn
	if dev {
		level = logger.Info
	}

	return logger.New(log.New(os.Stdout, "", log.LstdFlags), logger.Config{
		SlowThreshold: 200 * time.Millisecond,
		LogLevel:      level,
		// ไม่งั้นทุก 404 จะโผล่เป็น error ใน log ทั้งที่เป็นเคสปกติ
		IgnoreRecordNotFoundError: true,
		Colorful:                  dev,
	})
}
