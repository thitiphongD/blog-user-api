package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/thitiphongD/blog-user-api/internal/apperr"
	"github.com/thitiphongD/blog-user-api/internal/dto/request"
	"github.com/thitiphongD/blog-user-api/internal/model"
)

type BlogService struct {
	blogs BlogRepository
	tx    Transactor
}

func NewBlogService(blogs BlogRepository, tx Transactor) *BlogService {
	return &BlogService{blogs: blogs, tx: tx}
}

func (s *BlogService) GetBlogs(ctx context.Context, q request.BlogQuery) ([]model.Blog, int64, error) {
	f := q.Filter()

	total, err := s.blogs.Count(ctx, f)
	if err != nil {
		return nil, 0, err
	}

	blogs, err := s.blogs.FindAll(ctx, f)
	if err != nil {
		return nil, 0, err
	}

	return blogs, total, nil
}

func (s *BlogService) GetBlogByID(ctx context.Context, id uuid.UUID) (*model.Blog, error) {
	return s.blogs.FindByID(ctx, id)
}

func (s *BlogService) CreateBlog(
	ctx context.Context,
	userID uuid.UUID,
	req request.CreateBlogRequest,
) (*model.Blog, error) {
	blog := &model.Blog{
		Title:   req.Title,
		Content: req.Content,
		UserID:  userID,
	}

	var created *model.Blog

	// เขียนแล้วอ่านกลับ (เพื่อเอา author มาด้วย) เป็นสองคำสั่ง มัดไว้ใน transaction เดียว
	// จะได้ไม่มีจังหวะที่อ่านเจอสถานะครึ่งๆ กลางๆ
	err := s.tx.Do(ctx, func(ctx context.Context) error {
		if err := s.blogs.Create(ctx, blog); err != nil {
			return err
		}

		found, err := s.blogs.FindByID(ctx, blog.ID)
		if err != nil {
			return err
		}

		created = found

		return nil
	})
	if err != nil {
		return nil, err
	}

	return created, nil
}

func (s *BlogService) UpdateBlog(
	ctx context.Context,
	userID, id uuid.UUID,
	req request.UpdateBlogRequest,
) (*model.Blog, error) {
	blog, err := s.blogs.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if blog.UserID != userID {
		return nil, apperr.ErrForbidden
	}

	blog.Title = req.Title
	blog.Content = req.Content

	var updated *model.Blog

	err = s.tx.Do(ctx, func(ctx context.Context) error {
		if err := s.blogs.Update(ctx, blog); err != nil {
			return err
		}

		found, err := s.blogs.FindByID(ctx, id)
		if err != nil {
			return err
		}

		updated = found

		return nil
	})
	if err != nil {
		return nil, err
	}

	return updated, nil
}

func (s *BlogService) DeleteBlog(ctx context.Context, userID, id uuid.UUID) error {
	blog, err := s.blogs.FindByID(ctx, id)
	if err != nil {
		return err
	}

	// ไม่ใช่เจ้าของคืน 403 ไม่ใช่ 404 — blog มีอยู่จริง แค่ไม่มีสิทธิ์
	if blog.UserID != userID {
		return apperr.ErrForbidden
	}

	return s.blogs.Delete(ctx, id)
}
