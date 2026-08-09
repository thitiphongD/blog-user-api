package response

import (
	"time"

	"github.com/thitiphongD/blog-user-api/internal/model"
)

type AuthResponse struct {
	Token     string       `json:"token"`
	ExpiredAt time.Time    `json:"expired_at"`
	User      UserResponse `json:"user"`
}

func NewAuthResponse(token string, expiredAt time.Time, u model.User) AuthResponse {
	return AuthResponse{
		Token:     token,
		ExpiredAt: expiredAt.UTC(),
		User:      NewUserResponse(u),
	}
}
