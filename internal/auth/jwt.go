// Package auth ทำ JWT กับ password hash ล้วนๆ ไม่รู้จัก echo ไม่รู้จัก HTTP
package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Claims struct {
	UserID uuid.UUID `json:"user_id"`
	jwt.RegisteredClaims
}

type JWT struct {
	secret []byte
	expire time.Duration
}

func NewJWT(secret string, expire time.Duration) *JWT {
	return &JWT{secret: []byte(secret), expire: expire}
}

// Generate คืน token กับเวลาหมดอายุ เพื่อให้ response บอก expired_at ได้ตรงกับใน token
func (j *JWT) Generate(userID uuid.UUID) (string, time.Time, error) {
	now := time.Now()
	expiresAt := now.Add(j.expire)

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	})

	signed, err := token.SignedString(j.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign token: %w", err)
	}

	return signed, expiresAt, nil
}

func (j *JWT) Verify(token string) (uuid.UUID, error) {
	claims := &Claims{}

	// WithValidMethods ตัด token ที่ alg ไม่ใช่ HS256 ทิ้งตั้งแต่ก่อนเรียก keyfunc
	// (เช็คซ้ำใน keyfunc อีกรอบคือโค้ดที่ไม่มีวันวิ่งถึง)
	parsed, err := jwt.ParseWithClaims(token, claims, func(*jwt.Token) (any, error) {
		return j.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil {
		return uuid.Nil, fmt.Errorf("parse token: %w", err)
	}

	if !parsed.Valid || claims.UserID == uuid.Nil {
		return uuid.Nil, errors.New("invalid token")
	}

	return claims.UserID, nil
}
