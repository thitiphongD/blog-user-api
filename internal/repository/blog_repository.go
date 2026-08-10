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

	if err := conn(ctx, r.db).Preload("User").First(&blog, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.NotFound("Blog")
		}
		return nil, fmt.Errorf("find blog by id: %w", err)
	}

	return &blog, nil
}

// FindAll ต้อง Preload("User") เพราะ BlogResponse มี author — ไม่งั้นเจอ N+1
func (r *BlogRepository) FindAll(ctx context.Context, f model.BlogFilter) ([]model.Blog, error) {
	var blogs []model.Blog

	err := r.filtered(ctx, f).
		Preload("User").
		Order(orderClause(f)).
		Offset(f.Offset).
		Limit(f.Limit).
		Find(&blogs).Error
	if err != nil {
		return nil, fmt.Errorf("find all blogs: %w", err)
	}

	return blogs, nil
}

func (r *BlogRepository) Count(ctx context.Context, f model.BlogFilter) (int64, error) {
	var total int64

	if err := r.filtered(ctx, f).Count(&total).Error; err != nil {
		return 0, fmt.Errorf("count blogs: %w", err)
	}

	return total, nil
}

// filtered ประกอบ where ที่ FindAll กับ Count ต้องใช้เหมือนกัน
// แยกไว้เพื่อไม่ให้ total กับ data หลุดจากกัน
func (r *BlogRepository) filtered(ctx context.Context, f model.BlogFilter) *gorm.DB {
	q := conn(ctx, r.db).Model(&model.Blog{})

	if f.Search != "" {
		pattern := "%" + f.Search + "%"
		q = q.Where("title ILIKE ? OR content ILIKE ?", pattern, pattern)
	}

	if f.UserID != nil {
		q = q.Where("user_id = ?", *f.UserID)
	}

	return q
}

// orderClause ประกอบจาก constant ที่ผ่าน whitelist แล้วเท่านั้น ไม่เอา string จาก query มาต่อตรงๆ
//
// ต่อ id ปิดท้ายเสมอเพื่อให้ลำดับนิ่ง — ค่าที่ซ้ำกัน (title โหลๆ หรือ created_at ที่เท่ากัน
// เพราะ insert รวดเดียว) postgres คืนลำดับไม่คงที่ระหว่างหน้า แถวเลยหลุดหรือโผล่ซ้ำได้
// ตอนไล่ page ตัว id เป็น UUID ลำดับเลยไม่มีความหมายอะไร ขอแค่คงที่ก็พอ
func orderClause(f model.BlogFilter) string {
	column := model.SortCreatedAt
	if f.Sort == model.SortTitle {
		column = model.SortTitle
	}

	direction := "DESC"
	if f.Order == model.OrderAsc {
		direction = "ASC"
	}

	return column + " " + direction + ", id"
}

func (r *BlogRepository) Create(ctx context.Context, blog *model.Blog) error {
	if err := conn(ctx, r.db).Create(blog).Error; err != nil {
		return fmt.Errorf("create blog: %w", err)
	}

	return nil
}

// Update เขียนเฉพาะ title/content — ส่ง map ไม่ใช่ struct เพราะ Updates(struct)
// จะเขียนทุก field ที่ไม่ใช่ zero value ลงไปด้วย รวมถึง user_id ซึ่งไม่ควรเปลี่ยนได้ทางนี้
func (r *BlogRepository) Update(ctx context.Context, blog *model.Blog) error {
	err := conn(ctx, r.db).
		Model(blog).
		Updates(map[string]any{"title": blog.Title, "content": blog.Content}).Error
	if err != nil {
		return fmt.Errorf("update blog: %w", err)
	}

	return nil
}

// Delete เป็น soft delete — model มี gorm.DeletedAt อยู่แล้ว GORM จัดการให้เอง
func (r *BlogRepository) Delete(ctx context.Context, id uuid.UUID) error {
	result := conn(ctx, r.db).Delete(&model.Blog{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("delete blog: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return apperr.NotFound("Blog")
	}

	return nil
}
