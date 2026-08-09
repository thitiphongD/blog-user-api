package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	App    AppConfig
	DB     DBConfig
	Server ServerConfig
	JWT    JWTConfig
}

type AppConfig struct {
	Port string
	Env  string
}

type DBConfig struct {
	Host            string
	Port            string
	User            string
	Password        string
	Name            string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

type ServerConfig struct {
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ReadHeaderTimeout time.Duration
}

type JWTConfig struct {
	Secret string
	Expire time.Duration
}

// Load อ่าน .env ถ้ามี (ไม่มีก็ไม่เป็นไร บน docker ใช้ env จริง) แล้วประกอบเป็น Config
func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		App: AppConfig{
			Port: env("APP_PORT", "8080"),
			Env:  env("APP_ENV", "development"),
		},
		DB: DBConfig{
			Host:            env("DB_HOST", "localhost"),
			Port:            env("DB_PORT", "5432"),
			User:            env("DB_USER", "postgres"),
			Password:        env("DB_PASSWORD", "postgres"),
			Name:            env("DB_NAME", "blog_user_api"),
			SSLMode:         env("DB_SSLMODE", "disable"),
			MaxOpenConns:    envInt("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns:    envInt("DB_MAX_IDLE_CONNS", 10),
			ConnMaxLifetime: envDuration("DB_CONN_MAX_LIFETIME", 5*time.Minute),
		},
		Server: ServerConfig{
			ReadTimeout:       envDuration("SERVER_READ_TIMEOUT", 10*time.Second),
			WriteTimeout:      envDuration("SERVER_WRITE_TIMEOUT", 10*time.Second),
			IdleTimeout:       envDuration("SERVER_IDLE_TIMEOUT", 60*time.Second),
			ReadHeaderTimeout: envDuration("SERVER_READ_HEADER_TIMEOUT", 5*time.Second),
		},
		JWT: JWTConfig{
			Secret: os.Getenv("JWT_SECRET"),
			Expire: time.Duration(envInt("JWT_EXPIRE_HOURS", 24)) * time.Hour,
		},
	}

	if cfg.JWT.Secret == "" {
		return nil, errors.New("JWT_SECRET is required")
	}

	return cfg, nil
}

// DSN ปัก TimeZone=UTC เพื่อให้ฝั่ง SQL (now(), date_trunc) เป็น UTC ไม่แกว่งตาม host
// ส่วนเวลาที่ออก response บังคับ .UTC() ที่ชั้น DTO อีกที เพราะ pgx คืน time.Time
// เป็น Local เสมอไม่ว่า session timezone จะเป็นอะไร
func (c DBConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s TimeZone=UTC",
		c.Host, c.Port, c.User, c.Password, c.Name, c.SSLMode,
	)
}

func (c DBConfig) URL() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		c.User, c.Password, c.Host, c.Port, c.Name, c.SSLMode,
	)
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	v, err := strconv.Atoi(os.Getenv(key))
	if err != nil {
		return fallback
	}
	return v
}

func envDuration(key string, fallback time.Duration) time.Duration {
	v, err := time.ParseDuration(os.Getenv(key))
	if err != nil {
		return fallback
	}
	return v
}
