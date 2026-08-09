package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/thitiphongD/blog-user-api/internal/model"
)

// mock เขียนมือด้วย func field — ไม่ต้องลง mock generator เพิ่ม
// method ไหนไม่ได้เซ็ตแล้วถูกเรียก = panic ซึ่งคือสิ่งที่อยากรู้อยู่แล้ว
// (เช่น ownership check ต้องกัน Update ไม่ให้ถูกเรียกเลย)

type mockUserRepo struct {
	findByEmail func(ctx context.Context, email string) (*model.User, error)
	findByID    func(ctx context.Context, id uuid.UUID) (*model.User, error)
	findAll     func(ctx context.Context, offset, limit int) ([]model.User, error)
	count       func(ctx context.Context) (int64, error)
	create      func(ctx context.Context, user *model.User) error
}

func (m *mockUserRepo) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	if m.findByEmail == nil {
		panic("unexpected call: FindByEmail")
	}
	return m.findByEmail(ctx, email)
}

func (m *mockUserRepo) FindByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	if m.findByID == nil {
		panic("unexpected call: FindByID")
	}
	return m.findByID(ctx, id)
}

func (m *mockUserRepo) FindAll(ctx context.Context, offset, limit int) ([]model.User, error) {
	if m.findAll == nil {
		panic("unexpected call: FindAll")
	}
	return m.findAll(ctx, offset, limit)
}

func (m *mockUserRepo) Count(ctx context.Context) (int64, error) {
	if m.count == nil {
		panic("unexpected call: Count")
	}
	return m.count(ctx)
}

func (m *mockUserRepo) Create(ctx context.Context, user *model.User) error {
	if m.create == nil {
		panic("unexpected call: Create")
	}
	return m.create(ctx, user)
}

type mockBlogRepo struct {
	findByID func(ctx context.Context, id uuid.UUID) (*model.Blog, error)
	findAll  func(ctx context.Context, f model.BlogFilter) ([]model.Blog, error)
	count    func(ctx context.Context, f model.BlogFilter) (int64, error)
	create   func(ctx context.Context, blog *model.Blog) error
	update   func(ctx context.Context, blog *model.Blog) error
	delete   func(ctx context.Context, id uuid.UUID) error
}

func (m *mockBlogRepo) FindByID(ctx context.Context, id uuid.UUID) (*model.Blog, error) {
	if m.findByID == nil {
		panic("unexpected call: FindByID")
	}
	return m.findByID(ctx, id)
}

func (m *mockBlogRepo) FindAll(ctx context.Context, f model.BlogFilter) ([]model.Blog, error) {
	if m.findAll == nil {
		panic("unexpected call: FindAll")
	}
	return m.findAll(ctx, f)
}

func (m *mockBlogRepo) Count(ctx context.Context, f model.BlogFilter) (int64, error) {
	if m.count == nil {
		panic("unexpected call: Count")
	}
	return m.count(ctx, f)
}

func (m *mockBlogRepo) Create(ctx context.Context, blog *model.Blog) error {
	if m.create == nil {
		panic("unexpected call: Create")
	}
	return m.create(ctx, blog)
}

func (m *mockBlogRepo) Update(ctx context.Context, blog *model.Blog) error {
	if m.update == nil {
		panic("unexpected call: Update")
	}
	return m.update(ctx, blog)
}

func (m *mockBlogRepo) Delete(ctx context.Context, id uuid.UUID) error {
	if m.delete == nil {
		panic("unexpected call: Delete")
	}
	return m.delete(ctx, id)
}

// fakeTx รัน fn ตรงๆ ไม่มี transaction จริง — service แค่ต้องเรียกให้ถูก
type fakeTx struct {
	called bool
}

func (t *fakeTx) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	t.called = true
	return fn(ctx)
}
