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
	Token     string
	ExpiredAt time.Time
	User      *model.User
}

type AuthService struct {
	users UserRepository
	jwt   *auth.JWT
}

func NewAuthService(users UserRepository, jwt *auth.JWT) *AuthService {
	return &AuthService{users: users, jwt: jwt}
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

	token, expiredAt, err := s.jwt.Generate(user.ID)
	if err != nil {
		return nil, err
	}

	return &LoginResult{Token: token, ExpiredAt: expiredAt, User: user}, nil
}

func (s *AuthService) GetMe(ctx context.Context, userID uuid.UUID) (*model.User, error) {
	return s.users.FindByID(ctx, userID)
}
