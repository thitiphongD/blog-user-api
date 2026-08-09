package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/thitiphongD/blog-user-api/internal/apperr"
	"github.com/thitiphongD/blog-user-api/internal/dto/request"
	"github.com/thitiphongD/blog-user-api/internal/model"
)

type CommentService struct {
	comments CommentRepository
	blogs    BlogRepository
	tx       Transactor
}

func NewCommentService(comments CommentRepository, blogs BlogRepository, tx Transactor) *CommentService {
	return &CommentService{comments: comments, blogs: blogs, tx: tx}
}

// GetComments เช็คก่อนว่า blog มีอยู่จริง — ไม่งั้น blog ที่ถูกลบไปแล้วจะคืน list ว่างเหมือนมีอยู่
func (s *CommentService) GetComments(
	ctx context.Context,
	blogID uuid.UUID,
	q request.ListQuery,
) ([]model.Comment, int64, error) {
	if _, err := s.blogs.FindByID(ctx, blogID); err != nil {
		return nil, 0, err
	}

	total, err := s.comments.CountByBlog(ctx, blogID)
	if err != nil {
		return nil, 0, err
	}

	comments, err := s.comments.FindAllByBlog(ctx, blogID, q.Offset(), q.Limit)
	if err != nil {
		return nil, 0, err
	}

	return comments, total, nil
}

func (s *CommentService) CreateComment(
	ctx context.Context,
	userID, blogID uuid.UUID,
	req request.CreateCommentRequest,
) (*model.Comment, error) {
	// blog ที่ถูกลบไปแล้วต้องคอมเมนต์ไม่ได้ — FK กันแค่กรณีลบถาวร ไม่ได้กัน soft delete
	if _, err := s.blogs.FindByID(ctx, blogID); err != nil {
		return nil, err
	}

	comment := &model.Comment{
		Content: req.Content,
		BlogID:  blogID,
		UserID:  userID,
	}

	var created *model.Comment

	err := s.tx.Do(ctx, func(ctx context.Context) error {
		if err := s.comments.Create(ctx, comment); err != nil {
			return err
		}

		found, err := s.comments.FindByID(ctx, comment.ID)
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

func (s *CommentService) UpdateComment(
	ctx context.Context,
	userID, id uuid.UUID,
	req request.UpdateCommentRequest,
) (*model.Comment, error) {
	comment, err := s.comments.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if comment.UserID != userID {
		return nil, apperr.ErrForbidden
	}

	comment.Content = req.Content

	var updated *model.Comment

	err = s.tx.Do(ctx, func(ctx context.Context) error {
		if err := s.comments.Update(ctx, comment); err != nil {
			return err
		}

		found, err := s.comments.FindByID(ctx, id)
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

// DeleteComment ให้สิทธิ์เจ้าของคอมเมนต์เท่านั้น — เจ้าของ blog ลบคอมเมนต์คนอื่นไม่ได้
// (เป็นเรื่อง moderation ซึ่งยังไม่อยู่ใน scope จงใจไม่ทำ ไม่ใช่ลืม)
func (s *CommentService) DeleteComment(ctx context.Context, userID, id uuid.UUID) error {
	comment, err := s.comments.FindByID(ctx, id)
	if err != nil {
		return err
	}

	if comment.UserID != userID {
		return apperr.ErrForbidden
	}

	return s.comments.Delete(ctx, id)
}
