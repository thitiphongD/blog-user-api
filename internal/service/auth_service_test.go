package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/thitiphongD/blog-user-api/internal/apperr"
	"github.com/thitiphongD/blog-user-api/internal/auth"
	"github.com/thitiphongD/blog-user-api/internal/dto/request"
	"github.com/thitiphongD/blog-user-api/internal/model"
)

func testJWT() *auth.JWT {
	return auth.NewJWT("test-secret", time.Hour)
}

func TestRegisterHashesPassword(t *testing.T) {
	var saved *model.User

	repo := &mockUserRepo{
		create: func(_ context.Context, u *model.User) error {
			saved = u
			return nil
		},
	}

	user, err := NewAuthService(repo, testJWT()).Register(context.Background(), request.RegisterRequest{
		Name:     "Daew",
		Email:    "daew@example.com",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	if saved == nil {
		t.Fatal("repository.Create ไม่ถูกเรียก")
	}
	if user.Password == "password123" {
		t.Fatal("password ถูกเก็บเป็น plain text")
	}
	if !auth.ComparePassword(user.Password, "password123") {
		t.Fatal("hash ที่เก็บ compare กับ password เดิมไม่ผ่าน")
	}
}

func TestRegisterPropagatesEmailTaken(t *testing.T) {
	repo := &mockUserRepo{
		create: func(context.Context, *model.User) error { return apperr.ErrEmailTaken },
	}

	_, err := NewAuthService(repo, testJWT()).Register(context.Background(), request.RegisterRequest{
		Name: "Daew", Email: "daew@example.com", Password: "password123",
	})

	if !errors.Is(err, apperr.ErrEmailTaken) {
		t.Fatalf("อยากได้ ErrEmailTaken ได้ %v", err)
	}
}

func TestLoginSuccess(t *testing.T) {
	hashed, err := auth.HashPassword("password123")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}

	id := uuid.New()
	repo := &mockUserRepo{
		findByEmail: func(context.Context, string) (*model.User, error) {
			return &model.User{ID: id, Email: "daew@example.com", Password: hashed}, nil
		},
	}

	jwt := testJWT()

	result, err := NewAuthService(repo, jwt).Login(context.Background(), request.LoginRequest{
		Email: "daew@example.com", Password: "password123",
	})
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	if result.ExpiredAt.Before(time.Now()) {
		t.Fatal("expired_at เป็นอดีต")
	}

	got, err := jwt.Verify(result.Token)
	if err != nil {
		t.Fatalf("verify token ที่เพิ่งออกไม่ผ่าน: %v", err)
	}
	if got != id {
		t.Fatalf("token ถือ user id %v อยากได้ %v", got, id)
	}
}

// email ที่ไม่มีในระบบต้องได้ ErrInvalidCredential ไม่ใช่ ErrNotFound
// ไม่งั้น client เดาได้ว่า email ไหนสมัครไว้
func TestLoginUnknownEmailHidesNotFound(t *testing.T) {
	repo := &mockUserRepo{
		findByEmail: func(context.Context, string) (*model.User, error) {
			return nil, apperr.NotFound("User")
		},
	}

	_, err := NewAuthService(repo, testJWT()).Login(context.Background(), request.LoginRequest{
		Email: "nobody@example.com", Password: "password123",
	})

	if errors.Is(err, apperr.ErrNotFound) {
		t.Fatal("หลุด ErrNotFound ออกไป = บอกใบ้ว่า email นี้ไม่มีในระบบ")
	}
	if !errors.Is(err, apperr.ErrInvalidCredential) {
		t.Fatalf("อยากได้ ErrInvalidCredential ได้ %v", err)
	}
}

func TestLoginWrongPassword(t *testing.T) {
	hashed, err := auth.HashPassword("password123")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}

	repo := &mockUserRepo{
		findByEmail: func(context.Context, string) (*model.User, error) {
			return &model.User{ID: uuid.New(), Password: hashed}, nil
		},
	}

	_, err = NewAuthService(repo, testJWT()).Login(context.Background(), request.LoginRequest{
		Email: "daew@example.com", Password: "wrong-password",
	})

	if !errors.Is(err, apperr.ErrInvalidCredential) {
		t.Fatalf("อยากได้ ErrInvalidCredential ได้ %v", err)
	}
}

func TestGetMe(t *testing.T) {
	id := uuid.New()
	repo := &mockUserRepo{
		findByID: func(_ context.Context, got uuid.UUID) (*model.User, error) {
			return &model.User{ID: got, Email: "daew@example.com"}, nil
		},
	}

	user, err := NewAuthService(repo, testJWT()).GetMe(context.Background(), id)
	if err != nil {
		t.Fatalf("get me: %v", err)
	}
	if user.ID != id {
		t.Fatalf("ได้คนละคน: %v", user.ID)
	}
}

func TestGetMePropagatesNotFound(t *testing.T) {
	repo := &mockUserRepo{
		findByID: func(context.Context, uuid.UUID) (*model.User, error) {
			return nil, apperr.NotFound("User")
		},
	}

	_, err := NewAuthService(repo, testJWT()).GetMe(context.Background(), uuid.New())
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("อยากได้ ErrNotFound ได้ %v", err)
	}
}

// รหัสยาวเกิน 72 bytes ทำให้ bcrypt คืน error — service ต้องไม่ไปเรียก Create ต่อ
// (ปกติ validator กันไว้ก่อนแล้ว แต่ service ต้องไม่พังถ้ามีใครเรียกตรงๆ)
func TestRegisterStopsWhenHashFails(t *testing.T) {
	repo := &mockUserRepo{
		create: func(context.Context, *model.User) error {
			t.Error("hash พังแล้วยังไปเรียก Create ต่อ")

			return nil
		},
	}

	_, err := NewAuthService(repo, testJWT()).Register(context.Background(), request.RegisterRequest{
		Name: "Daew", Email: "daew@example.com", Password: strings.Repeat("a", 73),
	})
	if err == nil {
		t.Fatal("ควร error แต่ผ่านไปได้")
	}
}

// DB พังต้องเด้งเป็น error จริงออกไป (= 500) ไม่ใช่กลายเป็น ErrInvalidCredential (= 401)
// ไม่งั้น DB ล่มทีไร ผู้ใช้จะเห็นว่า "รหัสผิด" แล้วไปนั่งรีเซ็ตรหัสกันทั้งบ้าน
func TestLoginPropagatesDatabaseError(t *testing.T) {
	dbDown := errors.New("connection refused")

	repo := &mockUserRepo{
		findByEmail: func(context.Context, string) (*model.User, error) { return nil, dbDown },
	}

	_, err := NewAuthService(repo, testJWT()).Login(context.Background(), request.LoginRequest{
		Email: "daew@example.com", Password: "password123",
	})

	if errors.Is(err, apperr.ErrInvalidCredential) {
		t.Fatal("DB ล่มแล้วบอกผู้ใช้ว่ารหัสผิด")
	}
	if !errors.Is(err, dbDown) {
		t.Fatalf("อยากได้ error เดิม ได้ %v", err)
	}
}
