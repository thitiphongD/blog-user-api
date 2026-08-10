package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/thitiphongD/blog-user-api/internal/apperr"
	"github.com/thitiphongD/blog-user-api/internal/auth"
	"github.com/thitiphongD/blog-user-api/internal/dto/request"
	"github.com/thitiphongD/blog-user-api/internal/model"
)

type LoginResult struct {
	Token            string
	ExpiredAt        time.Time
	RefreshToken     string
	RefreshExpiredAt time.Time
	User             *model.User
}

type AuthService struct {
	users      UserRepository
	refresh    RefreshTokenRepository
	tx         Transactor
	jwt        *auth.JWT
	refreshTTL time.Duration
	now        func() time.Time
}

func NewAuthService(
	users UserRepository,
	refresh RefreshTokenRepository,
	tx Transactor,
	jwt *auth.JWT,
	refreshTTL time.Duration,
) *AuthService {
	return &AuthService{
		users:      users,
		refresh:    refresh,
		tx:         tx,
		jwt:        jwt,
		refreshTTL: refreshTTL,
		now:        time.Now,
	}
}

// Register ไม่ auto-login — อยากได้ token ก็ยิง /auth/login ต่อ
func (s *AuthService) Register(ctx context.Context, req request.RegisterRequest) (*model.User, error) {
	hashed, err := auth.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}

	user := &model.User{
		Name:     req.Name,
		Email:    req.Email,
		Password: hashed,
	}

	// ไม่เช็ค email ซ้ำก่อน แล้วค่อย create — race กันได้ ปล่อยให้ unique constraint ตัดสิน
	if err := s.users.Create(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

func (s *AuthService) Login(ctx context.Context, req request.LoginRequest) (*LoginResult, error) {
	user, err := s.users.FindByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, apperr.ErrNotFound) {
			// เผา CPU ให้พอๆ กับกรณีเจอ user จริง ไม่งั้นวัดเวลาแล้วเดาได้ว่า email ไหนมีในระบบ
			auth.BurnCompare(req.Password)

			return nil, apperr.ErrInvalidCredential
		}

		return nil, err
	}

	if !auth.ComparePassword(user.Password, req.Password) {
		return nil, apperr.ErrInvalidCredential
	}

	return s.issue(ctx, user)
}

// Refresh หมุน token ทุกครั้ง — ตัวเดิมถูกเพิกถอนแล้วออกใบใหม่ให้ในธุรกรรมเดียว
// ใครขโมย refresh token ไปใช้ จะทำให้ token ของเจ้าของกลายเป็นของใช้ไม่ได้ทันที
// แล้วรอบต่อไปที่เจ้าของยิงมา ระบบจะจับได้ว่ามีการใช้ซ้ำ
func (s *AuthService) Refresh(ctx context.Context, raw string) (*LoginResult, error) {
	stored, err := s.refresh.FindByHash(ctx, auth.HashRefreshToken(raw))
	if err != nil {
		if errors.Is(err, apperr.ErrNotFound) {
			return nil, apperr.ErrInvalidRefresh
		}

		return nil, err
	}

	now := s.now()

	// token ที่ถูกใช้ไปแล้วโผล่มาอีก = มีคนถืออยู่สองมือ ตัดทุก session ของ user คนนี้ทิ้ง
	if stored.RevokedAt != nil {
		if err := s.refresh.RevokeAllForUser(ctx, stored.UserID, now); err != nil {
			return nil, err
		}

		return nil, apperr.ErrInvalidRefresh
	}

	if !stored.Usable(now) {
		return nil, apperr.ErrInvalidRefresh
	}

	user, err := s.users.FindByID(ctx, stored.UserID)
	if err != nil {
		if errors.Is(err, apperr.ErrNotFound) {
			return nil, apperr.ErrInvalidRefresh
		}

		return nil, err
	}

	var result *LoginResult

	err = s.tx.Do(ctx, func(ctx context.Context) error {
		// การอ่านข้างบนอยู่นอก transaction สองคำขอที่ถือ token ใบเดียวกันเลยเห็น
		// revoked_at เป็น null ได้ทั้งคู่ ตัวที่แพ้จะโดน Revoke ตีกลับเป็น ErrInvalidRefresh
		// แล้ว transaction ทั้งก้อน rollback — ไม่มีทางออก token ใหม่สองใบจากใบเดียว
		if err := s.refresh.Revoke(ctx, stored.ID, now); err != nil {
			return err
		}

		issued, err := s.issue(ctx, user)
		if err != nil {
			return err
		}

		result = issued

		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

// Logout เพิกถอนเฉพาะ session ที่ยื่นมา ไม่ยุ่งกับเครื่องอื่นที่ user คนเดียวกัน login ค้างไว้
func (s *AuthService) Logout(ctx context.Context, raw string) error {
	stored, err := s.refresh.FindByHash(ctx, auth.HashRefreshToken(raw))
	if err != nil {
		if errors.Is(err, apperr.ErrNotFound) {
			return apperr.ErrInvalidRefresh
		}

		return err
	}

	// เพิกถอนไม่โดนสักแถว = มีคนชิงทำไปแล้ว (logout ซ้ำ หรือ refresh หมุนไปก่อน)
	// ปลายทางเหมือนกันคือ session นี้ตายแล้ว นับว่าสำเร็จ ไม่ต้องคืน error ให้ client งง
	if err := s.refresh.Revoke(ctx, stored.ID, s.now()); err != nil &&
		!errors.Is(err, apperr.ErrInvalidRefresh) {
		return err
	}

	return nil
}

// PruneExpiredTokens กวาด refresh token ที่หมดอายุแล้วทิ้ง คืนจำนวนแถวที่ลบ
// ไม่มีใครยิงผ่าน HTTP — main เรียกเป็นระยะเอง
func (s *AuthService) PruneExpiredTokens(ctx context.Context) (int64, error) {
	return s.refresh.DeleteExpired(ctx, s.now())
}

func (s *AuthService) GetMe(ctx context.Context, userID uuid.UUID) (*model.User, error) {
	return s.users.FindByID(ctx, userID)
}

// issue ออก access token คู่กับ refresh token ใบใหม่ — เก็บลง DB แค่ hash
func (s *AuthService) issue(ctx context.Context, user *model.User) (*LoginResult, error) {
	token, expiredAt, err := s.jwt.Generate(user.ID)
	if err != nil {
		return nil, err
	}

	raw, err := auth.NewRefreshToken()
	if err != nil {
		return nil, err
	}

	refreshExpiredAt := s.now().Add(s.refreshTTL)

	err = s.refresh.Create(ctx, &model.RefreshToken{
		UserID:    user.ID,
		TokenHash: auth.HashRefreshToken(raw),
		ExpiresAt: refreshExpiredAt,
	})
	if err != nil {
		return nil, err
	}

	return &LoginResult{
		Token:            token,
		ExpiredAt:        expiredAt,
		RefreshToken:     raw,
		RefreshExpiredAt: refreshExpiredAt,
		User:             user,
	}, nil
}
