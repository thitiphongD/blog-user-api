package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/thitiphongD/blog-user-api/internal/apperr"
	"github.com/thitiphongD/blog-user-api/internal/model"
)

type RefreshTokenRepository struct {
	db *gorm.DB
}

func NewRefreshTokenRepository(db *gorm.DB) *RefreshTokenRepository {
	return &RefreshTokenRepository{db: db}
}

func (r *RefreshTokenRepository) Create(ctx context.Context, token *model.RefreshToken) error {
	if err := conn(ctx, r.db).Create(token).Error; err != nil {
		return fmt.Errorf("create refresh token: %w", err)
	}

	return nil
}

// FindByHash คืนแถวแม้จะถูกเพิกถอนหรือหมดอายุแล้ว เพราะ service ต้องแยกให้ออกว่า
// "ไม่เคยมี token นี้" กับ "เคยมีแต่ถูกใช้ไปแล้ว" — อย่างหลังคือสัญญาณว่า token รั่ว
func (r *RefreshTokenRepository) FindByHash(ctx context.Context, hash string) (*model.RefreshToken, error) {
	var token model.RefreshToken

	if err := conn(ctx, r.db).Where("token_hash = ?", hash).First(&token).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.NotFound("Refresh token")
		}

		return nil, fmt.Errorf("find refresh token: %w", err)
	}

	return &token, nil
}

func (r *RefreshTokenRepository) Revoke(ctx context.Context, id uuid.UUID, at time.Time) error {
	result := conn(ctx, r.db).Model(&model.RefreshToken{}).
		Where("id = ? AND revoked_at IS NULL", id).
		Update("revoked_at", at)
	if result.Error != nil {
		return fmt.Errorf("revoke refresh token: %w", result.Error)
	}

	return nil
}

// RevokeAllForUser ใช้ตอนจับได้ว่ามีการเอา token ที่ใช้ไปแล้วมาใช้ซ้ำ
// ตัดทุก session ของ user คนนั้นทิ้ง เพราะไม่รู้ว่าใครถืออันไหนอยู่บ้าง
func (r *RefreshTokenRepository) RevokeAllForUser(ctx context.Context, userID uuid.UUID, at time.Time) error {
	err := conn(ctx, r.db).Model(&model.RefreshToken{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Update("revoked_at", at).Error
	if err != nil {
		return fmt.Errorf("revoke all refresh tokens: %w", err)
	}

	return nil
}
