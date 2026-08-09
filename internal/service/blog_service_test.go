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

var errRepo = errors.New("repository พัง")

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

func TestUpdateBlogByOwner(t *testing.T) {
	owner := uuid.New()
	id := uuid.New()
	tx := &fakeTx{}

	var sent *model.Blog
	stored := &model.Blog{ID: id, UserID: owner, Title: "Hello", Content: "First post"}

	repo := &mockBlogRepo{
		findByID: func(context.Context, uuid.UUID) (*model.Blog, error) { return stored, nil },
		update: func(_ context.Context, b *model.Blog) error {
			sent = b
			return nil
		},
	}

	got, err := NewBlogService(repo, tx).UpdateBlog(context.Background(), owner, id, request.UpdateBlogRequest{
		Title: "แก้แล้ว", Content: "edited",
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	if !tx.called {
		t.Fatal("update + อ่านกลับ ไม่ได้อยู่ใน transaction")
	}
	if sent == nil || sent.Title != "แก้แล้ว" || sent.Content != "edited" {
		t.Fatalf("ค่าที่ส่งให้ repository ไม่ถูก: %+v", sent)
	}
	if sent.UserID != owner {
		t.Fatal("เจ้าของถูกเปลี่ยนระหว่างทาง")
	}
	if got.ID != id {
		t.Fatal("ไม่ได้คืนตัวที่อ่านกลับมา")
	}
}

func TestUpdateBlogPropagatesNotFound(t *testing.T) {
	repo := &mockBlogRepo{
		findByID: func(context.Context, uuid.UUID) (*model.Blog, error) {
			return nil, apperr.NotFound("Blog")
		},
	}

	_, err := NewBlogService(repo, &fakeTx{}).UpdateBlog(
		context.Background(), uuid.New(), uuid.New(), request.UpdateBlogRequest{Title: "x", Content: "y"},
	)
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("อยากได้ ErrNotFound ได้ %v", err)
	}
}

// repository พังกลาง transaction ต้องเด้ง error ออกมา ไม่ใช่คืน blog ที่ยังไม่ถูกเขียน
func TestUpdateBlogPropagatesUpdateError(t *testing.T) {
	owner := uuid.New()

	repo := &mockBlogRepo{
		findByID: func(_ context.Context, id uuid.UUID) (*model.Blog, error) {
			return &model.Blog{ID: id, UserID: owner}, nil
		},
		update: func(context.Context, *model.Blog) error { return errRepo },
	}

	got, err := NewBlogService(repo, &fakeTx{}).UpdateBlog(
		context.Background(), owner, uuid.New(), request.UpdateBlogRequest{Title: "x", Content: "y"},
	)
	if !errors.Is(err, errRepo) {
		t.Fatalf("อยากได้ errRepo ได้ %v", err)
	}
	if got != nil {
		t.Fatalf("พังแล้วยังคืน blog มา: %+v", got)
	}
}

func TestCreateBlogPropagatesCreateError(t *testing.T) {
	repo := &mockBlogRepo{
		create: func(context.Context, *model.Blog) error { return errRepo },
		findByID: func(context.Context, uuid.UUID) (*model.Blog, error) {
			t.Error("create พังแล้วยังไปอ่านกลับต่อ")

			return nil, errRepo
		},
	}

	_, err := NewBlogService(repo, &fakeTx{}).CreateBlog(context.Background(), uuid.New(), request.CreateBlogRequest{
		Title: "Hello", Content: "x",
	})
	if !errors.Is(err, errRepo) {
		t.Fatalf("อยากได้ errRepo ได้ %v", err)
	}
}

func TestGetBlogsStopsWhenCountFails(t *testing.T) {
	repo := &mockBlogRepo{
		count: func(context.Context, model.BlogFilter) (int64, error) { return 0, errRepo },
		findAll: func(context.Context, model.BlogFilter) ([]model.Blog, error) {
			t.Error("count พังแล้วยังไปเรียก FindAll ต่อ")

			return nil, errRepo
		},
	}

	if _, _, err := NewBlogService(repo, &fakeTx{}).GetBlogs(context.Background(), blogQuery(1, 20)); !errors.Is(err, errRepo) {
		t.Fatalf("อยากได้ errRepo ได้ %v", err)
	}
}

func TestGetBlogsPropagatesFindAllError(t *testing.T) {
	repo := &mockBlogRepo{
		count:   func(context.Context, model.BlogFilter) (int64, error) { return 1, nil },
		findAll: func(context.Context, model.BlogFilter) ([]model.Blog, error) { return nil, errRepo },
	}

	if _, _, err := NewBlogService(repo, &fakeTx{}).GetBlogs(context.Background(), blogQuery(1, 20)); !errors.Is(err, errRepo) {
		t.Fatalf("อยากได้ errRepo ได้ %v", err)
	}
}

func TestDeleteBlogPropagatesNotFound(t *testing.T) {
	repo := &mockBlogRepo{
		findByID: func(context.Context, uuid.UUID) (*model.Blog, error) {
			return nil, apperr.NotFound("Blog")
		},
	}

	err := NewBlogService(repo, &fakeTx{}).DeleteBlog(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("อยากได้ ErrNotFound ได้ %v", err)
	}
}

// สร้างสำเร็จแต่อ่านกลับพัง ต้องไม่คืน blog ที่ไม่มี author ออกไปเงียบๆ
func TestCreateBlogPropagatesRefetchError(t *testing.T) {
	repo := &mockBlogRepo{
		create: func(_ context.Context, b *model.Blog) error {
			b.ID = uuid.New()
			return nil
		},
		findByID: func(context.Context, uuid.UUID) (*model.Blog, error) { return nil, errRepo },
	}

	got, err := NewBlogService(repo, &fakeTx{}).CreateBlog(context.Background(), uuid.New(), request.CreateBlogRequest{
		Title: "Hello", Content: "x",
	})
	if !errors.Is(err, errRepo) {
		t.Fatalf("อยากได้ errRepo ได้ %v", err)
	}
	if got != nil {
		t.Fatalf("อ่านกลับพังแล้วยังคืน blog: %+v", got)
	}
}
