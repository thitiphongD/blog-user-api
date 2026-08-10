//go:build integration

package repository_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/thitiphongD/blog-user-api/internal/apperr"
	"github.com/thitiphongD/blog-user-api/internal/model"
	"github.com/thitiphongD/blog-user-api/internal/repository"
)

func newRefreshToken(t *testing.T, userID uuid.UUID, hash string) *model.RefreshToken {
	t.Helper()

	token := &model.RefreshToken{
		UserID:    userID,
		TokenHash: hash,
		ExpiresAt: time.Now().Add(time.Hour),
	}

	if err := repository.NewRefreshTokenRepository(testDB).Create(t.Context(), token); err != nil {
		t.Fatalf("create refresh token: %v", err)
	}

	return token
}

func TestRefreshTokenRoundTrip(t *testing.T) {
	reset(t)
	user := newUser(t, "daew@example.com")
	created := newRefreshToken(t, user.ID, "hash-1")

	repo := repository.NewRefreshTokenRepository(testDB)

	found, err := repo.FindByHash(t.Context(), "hash-1")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if found.ID != created.ID || found.UserID != user.ID {
		t.Fatalf("ได้คนละแถว: %+v", found)
	}
	if found.RevokedAt != nil {
		t.Fatal("เพิ่งสร้างแต่ถูกเพิกถอนแล้ว")
	}

	if _, err := repo.FindByHash(t.Context(), "ไม่มีอยู่จริง"); !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("อยากได้ ErrNotFound ได้ %v", err)
	}
}

// hash ซ้ำต้องเข้าไม่ได้ — unique constraint คือกันชนสุดท้ายกัน token ชนกัน
func TestRefreshTokenHashIsUnique(t *testing.T) {
	reset(t)
	user := newUser(t, "daew@example.com")
	newRefreshToken(t, user.ID, "hash-1")

	err := repository.NewRefreshTokenRepository(testDB).Create(t.Context(), &model.RefreshToken{
		UserID: user.ID, TokenHash: "hash-1", ExpiresAt: time.Now().Add(time.Hour),
	})
	if err == nil {
		t.Fatal("สร้าง token ที่ hash ซ้ำได้")
	}
}

// FindByHash ต้องคืนแถวที่ถูกเพิกถอนแล้วด้วย ไม่งั้น service แยกไม่ออกระหว่าง
// "ไม่เคยมี token นี้" กับ "เคยมีแต่ถูกใช้ไปแล้ว" ซึ่งอย่างหลังคือสัญญาณว่า token รั่ว
func TestRevokedTokenIsStillFindable(t *testing.T) {
	reset(t)
	user := newUser(t, "daew@example.com")
	token := newRefreshToken(t, user.ID, "hash-1")

	repo := repository.NewRefreshTokenRepository(testDB)
	now := time.Now()

	if err := repo.Revoke(t.Context(), token.ID, now); err != nil {
		t.Fatalf("revoke: %v", err)
	}

	found, err := repo.FindByHash(t.Context(), "hash-1")
	if err != nil {
		t.Fatalf("เพิกถอนแล้วหาไม่เจอ: %v", err)
	}
	if found.RevokedAt == nil {
		t.Fatal("revoked_at ไม่ถูกเซ็ต")
	}
	if found.Usable(now) {
		t.Fatal("เพิกถอนแล้วยังบอกว่าใช้ได้")
	}
}

// เพิกถอนซ้ำต้องไม่ทับเวลาเดิม (timestamp ของเหตุการณ์จริงต้องไม่หาย) และต้องฟ้องกลับมาว่า
// ไม่ได้แตะแถวไหน — ตัวหลังคือสิ่งที่กันไม่ให้ refresh สองอันที่ยิงพร้อมกันผ่านทั้งคู่
func TestRevokeTwiceKeepsFirstTimeAndReports(t *testing.T) {
	reset(t)
	user := newUser(t, "daew@example.com")
	token := newRefreshToken(t, user.ID, "hash-1")

	repo := repository.NewRefreshTokenRepository(testDB)
	first := time.Now().Add(-time.Hour).Truncate(time.Millisecond)

	if err := repo.Revoke(t.Context(), token.ID, first); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if err := repo.Revoke(t.Context(), token.ID, time.Now()); !errors.Is(err, apperr.ErrInvalidRefresh) {
		t.Fatalf("revoke ซ้ำต้องได้ ErrInvalidRefresh ได้: %v", err)
	}

	found, err := repo.FindByHash(t.Context(), "hash-1")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if !found.RevokedAt.UTC().Truncate(time.Millisecond).Equal(first.UTC()) {
		t.Fatalf("เวลาเพิกถอนถูกทับ: %v อยากได้ %v", found.RevokedAt.UTC(), first.UTC())
	}
}

func TestRevokeAllForUserOnlyTouchesThatUser(t *testing.T) {
	reset(t)
	daew := newUser(t, "daew@example.com")
	somchai := newUser(t, "somchai@example.com")

	newRefreshToken(t, daew.ID, "daew-1")
	newRefreshToken(t, daew.ID, "daew-2")
	newRefreshToken(t, somchai.ID, "somchai-1")

	repo := repository.NewRefreshTokenRepository(testDB)

	if err := repo.RevokeAllForUser(t.Context(), daew.ID, time.Now()); err != nil {
		t.Fatalf("revoke all: %v", err)
	}

	for _, hash := range []string{"daew-1", "daew-2"} {
		found, err := repo.FindByHash(t.Context(), hash)
		if err != nil {
			t.Fatalf("find %s: %v", hash, err)
		}
		if found.RevokedAt == nil {
			t.Fatalf("%s ยังไม่ถูกเพิกถอน", hash)
		}
	}

	other, err := repo.FindByHash(t.Context(), "somchai-1")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if other.RevokedAt != nil {
		t.Fatal("ไปเพิกถอน session ของคนอื่นด้วย")
	}
}

// ลบ user แล้ว session ต้องตายตาม (FK เป็น CASCADE ต่างจาก blogs ที่เป็น RESTRICT)
func TestRefreshTokensDieWithUser(t *testing.T) {
	reset(t)
	user := newUser(t, "daew@example.com")
	newRefreshToken(t, user.ID, "hash-1")

	if err := testDB.Exec("DELETE FROM users WHERE id = ?", user.ID).Error; err != nil {
		t.Fatalf("ลบ user: %v", err)
	}

	var rows int64
	if err := testDB.Raw("SELECT count(*) FROM refresh_tokens").Scan(&rows).Error; err != nil {
		t.Fatalf("นับแถว: %v", err)
	}
	if rows != 0 {
		t.Fatalf("ลบ user แล้วยังเหลือ token %d แถว", rows)
	}
}

// สองคำขอชิงเพิกถอน token ใบเดียวกันพร้อมกัน ต้องสำเร็จได้ตัวเดียว —
// postgres ล็อกแถวให้ ตัวที่มาทีหลังจะเห็น revoked_at ไม่ null แล้ว update ไม่โดนสักแถว
// นี่คือของที่กันไม่ให้ refresh ใบเดียวแตกเป็นสองสาย เลยต้องมีเทสต์ยืนยันของจริง
func TestRevokeConcurrentOnlyOneWins(t *testing.T) {
	reset(t)
	user := newUser(t, "daew@example.com")
	token := newRefreshToken(t, user.ID, "hash-race")

	repo := repository.NewRefreshTokenRepository(testDB)
	now := time.Now()

	var wg sync.WaitGroup
	errs := make([]error, 2)

	for i := range errs {
		wg.Add(1)

		go func() {
			defer wg.Done()

			errs[i] = repo.Revoke(context.Background(), token.ID, now)
		}()
	}

	wg.Wait()

	won := 0
	for _, err := range errs {
		switch {
		case err == nil:
			won++
		case errors.Is(err, apperr.ErrInvalidRefresh):
		default:
			t.Fatalf("error ที่ไม่ได้คาดไว้: %v", err)
		}
	}

	if won != 1 {
		t.Fatalf("ต้องมีตัวชนะตัวเดียว แต่ชนะ %d ตัว", won)
	}
}

// กวาดของหมดอายุต้องไม่แตะใบที่ยังไม่หมดอายุ ถึงจะถูกเพิกถอนไปแล้วก็ตาม —
// ใบที่เพิกถอนแล้วแต่ยังไม่หมดอายุคือของที่ใช้จับการเอา token ไปใช้ซ้ำ ลบทิ้งแล้ว reuse detection ตาบอด
func TestDeleteExpiredKeepsRevokedButLiveTokens(t *testing.T) {
	reset(t)
	user := newUser(t, "daew@example.com")
	repo := repository.NewRefreshTokenRepository(testDB)
	now := time.Now()

	expired := &model.RefreshToken{
		UserID: user.ID, TokenHash: "หมดอายุแล้ว", ExpiresAt: now.Add(-time.Hour),
	}
	if err := repo.Create(t.Context(), expired); err != nil {
		t.Fatalf("create: %v", err)
	}

	revokedButLive := newRefreshToken(t, user.ID, "เพิกถอนแล้วแต่ยังไม่หมดอายุ")
	if err := repo.Revoke(t.Context(), revokedButLive.ID, now); err != nil {
		t.Fatalf("revoke: %v", err)
	}

	newRefreshToken(t, user.ID, "ยังใช้ได้")

	deleted, err := repo.DeleteExpired(t.Context(), now)
	if err != nil {
		t.Fatalf("delete expired: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("ลบไป %d แถว อยากได้ 1", deleted)
	}

	if _, err := repo.FindByHash(t.Context(), "หมดอายุแล้ว"); !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("ใบที่หมดอายุยังอยู่: %v", err)
	}
	for _, hash := range []string{"เพิกถอนแล้วแต่ยังไม่หมดอายุ", "ยังใช้ได้"} {
		if _, err := repo.FindByHash(t.Context(), hash); err != nil {
			t.Fatalf("%s ถูกลบไปด้วย: %v", hash, err)
		}
	}
}
