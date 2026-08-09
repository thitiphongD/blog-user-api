package response

import (
	"time"

	"github.com/thitiphongD/blog-user-api/internal/model"
)

type AuthResponse struct {
	Token            string       `json:"token"`
	ExpiredAt        time.Time    `json:"expired_at"`
	RefreshToken     string       `json:"refresh_token"`
	RefreshExpiredAt time.Time    `json:"refresh_expired_at"`
	User             UserResponse `json:"user"`
}

func NewAuthResponse(
	token string,
	expiredAt time.Time,
	refreshToken string,
	refreshExpiredAt time.Time,
	u model.User,
) AuthResponse {
	return AuthResponse{
		Token:            token,
		ExpiredAt:        expiredAt.UTC(),
		RefreshToken:     refreshToken,
		RefreshExpiredAt: refreshExpiredAt.UTC(),
		User:             NewUserResponse(u),
	}
}
