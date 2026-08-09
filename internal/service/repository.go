// Package service เก็บ business logic ทั้งหมด
//
// interface ของ repository ประกาศไว้ฝั่งนี้ (ฝั่งที่ใช้) ไม่ใช่ฝั่ง repository —
// service เลยไม่ผูกกับ GORM และ unit test ใช้ mock ได้โดยไม่ต้องต่อ DB
package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/thitiphongD/blog-user-api/internal/model"
)

type UserRepository interface {
	FindByEmail(ctx context.Context, email string) (*model.User, error)
	FindByID(ctx context.Context, id uuid.UUID) (*model.User, error)
	FindAll(ctx context.Context, offset, limit int) ([]model.User, error)
	Count(ctx context.Context) (int64, error)
	Create(ctx context.Context, user *model.User) error
}

type BlogRepository interface {
	FindByID(ctx context.Context, id uuid.UUID) (*model.Blog, error)
	FindAll(ctx context.Context, offset, limit int) ([]model.Blog, error)
	Count(ctx context.Context) (int64, error)
	Create(ctx context.Context, blog *model.Blog) error
	Update(ctx context.Context, blog *model.Blog) error
	Delete(ctx context.Context, id uuid.UUID) error
}
