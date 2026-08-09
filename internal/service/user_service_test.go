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

var errDB = errors.New("db พัง")

func listQuery(page, limit int) request.ListQuery {
	q := request.ListQuery{Page: page, Limit: limit}
	q.Normalize()

	return q
}

func TestGetUsersPassesPagination(t *testing.T) {
	var gotOffset, gotLimit int

	repo := &mockUserRepo{
		count: func(context.Context) (int64, error) { return 42, nil },
		findAll: func(_ context.Context, offset, limit int) ([]model.User, error) {
			gotOffset, gotLimit = offset, limit

			return []model.User{{Email: "daew@example.com"}}, nil
		},
	}

	users, total, err := NewUserService(repo).GetUsers(context.Background(), listQuery(3, 10))
	if err != nil {
		t.Fatalf("get users: %v", err)
	}

	if total != 42 {
		t.Fatalf("total = %d อยากได้ 42", total)
	}
	if len(users) != 1 {
		t.Fatalf("ได้ %d คน อยากได้ 1", len(users))
	}
	if gotOffset != 20 || gotLimit != 10 {
		t.Fatalf("offset/limit = %d/%d อยากได้ 20/10", gotOffset, gotLimit)
	}
}

// count พังแล้วต้องเลิกตั้งแต่ตรงนั้น ไม่ต้องไปยิง FindAll ต่อให้เปลือง
func TestGetUsersStopsWhenCountFails(t *testing.T) {
	repo := &mockUserRepo{
		count: func(context.Context) (int64, error) { return 0, errDB },
		findAll: func(context.Context, int, int) ([]model.User, error) {
			t.Error("count พังแล้วยังไปเรียก FindAll ต่อ")

			return nil, errDB
		},
	}

	if _, _, err := NewUserService(repo).GetUsers(context.Background(), listQuery(1, 20)); !errors.Is(err, errDB) {
		t.Fatalf("อยากได้ errDB ได้ %v", err)
	}
}

func TestGetUsersPropagatesFindAllError(t *testing.T) {
	repo := &mockUserRepo{
		count:   func(context.Context) (int64, error) { return 1, nil },
		findAll: func(context.Context, int, int) ([]model.User, error) { return nil, errDB },
	}

	if _, _, err := NewUserService(repo).GetUsers(context.Background(), listQuery(1, 20)); !errors.Is(err, errDB) {
		t.Fatalf("อยากได้ errDB ได้ %v", err)
	}
}

func TestGetUserByID(t *testing.T) {
	id := uuid.New()
	repo := &mockUserRepo{
		findByID: func(_ context.Context, got uuid.UUID) (*model.User, error) {
			return &model.User{ID: got, Email: "daew@example.com"}, nil
		},
	}

	user, err := NewUserService(repo).GetUserByID(context.Background(), id)
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	if user.ID != id {
		t.Fatalf("ได้คนละคน: %v", user.ID)
	}
}

func TestGetUserByIDPropagatesNotFound(t *testing.T) {
	repo := &mockUserRepo{
		findByID: func(context.Context, uuid.UUID) (*model.User, error) {
			return nil, apperr.NotFound("User")
		},
	}

	if _, err := NewUserService(repo).GetUserByID(context.Background(), uuid.New()); !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("อยากได้ ErrNotFound ได้ %v", err)
	}
}
