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
}

func NewBlogService(blogs BlogRepository) *BlogService {
	return &BlogService{blogs: blogs}
}

func (s *BlogService) GetBlogs(ctx context.Context, q request.ListQuery) ([]model.Blog, int64, error) {
	total, err := s.blogs.Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	blogs, err := s.blogs.FindAll(ctx, q.Offset(), q.Limit)
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

	if err := s.blogs.Create(ctx, blog); err != nil {
		return nil, err
	}

	// อ่านกลับมาเพื่อให้ได้ author ติดมาด้วย response จะได้ครบเหมือน endpoint อื่น
	return s.blogs.FindByID(ctx, blog.ID)
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

	if err := s.blogs.Update(ctx, blog); err != nil {
		return nil, err
	}

	return s.blogs.FindByID(ctx, id)
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
