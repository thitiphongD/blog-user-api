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

func blogQuery(page, limit int) request.BlogQuery {
	q := request.BlogQuery{ListQuery: request.ListQuery{Page: page, Limit: limit}}
	if err := q.Normalize(); err != nil {
		panic(err)
	}
	return q
}

func TestGetBlogsPassesFilterAndTotal(t *testing.T) {
	var gotFilter model.BlogFilter

	repo := &mockBlogRepo{
		count: func(context.Context, model.BlogFilter) (int64, error) { return 42, nil },
		findAll: func(_ context.Context, f model.BlogFilter) ([]model.Blog, error) {
			gotFilter = f
			return []model.Blog{{Title: "one"}}, nil
		},
	}

	blogs, total, err := NewBlogService(repo, &fakeTx{}).GetBlogs(context.Background(), blogQuery(3, 10))
	if err != nil {
		t.Fatalf("get blogs: %v", err)
	}

	if total != 42 {
		t.Fatalf("total = %d อยากได้ 42", total)
	}
	if len(blogs) != 1 {
		t.Fatalf("ได้ blog %d อัน อยากได้ 1", len(blogs))
	}
	if gotFilter.Offset != 20 || gotFilter.Limit != 10 {
		t.Fatalf("filter offset/limit = %d/%d อยากได้ 20/10", gotFilter.Offset, gotFilter.Limit)
	}
	if gotFilter.Sort != model.SortCreatedAt || gotFilter.Order != model.OrderDesc {
		t.Fatalf("default sort/order ไม่ถูก: %s/%s", gotFilter.Sort, gotFilter.Order)
	}
}

func TestCreateBlogRunsInTransaction(t *testing.T) {
	tx := &fakeTx{}
	created := uuid.New()

	repo := &mockBlogRepo{
		create: func(_ context.Context, b *model.Blog) error {
			b.ID = created
			return nil
		},
		findByID: func(_ context.Context, id uuid.UUID) (*model.Blog, error) {
			return &model.Blog{ID: id, Title: "Hello", User: model.User{Name: "Daew"}}, nil
		},
	}

	blog, err := NewBlogService(repo, tx).CreateBlog(context.Background(), uuid.New(), request.CreateBlogRequest{
		Title: "Hello", Content: "First post",
	})
	if err != nil {
		t.Fatalf("create blog: %v", err)
	}

	if !tx.called {
		t.Fatal("create + อ่านกลับ ไม่ได้อยู่ใน transaction")
	}
	if blog.ID != created {
		t.Fatal("ไม่ได้คืนตัวที่อ่านกลับมา (author จะหาย)")
	}
}

func TestUpdateBlogRejectsNonOwner(t *testing.T) {
	owner := uuid.New()
	id := uuid.New()

	// ไม่เซ็ต update → ถ้าถูกเรียกจะ panic ซึ่งคือสิ่งที่อยากให้เทสต์จับ
	repo := &mockBlogRepo{
		findByID: func(context.Context, uuid.UUID) (*model.Blog, error) {
			return &model.Blog{ID: id, UserID: owner}, nil
		},
	}

	_, err := NewBlogService(repo, &fakeTx{}).UpdateBlog(
		context.Background(), uuid.New(), id, request.UpdateBlogRequest{Title: "แอบแก้", Content: "hack"},
	)

	if !errors.Is(err, apperr.ErrForbidden) {
		t.Fatalf("อยากได้ ErrForbidden ได้ %v", err)
	}
}

func TestDeleteBlogRejectsNonOwner(t *testing.T) {
	id := uuid.New()

	repo := &mockBlogRepo{
		findByID: func(context.Context, uuid.UUID) (*model.Blog, error) {
			return &model.Blog{ID: id, UserID: uuid.New()}, nil
		},
	}

	err := NewBlogService(repo, &fakeTx{}).DeleteBlog(context.Background(), uuid.New(), id)

	if !errors.Is(err, apperr.ErrForbidden) {
		t.Fatalf("อยากได้ ErrForbidden ได้ %v", err)
	}
}

func TestDeleteBlogByOwner(t *testing.T) {
	owner := uuid.New()
	id := uuid.New()
	deleted := false

	repo := &mockBlogRepo{
		findByID: func(context.Context, uuid.UUID) (*model.Blog, error) {
			return &model.Blog{ID: id, UserID: owner}, nil
		},
		delete: func(context.Context, uuid.UUID) error {
			deleted = true
			return nil
		},
	}

	if err := NewBlogService(repo, &fakeTx{}).DeleteBlog(context.Background(), owner, id); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if !deleted {
		t.Fatal("เจ้าของลบแล้วแต่ repository.Delete ไม่ถูกเรียก")
	}
}

func TestGetBlogByIDPropagatesNotFound(t *testing.T) {
	repo := &mockBlogRepo{
		findByID: func(context.Context, uuid.UUID) (*model.Blog, error) {
			return nil, apperr.NotFound("Blog")
		},
	}

	_, err := NewBlogService(repo, &fakeTx{}).GetBlogByID(context.Background(), uuid.New())

	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("อยากได้ ErrNotFound ได้ %v", err)
	}
}
