package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/thitiphongD/blog-user-api/internal/apperr"
	"github.com/thitiphongD/blog-user-api/internal/auth"
	"github.com/thitiphongD/blog-user-api/internal/model"
)

func storedToken(userID uuid.UUID, raw string, expiresAt time.Time) *model.RefreshToken {
	return &model.RefreshToken{
		ID:        uuid.New(),
		UserID:    userID,
		TokenHash: auth.HashRefreshToken(raw),
		ExpiresAt: expiresAt,
	}
}

// login ต้องได้ทั้ง access token และ refresh token และ DB ต้องเก็บแค่ hash ไม่ใช่ token ดิบ
func TestLoginIssuesRefreshTokenStoredAsHash(t *testing.T) {
	hashed, err := auth.HashPassword("password123")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}

	var saved *model.RefreshToken

	users := &mockUserRepo{
		findByEmail: func(context.Context, string) (*model.User, error) {
			return &model.User{ID: uuid.New(), Password: hashed}, nil
		},
	}
	refresh := &mockRefreshRepo{
		create: func(_ context.Context, token *model.RefreshToken) error {
			saved = token

			return nil
		},
	}

	result, err := newAuthService(users, refresh).Login(context.Background(), loginReq("password123"))
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	if result.RefreshToken == "" {
		t.Fatal("ไม่ได้ refresh token")
	}
	if saved == nil {
		t.Fatal("ไม่ได้เก็บ refresh token ลง DB")
	}
	if saved.TokenHash == result.RefreshToken {
		t.Fatal("เก็บ token ดิบลง DB — DB หลุดแล้วเอาไป login ต่อได้เลย")
	}
	if saved.TokenHash != auth.HashRefreshToken(result.RefreshToken) {
		t.Fatal("hash ที่เก็บไม่ตรงกับ token ที่ออกให้")
	}
	if !result.RefreshExpiredAt.After(result.ExpiredAt) {
		t.Fatal("refresh token ควรอายุยาวกว่า access token")
	}
}

// refresh สำเร็จต้องหมุน — ตัวเก่าถูกเพิกถอน แล้วออกใบใหม่ที่ไม่ซ้ำของเดิม
func TestRefreshRotatesToken(t *testing.T) {
	userID := uuid.New()
	raw := "old-refresh-token"
	stored := storedToken(userID, raw, time.Now().Add(time.Hour))

	var revoked uuid.UUID
	tx := &fakeTx{}

	users := &mockUserRepo{
		findByID: func(context.Context, uuid.UUID) (*model.User, error) {
			return &model.User{ID: userID}, nil
		},
	}
	refresh := &mockRefreshRepo{
		findByHash: func(context.Context, string) (*model.RefreshToken, error) { return stored, nil },
		revoke: func(_ context.Context, id uuid.UUID, _ time.Time) error {
			revoked = id

			return nil
		},
	}

	svc := NewAuthService(users, refresh, tx, testJWT(), 7*24*time.Hour)

	result, err := svc.Refresh(context.Background(), raw)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}

	if !tx.called {
		t.Fatal("เพิกถอนตัวเก่า + ออกตัวใหม่ ไม่ได้อยู่ใน transaction เดียวกัน")
	}
	if revoked != stored.ID {
		t.Fatal("ไม่ได้เพิกถอน token ตัวเก่า — ใช้ซ้ำได้ไม่จำกัด")
	}
	if result.RefreshToken == raw {
		t.Fatal("คืน token ตัวเดิมกลับมา ไม่ได้หมุน")
	}
}

// เอา token ที่ถูกใช้ไปแล้วมาใช้ซ้ำ = มีคนถืออยู่สองมือ ต้องตัดทุก session ของ user คนนั้น
func TestRefreshDetectsReuseAndRevokesEverything(t *testing.T) {
	userID := uuid.New()
	raw := "already-used"
	revokedAt := time.Now().Add(-time.Minute)

	stored := storedToken(userID, raw, time.Now().Add(time.Hour))
	stored.RevokedAt = &revokedAt

	killedAll := false

	refresh := &mockRefreshRepo{
		findByHash: func(context.Context, string) (*model.RefreshToken, error) { return stored, nil },
		revokeAllForUser: func(_ context.Context, got uuid.UUID, _ time.Time) error {
			if got != userID {
				t.Errorf("เพิกถอนของ user ผิดคน: %v", got)
			}
			killedAll = true

			return nil
		},
	}

	_, err := newAuthService(&mockUserRepo{}, refresh).Refresh(context.Background(), raw)

	if !errors.Is(err, apperr.ErrInvalidRefresh) {
		t.Fatalf("อยากได้ ErrInvalidRefresh ได้ %v", err)
	}
	if !killedAll {
		t.Fatal("จับ reuse ได้แล้วแต่ไม่ตัด session อื่นทิ้ง")
	}
}

func TestRefreshRejects(t *testing.T) {
	userID := uuid.New()

	cases := []struct {
		name    string
		refresh *mockRefreshRepo
	}{
		{
			"ไม่เคยมี token นี้",
			&mockRefreshRepo{
				findByHash: func(context.Context, string) (*model.RefreshToken, error) {
					return nil, apperr.NotFound("Refresh token")
				},
			},
		},
		{
			"หมดอายุแล้ว",
			&mockRefreshRepo{
				findByHash: func(context.Context, string) (*model.RefreshToken, error) {
					return storedToken(userID, "expired", time.Now().Add(-time.Hour)), nil
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := newAuthService(&mockUserRepo{}, tc.refresh).Refresh(context.Background(), "whatever")
			if !errors.Is(err, apperr.ErrInvalidRefresh) {
				t.Fatalf("อยากได้ ErrInvalidRefresh ได้ %v", err)
			}
		})
	}
}

// user ถูกลบไปแล้วแต่ refresh token ยังค้าง ต้องใช้ไม่ได้ ไม่ใช่ 500
func TestRefreshRejectsWhenUserGone(t *testing.T) {
	stored := storedToken(uuid.New(), "raw", time.Now().Add(time.Hour))

	users := &mockUserRepo{
		findByID: func(context.Context, uuid.UUID) (*model.User, error) {
			return nil, apperr.NotFound("User")
		},
	}
	refresh := &mockRefreshRepo{
		findByHash: func(context.Context, string) (*model.RefreshToken, error) { return stored, nil },
	}

	_, err := newAuthService(users, refresh).Refresh(context.Background(), "raw")
	if !errors.Is(err, apperr.ErrInvalidRefresh) {
		t.Fatalf("อยากได้ ErrInvalidRefresh ได้ %v", err)
	}
}

func TestLogoutRevokesOnlyThatSession(t *testing.T) {
	stored := storedToken(uuid.New(), "raw", time.Now().Add(time.Hour))

	var revoked uuid.UUID
	refresh := &mockRefreshRepo{
		findByHash: func(context.Context, string) (*model.RefreshToken, error) { return stored, nil },
		revoke: func(_ context.Context, id uuid.UUID, _ time.Time) error {
			revoked = id

			return nil
		},
		// ไม่เซ็ต revokeAllForUser → ถ้าถูกเรียกจะ panic
	}

	if err := newAuthService(&mockUserRepo{}, refresh).Logout(context.Background(), "raw"); err != nil {
		t.Fatalf("logout: %v", err)
	}

	if revoked != stored.ID {
		t.Fatal("ไม่ได้เพิกถอน session ที่ยื่นมา")
	}
}

func TestLogoutWithUnknownToken(t *testing.T) {
	refresh := &mockRefreshRepo{
		findByHash: func(context.Context, string) (*model.RefreshToken, error) {
			return nil, apperr.NotFound("Refresh token")
		},
	}

	err := newAuthService(&mockUserRepo{}, refresh).Logout(context.Background(), "ไม่มีอยู่จริง")
	if !errors.Is(err, apperr.ErrInvalidRefresh) {
		t.Fatalf("อยากได้ ErrInvalidRefresh ได้ %v", err)
	}
}

// DB พังต้องเด้งเป็น error จริง (500) ไม่ใช่กลายเป็น "token ไม่ถูกต้อง" (401)
func TestRefreshPropagatesDatabaseError(t *testing.T) {
	dbDown := errors.New("connection refused")

	refresh := &mockRefreshRepo{
		findByHash: func(context.Context, string) (*model.RefreshToken, error) { return nil, dbDown },
	}

	_, err := newAuthService(&mockUserRepo{}, refresh).Refresh(context.Background(), "raw")

	if errors.Is(err, apperr.ErrInvalidRefresh) {
		t.Fatal("DB ล่มแล้วบอกว่า token ไม่ถูกต้อง")
	}
	if !errors.Is(err, dbDown) {
		t.Fatalf("อยากได้ error เดิม ได้ %v", err)
	}
}

func TestLogoutPropagatesDatabaseError(t *testing.T) {
	dbDown := errors.New("connection refused")

	refresh := &mockRefreshRepo{
		findByHash: func(context.Context, string) (*model.RefreshToken, error) { return nil, dbDown },
	}

	err := newAuthService(&mockUserRepo{}, refresh).Logout(context.Background(), "raw")
	if !errors.Is(err, dbDown) {
		t.Fatalf("อยากได้ error เดิม ได้ %v", err)
	}
}

// เขียน refresh token ลง DB ไม่ได้ ต้องไม่คืน token ที่ระบบจำไม่ได้ออกไปให้ client
func TestLoginFailsWhenRefreshTokenCannotBeStored(t *testing.T) {
	hashed, err := auth.HashPassword("password123")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}

	users := &mockUserRepo{
		findByEmail: func(context.Context, string) (*model.User, error) {
			return &model.User{ID: uuid.New(), Password: hashed}, nil
		},
	}
	refresh := &mockRefreshRepo{
		create: func(context.Context, *model.RefreshToken) error { return errRepo },
	}

	result, err := newAuthService(users, refresh).Login(context.Background(), loginReq("password123"))
	if !errors.Is(err, errRepo) {
		t.Fatalf("อยากได้ errRepo ได้ %v", err)
	}
	if result != nil {
		t.Fatal("เก็บ token ไม่ได้แต่ยังคืน session ออกไป")
	}
}

// เพิกถอนตัวเก่าไม่สำเร็จ ต้องไม่ออกใบใหม่ให้ ไม่งั้นใบเก่าจะยังใช้ได้คู่กับใบใหม่
func TestRefreshFailsWhenRevokeFails(t *testing.T) {
	userID := uuid.New()
	stored := storedToken(userID, "raw", time.Now().Add(time.Hour))

	users := &mockUserRepo{
		findByID: func(context.Context, uuid.UUID) (*model.User, error) {
			return &model.User{ID: userID}, nil
		},
	}
	refresh := &mockRefreshRepo{
		findByHash: func(context.Context, string) (*model.RefreshToken, error) { return stored, nil },
		revoke:     func(context.Context, uuid.UUID, time.Time) error { return errRepo },
		create: func(context.Context, *model.RefreshToken) error {
			t.Error("เพิกถอนตัวเก่าไม่สำเร็จแต่ยังออกใบใหม่")

			return nil
		},
	}

	if _, err := newAuthService(users, refresh).Refresh(context.Background(), "raw"); !errors.Is(err, errRepo) {
		t.Fatalf("อยากได้ errRepo ได้ %v", err)
	}
}

// จับ reuse ได้แต่ตัด session ไม่สำเร็จ ต้องเด้ง error จริงออกไป ไม่ใช่กลบเป็น 401
func TestRefreshReusePropagatesRevokeAllError(t *testing.T) {
	revokedAt := time.Now().Add(-time.Minute)
	stored := storedToken(uuid.New(), "raw", time.Now().Add(time.Hour))
	stored.RevokedAt = &revokedAt

	refresh := &mockRefreshRepo{
		findByHash:       func(context.Context, string) (*model.RefreshToken, error) { return stored, nil },
		revokeAllForUser: func(context.Context, uuid.UUID, time.Time) error { return errRepo },
	}

	if _, err := newAuthService(&mockUserRepo{}, refresh).Refresh(context.Background(), "raw"); !errors.Is(err, errRepo) {
		t.Fatalf("อยากได้ errRepo ได้ %v", err)
	}
}
