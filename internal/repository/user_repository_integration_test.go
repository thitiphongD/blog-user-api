//go:build integration

package repository_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/thitiphongD/blog-user-api/internal/apperr"
	"github.com/thitiphongD/blog-user-api/internal/model"
	"github.com/thitiphongD/blog-user-api/internal/repository"
)

func newUser(t *testing.T, email string) *model.User {
	t.Helper()

	user := &model.User{Name: "Daew", Email: email, Password: "hashed"}
	if err := repository.NewUserRepository(testDB).Create(t.Context(), user); err != nil {
		t.Fatalf("create user: %v", err)
	}

	return user
}

func TestUserCreateFillsIDAndTimestamps(t *testing.T) {
	reset(t)

	user := newUser(t, "daew@example.com")

	if user.ID == uuid.Nil {
		t.Fatal("ไม่ได้ id กลับมา")
	}
	if user.CreatedAt.IsZero() || user.UpdatedAt.IsZero() {
		t.Fatalf("timestamp ไม่ถูกเติม: %+v", user)
	}
}

// email ซ้ำต้องได้ ErrEmailTaken ไม่ใช่ error ดิบของ driver
// อันนี้พึ่ง TranslateError: true ใน database.Connect — ลบเมื่อไหร่จะกลายเป็น 500 แทน 409
func TestUserCreateDuplicateEmail(t *testing.T) {
	reset(t)
	newUser(t, "daew@example.com")

	err := repository.NewUserRepository(testDB).Create(t.Context(), &model.User{
		Name: "คนอื่น", Email: "daew@example.com", Password: "hashed",
	})

	if !errors.Is(err, apperr.ErrEmailTaken) {
		t.Fatalf("อยากได้ ErrEmailTaken ได้ %v", err)
	}
}

func TestUserFindByEmail(t *testing.T) {
	reset(t)
	created := newUser(t, "daew@example.com")

	repo := repository.NewUserRepository(testDB)

	found, err := repo.FindByEmail(t.Context(), "daew@example.com")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if found.ID != created.ID {
		t.Fatalf("ได้คนละคน: %v vs %v", found.ID, created.ID)
	}

	if _, err := repo.FindByEmail(t.Context(), "nobody@example.com"); !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("อยากได้ ErrNotFound ได้ %v", err)
	}
}

func TestUserFindByID(t *testing.T) {
	reset(t)
	created := newUser(t, "daew@example.com")

	repo := repository.NewUserRepository(testDB)

	found, err := repo.FindByID(t.Context(), created.ID)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if found.Email != "daew@example.com" {
		t.Fatalf("email = %s", found.Email)
	}

	if _, err := repo.FindByID(t.Context(), uuid.New()); !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("อยากได้ ErrNotFound ได้ %v", err)
	}
}

func TestUserFindAllPagination(t *testing.T) {
	reset(t)
	for _, email := range []string{"a@example.com", "b@example.com", "c@example.com"} {
		newUser(t, email)
	}

	repo := repository.NewUserRepository(testDB)

	total, err := repo.Count(t.Context())
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if total != 3 {
		t.Fatalf("total = %d อยากได้ 3", total)
	}

	page, err := repo.FindAll(t.Context(), 1, 1)
	if err != nil {
		t.Fatalf("find all: %v", err)
	}
	if len(page) != 1 {
		t.Fatalf("ได้ %d แถว อยากได้ 1", len(page))
	}

	// เรียง created_at DESC → offset 1 ต้องเป็นคนที่สมัครเป็นคนที่สอง
	if page[0].Email != "b@example.com" {
		t.Fatalf("ได้ %s — ลำดับหรือ offset เพี้ยน", page[0].Email)
	}
}
