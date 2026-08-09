package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/thitiphongD/blog-user-api/internal/dto/request"
	"github.com/thitiphongD/blog-user-api/internal/model"
)

type UserService struct {
	users UserRepository
}

func NewUserService(users UserRepository) *UserService {
	return &UserService{users: users}
}

func (s *UserService) GetUsers(ctx context.Context, q request.ListQuery) ([]model.User, int64, error) {
	total, err := s.users.Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	users, err := s.users.FindAll(ctx, q.Offset(), q.Limit)
	if err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

func (s *UserService) GetUserByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	return s.users.FindByID(ctx, id)
}
