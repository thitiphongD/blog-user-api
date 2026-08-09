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

type CommentRepository struct {
	db *gorm.DB
}

func NewCommentRepository(db *gorm.DB) *CommentRepository {
	return &CommentRepository{db: db}
}

func (r *CommentRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.Comment, error) {
	var comment model.Comment

	if err := conn(ctx, r.db).Preload("User").First(&comment, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.NotFound("Comment")
		}

		return nil, fmt.Errorf("find comment by id: %w", err)
	}

	return &comment, nil
}

// FindAllByBlog เรียงเก่าไปใหม่ ต่างจาก blog ที่เรียงใหม่ไปเก่า — บทสนทนาต้องอ่านไล่จากบนลงล่าง
func (r *CommentRepository) FindAllByBlog(
	ctx context.Context,
	blogID uuid.UUID,
	offset, limit int,
) ([]model.Comment, error) {
	var comments []model.Comment

	err := conn(ctx, r.db).
		Preload("User").
		Where("blog_id = ?", blogID).
		Order("created_at ASC").
		Offset(offset).
		Limit(limit).
		Find(&comments).Error
	if err != nil {
		return nil, fmt.Errorf("find comments: %w", err)
	}

	return comments, nil
}

func (r *CommentRepository) CountByBlog(ctx context.Context, blogID uuid.UUID) (int64, error) {
	var total int64

	err := conn(ctx, r.db).Model(&model.Comment{}).
		Where("blog_id = ?", blogID).
		Count(&total).Error
	if err != nil {
		return 0, fmt.Errorf("count comments: %w", err)
	}

	return total, nil
}

func (r *CommentRepository) Create(ctx context.Context, comment *model.Comment) error {
	if err := conn(ctx, r.db).Create(comment).Error; err != nil {
		return fmt.Errorf("create comment: %w", err)
	}

	return nil
}

// Update ส่ง map ไม่ใช่ struct ด้วยเหตุผลเดียวกับ blog — กัน field อื่นถูกเขียนตามไปด้วย
func (r *CommentRepository) Update(ctx context.Context, comment *model.Comment) error {
	err := conn(ctx, r.db).
		Model(comment).
		Updates(map[string]any{"content": comment.Content}).Error
	if err != nil {
		return fmt.Errorf("update comment: %w", err)
	}

	return nil
}

func (r *CommentRepository) Delete(ctx context.Context, id uuid.UUID) error {
	result := conn(ctx, r.db).Delete(&model.Comment{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("delete comment: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return apperr.NotFound("Comment")
	}

	return nil
}
