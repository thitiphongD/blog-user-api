package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/thitiphongD/blog-user-api/internal/apperr"
	"github.com/thitiphongD/blog-user-api/internal/model"
)

type BlogRepository struct {
	db *gorm.DB
}

func NewBlogRepository(db *gorm.DB) *BlogRepository {
	return &BlogRepository{db: db}
}

func (r *BlogRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.Blog, error) {
	var blog model.Blog

	if err := r.db.WithContext(ctx).Preload("User").First(&blog, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.NotFound("Blog")
		}
		return nil, fmt.Errorf("find blog by id: %w", err)
	}

	return &blog, nil
}

// FindAll ต้อง Preload("User") เพราะ BlogResponse มี author — ไม่งั้นเจอ N+1
func (r *BlogRepository) FindAll(ctx context.Context, offset, limit int) ([]model.Blog, error) {
	var blogs []model.Blog

	err := r.db.WithContext(ctx).
		Preload("User").
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&blogs).Error
	if err != nil {
		return nil, fmt.Errorf("find all blogs: %w", err)
	}

	return blogs, nil
}

func (r *BlogRepository) Count(ctx context.Context) (int64, error) {
	var total int64

	if err := r.db.WithContext(ctx).Model(&model.Blog{}).Count(&total).Error; err != nil {
		return 0, fmt.Errorf("count blogs: %w", err)
	}

	return total, nil
}

func (r *BlogRepository) Create(ctx context.Context, blog *model.Blog) error {
	if err := r.db.WithContext(ctx).Create(blog).Error; err != nil {
		return fmt.Errorf("create blog: %w", err)
	}

	return nil
}

func (r *BlogRepository) Update(ctx context.Context, blog *model.Blog) error {
	err := r.db.WithContext(ctx).
		Model(blog).
		Select("title", "content").
		Updates(map[string]any{"title": blog.Title, "content": blog.Content}).Error
	if err != nil {
		return fmt.Errorf("update blog: %w", err)
	}

	return nil
}

// Delete เป็น soft delete — model มี gorm.DeletedAt อยู่แล้ว GORM จัดการให้เอง
func (r *BlogRepository) Delete(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).Delete(&model.Blog{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("delete blog: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return apperr.NotFound("Blog")
	}

	return nil
}
