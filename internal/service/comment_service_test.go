package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/thitiphongD/blog-user-api/internal/apperr"
	"github.com/thitiphongD/blog-user-api/internal/dto/request"
	"github.com/thitiphongD/blog-user-api/internal/model"
)

func blogExists(id uuid.UUID) *mockBlogRepo {
	return &mockBlogRepo{
		findByID: func(context.Context, uuid.UUID) (*model.Blog, error) {
			return &model.Blog{ID: id}, nil
		},
	}
}

func blogMissing() *mockBlogRepo {
	return &mockBlogRepo{
		findByID: func(context.Context, uuid.UUID) (*model.Blog, error) {
			return nil, apperr.NotFound("Blog")
		},
	}
}

func TestGetCommentsPassesPagination(t *testing.T) {
	blogID := uuid.New()

	var gotBlog uuid.UUID
	var gotOffset, gotLimit int

	comments := &mockCommentRepo{
		countByBlog: func(context.Context, uuid.UUID) (int64, error) { return 7, nil },
		findAllByBlog: func(_ context.Context, id uuid.UUID, offset, limit int) ([]model.Comment, error) {
			gotBlog, gotOffset, gotLimit = id, offset, limit

			return []model.Comment{{Content: "อันแรก"}}, nil
		},
	}

	got, total, err := NewCommentService(comments, blogExists(blogID), &fakeTx{}).
		GetComments(context.Background(), blogID, listQuery(2, 5))
	if err != nil {
		t.Fatalf("get comments: %v", err)
	}

	if total != 7 || len(got) != 1 {
		t.Fatalf("total=%d len=%d", total, len(got))
	}
	if gotBlog != blogID || gotOffset != 5 || gotLimit != 5 {
		t.Fatalf("ส่งค่าไป repository ผิด: blog=%v offset=%d limit=%d", gotBlog, gotOffset, gotLimit)
	}
}

// blog ที่ไม่มีอยู่ต้องได้ 404 ไม่ใช่ list ว่างเหมือนมี blog อยู่แต่ยังไม่มีใครคอมเมนต์
func TestGetCommentsOfMissingBlog(t *testing.T) {
	// ไม่เซ็ต mock ของ comment เลย ถ้าหลุดไปถึงจะ panic
	_, _, err := NewCommentService(&mockCommentRepo{}, blogMissing(), &fakeTx{}).
		GetComments(context.Background(), uuid.New(), listQuery(1, 20))

	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("อยากได้ ErrNotFound ได้ %v", err)
	}
}

// FK กันได้แค่ตอนลบถาวร — blog ที่ถูก soft delete ยังมีแถวอยู่ ต้องเช็คเองก่อนเขียน
func TestCreateCommentOnMissingBlog(t *testing.T) {
	_, err := NewCommentService(&mockCommentRepo{}, blogMissing(), &fakeTx{}).
		CreateComment(context.Background(), uuid.New(), uuid.New(), request.CreateCommentRequest{Content: "x"})

	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("อยากได้ ErrNotFound ได้ %v", err)
	}
}

func TestCreateCommentUsesTransaction(t *testing.T) {
	blogID := uuid.New()
	userID := uuid.New()
	tx := &fakeTx{}

	var sent *model.Comment

	comments := &mockCommentRepo{
		create: func(_ context.Context, c *model.Comment) error {
			c.ID = uuid.New()
			sent = c

			return nil
		},
		findByID: func(_ context.Context, id uuid.UUID) (*model.Comment, error) {
			return &model.Comment{ID: id, User: model.User{Name: "Daew"}}, nil
		},
	}

	got, err := NewCommentService(comments, blogExists(blogID), tx).
		CreateComment(context.Background(), userID, blogID, request.CreateCommentRequest{Content: "สวัสดี"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if !tx.called {
		t.Fatal("create + อ่านกลับ ไม่ได้อยู่ใน transaction")
	}
	if sent.UserID != userID || sent.BlogID != blogID {
		t.Fatalf("ผูกกับ user/blog ผิด: %+v", sent)
	}
	if got.User.Name != "Daew" {
		t.Fatal("ไม่ได้คืนตัวที่อ่านกลับมา author จะหาย")
	}
}

func TestUpdateCommentRejectsNonOwner(t *testing.T) {
	comments := &mockCommentRepo{
		findByID: func(_ context.Context, id uuid.UUID) (*model.Comment, error) {
			return &model.Comment{ID: id, UserID: uuid.New()}, nil
		},
		// ไม่เซ็ต update → ถ้าถูกเรียกจะ panic
	}

	_, err := NewCommentService(comments, &mockBlogRepo{}, &fakeTx{}).
		UpdateComment(context.Background(), uuid.New(), uuid.New(), request.UpdateCommentRequest{Content: "แอบแก้"})

	if !errors.Is(err, apperr.ErrForbidden) {
		t.Fatalf("อยากได้ ErrForbidden ได้ %v", err)
	}
}

func TestUpdateCommentByOwner(t *testing.T) {
	owner := uuid.New()
	id := uuid.New()

	var sent *model.Comment

	comments := &mockCommentRepo{
		findByID: func(_ context.Context, got uuid.UUID) (*model.Comment, error) {
			return &model.Comment{ID: got, UserID: owner, Content: "ของเดิม"}, nil
		},
		update: func(_ context.Context, c *model.Comment) error {
			sent = c

			return nil
		},
	}

	got, err := NewCommentService(comments, &mockBlogRepo{}, &fakeTx{}).
		UpdateComment(context.Background(), owner, id, request.UpdateCommentRequest{Content: "แก้แล้ว"})
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	if sent == nil || sent.Content != "แก้แล้ว" {
		t.Fatalf("ค่าที่ส่งให้ repository ผิด: %+v", sent)
	}
	if sent.UserID != owner || sent.BlogID != got.BlogID {
		t.Fatal("เจ้าของหรือ blog ถูกเปลี่ยนระหว่างทาง")
	}
}

func TestDeleteCommentRejectsNonOwner(t *testing.T) {
	comments := &mockCommentRepo{
		findByID: func(_ context.Context, id uuid.UUID) (*model.Comment, error) {
			return &model.Comment{ID: id, UserID: uuid.New()}, nil
		},
		// ไม่เซ็ต delete → ถ้าถูกเรียกจะ panic
	}

	err := NewCommentService(comments, &mockBlogRepo{}, &fakeTx{}).
		DeleteComment(context.Background(), uuid.New(), uuid.New())

	if !errors.Is(err, apperr.ErrForbidden) {
		t.Fatalf("อยากได้ ErrForbidden ได้ %v", err)
	}
}

func TestDeleteCommentByOwner(t *testing.T) {
	owner := uuid.New()
	deleted := false

	comments := &mockCommentRepo{
		findByID: func(_ context.Context, id uuid.UUID) (*model.Comment, error) {
			return &model.Comment{ID: id, UserID: owner}, nil
		},
		delete: func(context.Context, uuid.UUID) error {
			deleted = true

			return nil
		},
	}

	err := NewCommentService(comments, &mockBlogRepo{}, &fakeTx{}).
		DeleteComment(context.Background(), owner, uuid.New())
	if err != nil {
		t.Fatalf("delete: %v", err)
	}

	if !deleted {
		t.Fatal("เจ้าของลบแล้วแต่ repository.Delete ไม่ถูกเรียก")
	}
}

func TestCommentErrorsPropagate(t *testing.T) {
	blogID := uuid.New()

	t.Run("count พังแล้วไม่ยิง FindAll ต่อ", func(t *testing.T) {
		comments := &mockCommentRepo{
			countByBlog: func(context.Context, uuid.UUID) (int64, error) { return 0, errRepo },
			findAllByBlog: func(context.Context, uuid.UUID, int, int) ([]model.Comment, error) {
				t.Error("count พังแล้วยังไปเรียก FindAllByBlog ต่อ")

				return nil, errRepo
			},
		}

		_, _, err := NewCommentService(comments, blogExists(blogID), &fakeTx{}).
			GetComments(context.Background(), blogID, listQuery(1, 20))
		if !errors.Is(err, errRepo) {
			t.Fatalf("อยากได้ errRepo ได้ %v", err)
		}
	})

	t.Run("FindAll พัง", func(t *testing.T) {
		comments := &mockCommentRepo{
			countByBlog: func(context.Context, uuid.UUID) (int64, error) { return 1, nil },
			findAllByBlog: func(context.Context, uuid.UUID, int, int) ([]model.Comment, error) {
				return nil, errRepo
			},
		}

		_, _, err := NewCommentService(comments, blogExists(blogID), &fakeTx{}).
			GetComments(context.Background(), blogID, listQuery(1, 20))
		if !errors.Is(err, errRepo) {
			t.Fatalf("อยากได้ errRepo ได้ %v", err)
		}
	})

	t.Run("create พังแล้วไม่อ่านกลับ", func(t *testing.T) {
		comments := &mockCommentRepo{
			create: func(context.Context, *model.Comment) error { return errRepo },
			findByID: func(context.Context, uuid.UUID) (*model.Comment, error) {
				t.Error("create พังแล้วยังไปอ่านกลับ")

				return nil, errRepo
			},
		}

		_, err := NewCommentService(comments, blogExists(blogID), &fakeTx{}).
			CreateComment(context.Background(), uuid.New(), blogID, request.CreateCommentRequest{Content: "x"})
		if !errors.Is(err, errRepo) {
			t.Fatalf("อยากได้ errRepo ได้ %v", err)
		}
	})

	t.Run("หา comment ที่จะแก้ไม่เจอ", func(t *testing.T) {
		comments := &mockCommentRepo{
			findByID: func(context.Context, uuid.UUID) (*model.Comment, error) {
				return nil, apperr.NotFound("Comment")
			},
		}

		_, err := NewCommentService(comments, &mockBlogRepo{}, &fakeTx{}).
			UpdateComment(context.Background(), uuid.New(), uuid.New(), request.UpdateCommentRequest{Content: "x"})
		if !errors.Is(err, apperr.ErrNotFound) {
			t.Fatalf("อยากได้ ErrNotFound ได้ %v", err)
		}
	})

	t.Run("update พัง", func(t *testing.T) {
		owner := uuid.New()
		comments := &mockCommentRepo{
			findByID: func(_ context.Context, id uuid.UUID) (*model.Comment, error) {
				return &model.Comment{ID: id, UserID: owner}, nil
			},
			update: func(context.Context, *model.Comment) error { return errRepo },
		}

		_, err := NewCommentService(comments, &mockBlogRepo{}, &fakeTx{}).
			UpdateComment(context.Background(), owner, uuid.New(), request.UpdateCommentRequest{Content: "x"})
		if !errors.Is(err, errRepo) {
			t.Fatalf("อยากได้ errRepo ได้ %v", err)
		}
	})

	t.Run("หา comment ที่จะลบไม่เจอ", func(t *testing.T) {
		comments := &mockCommentRepo{
			findByID: func(context.Context, uuid.UUID) (*model.Comment, error) {
				return nil, apperr.NotFound("Comment")
			},
		}

		err := NewCommentService(comments, &mockBlogRepo{}, &fakeTx{}).
			DeleteComment(context.Background(), uuid.New(), uuid.New())
		if !errors.Is(err, apperr.ErrNotFound) {
			t.Fatalf("อยากได้ ErrNotFound ได้ %v", err)
		}
	})
}
