package handler_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/thitiphongD/blog-user-api/internal/apperr"
	"github.com/thitiphongD/blog-user-api/internal/model"
)

func TestListUsersRequiresToken(t *testing.T) {
	app := newApp(t)

	rec := app.do(t, http.MethodGet, "/api/v1/users", "", "")

	assertStatus(t, rec, http.StatusUnauthorized)
}

func TestListUsers(t *testing.T) {
	app := newApp(t)

	var gotOffset, gotLimit int
	app.users.count = func(context.Context) (int64, error) { return 3, nil }
	app.users.findAll = func(_ context.Context, offset, limit int) ([]model.User, error) {
		gotOffset, gotLimit = offset, limit

		return []model.User{{ID: uuid.New(), Name: "Daew", Email: "daew@example.com", Password: "hashed"}}, nil
	}

	rec := app.do(t, http.MethodGet, "/api/v1/users?page=2&limit=10", "", app.tokenFor(t, uuid.New()))

	body := assertStatus(t, rec, http.StatusOK)

	if gotOffset != 10 || gotLimit != 10 {
		t.Fatalf("offset/limit = %d/%d อยากได้ 10/10", gotOffset, gotLimit)
	}

	items, ok := body["data"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("data = %v", body["data"])
	}

	user, _ := items[0].(map[string]any)
	if _, leaked := user["password"]; leaked {
		t.Fatal("password hash หลุดออก response")
	}

	pagination, ok := body["pagination"].(map[string]any)
	if !ok || pagination["total"] != float64(3) || pagination["total_page"] != float64(1) {
		t.Fatalf("pagination = %v", body["pagination"])
	}
}

func TestListUsersClampsLimit(t *testing.T) {
	app := newApp(t)

	var gotLimit int
	app.users.count = func(context.Context) (int64, error) { return 0, nil }
	app.users.findAll = func(_ context.Context, _, limit int) ([]model.User, error) {
		gotLimit = limit

		return nil, nil
	}

	rec := app.do(t, http.MethodGet, "/api/v1/users?limit=999999", "", app.tokenFor(t, uuid.New()))

	assertStatus(t, rec, http.StatusOK)

	if gotLimit != 100 {
		t.Fatalf("limit ที่ส่งถึง repository = %d อยากได้ 100", gotLimit)
	}
}

// page ที่ไม่ใช่ตัวเลข = bind ไม่ผ่าน = 400 ไม่ใช่เงียบๆ ใช้ default
func TestListUsersRejectsNonNumericPage(t *testing.T) {
	app := newApp(t)

	rec := app.do(t, http.MethodGet, "/api/v1/users?page=abc", "", app.tokenFor(t, uuid.New()))

	body := assertStatus(t, rec, http.StatusBadRequest)
	if body["message"] != "Invalid query parameter" {
		t.Fatalf("message = %v", body["message"])
	}
}

func TestGetUser(t *testing.T) {
	app := newApp(t)
	id := uuid.New()

	app.users.findByID = func(_ context.Context, got uuid.UUID) (*model.User, error) {
		return &model.User{ID: got, Name: "Daew", Email: "daew@example.com"}, nil
	}

	rec := app.do(t, http.MethodGet, "/api/v1/users/"+id.String(), "", app.tokenFor(t, uuid.New()))

	body := assertStatus(t, rec, http.StatusOK)

	data := dataOf(t, body)
	if data["id"] != id.String() {
		t.Fatalf("id = %v อยากได้ %v", data["id"], id)
	}
}

func TestGetUserInvalidUUID(t *testing.T) {
	app := newApp(t)

	rec := app.do(t, http.MethodGet, "/api/v1/users/not-a-uuid", "", app.tokenFor(t, uuid.New()))

	body := assertStatus(t, rec, http.StatusBadRequest)
	if body["message"] != "Invalid user id" {
		t.Fatalf("message = %v", body["message"])
	}
}

func TestGetUserNotFound(t *testing.T) {
	app := newApp(t)
	app.users.findByID = func(context.Context, uuid.UUID) (*model.User, error) {
		return nil, apperr.NotFound("User")
	}

	rec := app.do(t, http.MethodGet, "/api/v1/users/"+uuid.New().String(), "", app.tokenFor(t, uuid.New()))

	body := assertStatus(t, rec, http.StatusNotFound)
	if body["message"] != "User not found" {
		t.Fatalf("message = %v", body["message"])
	}
}

// repository พังต้องเป็น 500 ที่ไม่คายรายละเอียด ไม่ใช่ 200 พร้อม list ว่าง
func TestListUsersRepositoryError(t *testing.T) {
	app := newApp(t)
	app.users.count = func(context.Context) (int64, error) {
		return 0, errors.New("dial tcp: connect: connection refused")
	}

	rec := app.do(t, http.MethodGet, "/api/v1/users", "", app.tokenFor(t, uuid.New()))

	body := assertStatus(t, rec, http.StatusInternalServerError)
	if body["message"] != "Internal server error" {
		t.Fatalf("message = %v", body["message"])
	}
	if strings.Contains(rec.Body.String(), "connection refused") {
		t.Fatalf("error จริงหลุดออก response: %s", rec.Body.String())
	}
}
