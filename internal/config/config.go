package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	App       AppConfig
	DB        DBConfig
	Server    ServerConfig
	JWT       JWTConfig
	RateLimit RateLimitConfig
	CORS      CORSConfig
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
	Secret     string
	AccessTTL  time.Duration
	RefreshTTL time.Duration
	// RefreshPruneInterval คือทุกกี่นานจะกวาด refresh token ที่หมดอายุทิ้ง (<=0 คือไม่กวาด)
	RefreshPruneInterval time.Duration
}

type CORSConfig struct {
	// AllowedOrigins มี "*" ตัวเดียวคือเปิดให้ทุกเว็บ ใช้ได้ตอน dev แต่ก่อน deploy ควรระบุให้ชัด
	AllowedOrigins []string
}

type RateLimitConfig struct {
	// จำนวน request ต่อนาทีต่อ IP ที่ยอมให้ยิงเข้า endpoint ฝั่ง auth
	AuthPerMinute int
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
			// access token สั้นเพราะเพิกถอนไม่ได้ — ของที่อายุยาวคือ refresh token
			// ซึ่งเพิกถอนได้เพราะอยู่ใน DB
			AccessTTL:  envDuration("JWT_ACCESS_TTL", 15*time.Minute),
			RefreshTTL: envDuration("JWT_REFRESH_TTL", 7*24*time.Hour),
			// refresh_tokens โตทุกครั้งที่ login/refresh ไม่มีใครลบให้ ต้องกวาดเอง
			RefreshPruneInterval: envDuration("REFRESH_PRUNE_INTERVAL", time.Hour),
		},
		RateLimit: RateLimitConfig{
			AuthPerMinute: envInt("RATE_LIMIT_AUTH_PER_MINUTE", 10),
		},
		CORS: CORSConfig{
			AllowedOrigins: envList("CORS_ALLOWED_ORIGINS", "*"),
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

// envList แยกด้วย comma แล้วตัดช่องว่างกับตัวว่างทิ้ง — "a, ,b" ได้ ["a" "b"]
func envList(key, fallback string) []string {
	raw := env(key, fallback)

	out := make([]string, 0, strings.Count(raw, ",")+1)
	for _, part := range strings.Split(raw, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}

	return out
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
