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

var errBoom = errors.New("boom: connection reset by peer")

func sampleBlog(id, owner uuid.UUID) *model.Blog {
	return &model.Blog{
		ID:      id,
		Title:   "Hello",
		Content: "First post",
		UserID:  owner,
		User:    model.User{ID: owner, Name: "Daew", Email: "daew@example.com"},
	}
}

// /blogs เป็น public แต่ /users ปิด → author ห้ามมี email ไม่งั้นรั่วออกทางหลัง
func TestListBlogsIsPublicAndHidesAuthorEmail(t *testing.T) {
	app := newApp(t)
	owner := uuid.New()

	app.blogs.count = func(context.Context, model.BlogFilter) (int64, error) { return 1, nil }
	app.blogs.findAll = func(context.Context, model.BlogFilter) ([]model.Blog, error) {
		return []model.Blog{*sampleBlog(uuid.New(), owner)}, nil
	}

	rec := app.do(t, http.MethodGet, "/api/v1/blogs", "", "")

	body := assertStatus(t, rec, http.StatusOK)

	items, ok := body["data"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("data = %v", body["data"])
	}

	blog, _ := items[0].(map[string]any)
	author, ok := blog["author"].(map[string]any)
	if !ok {
		t.Fatalf("ไม่มี author: %v", blog)
	}
	if _, leaked := author["email"]; leaked {
		t.Fatal("email ของ author หลุดออกทาง /blogs ที่เป็น public")
	}

	pagination, ok := body["pagination"].(map[string]any)
	if !ok || pagination["total"] != float64(1) {
		t.Fatalf("pagination = %v", body["pagination"])
	}
}

// limit เกิน max ต้องถูก clamp ที่ระดับ query ไม่ใช่ยิงเต็มตารางแล้วค่อยว่ากัน
func TestListBlogsClampsLimit(t *testing.T) {
	app := newApp(t)

	var gotFilter model.BlogFilter
	app.blogs.count = func(context.Context, model.BlogFilter) (int64, error) { return 0, nil }
	app.blogs.findAll = func(_ context.Context, f model.BlogFilter) ([]model.Blog, error) {
		gotFilter = f
		return nil, nil
	}

	rec := app.do(t, http.MethodGet, "/api/v1/blogs?limit=999999", "", "")

	body := assertStatus(t, rec, http.StatusOK)

	if gotFilter.Limit != 100 {
		t.Fatalf("limit ที่ส่งถึง repository = %d อยากได้ 100", gotFilter.Limit)
	}

	pagination, _ := body["pagination"].(map[string]any)
	if pagination["limit"] != float64(100) {
		t.Fatalf("limit ใน response = %v", pagination["limit"])
	}
}

func TestListBlogsRejectsBadQuery(t *testing.T) {
	cases := []struct {
		name  string
		query string
	}{
		{"sort นอก whitelist", "?sort=password"},
		{"sort แถม SQL", "?sort=title%27%20OR%20%271%27%3D%271"},
		{"order นอก whitelist", "?order=sideways"},
		{"user_id ไม่ใช่ UUID", "?user_id=hack"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := newApp(t)
			// ไม่เซ็ต count/findAll ไว้เลย ถ้าหลุดไปถึง repository จะ panic
			rec := app.do(t, http.MethodGet, "/api/v1/blogs"+tc.query, "", "")

			assertStatus(t, rec, http.StatusBadRequest)
		})
	}
}

func TestGetBlogInvalidUUID(t *testing.T) {
	app := newApp(t)

	rec := app.do(t, http.MethodGet, "/api/v1/blogs/not-a-uuid", "", "")

	body := assertStatus(t, rec, http.StatusBadRequest)
	if body["message"] != "Invalid blog id" {
		t.Fatalf("message = %v", body["message"])
	}
}

func TestGetBlogNotFound(t *testing.T) {
	app := newApp(t)
	app.blogs.findByID = func(context.Context, uuid.UUID) (*model.Blog, error) {
		return nil, apperr.NotFound("Blog")
	}

	rec := app.do(t, http.MethodGet, "/api/v1/blogs/"+uuid.New().String(), "", "")

	body := assertStatus(t, rec, http.StatusNotFound)
	if body["message"] != "Blog not found" {
		t.Fatalf("message = %v", body["message"])
	}
}

func TestCreateBlogRequiresToken(t *testing.T) {
	app := newApp(t)

	rec := app.do(t, http.MethodPost, "/api/v1/blogs", `{"title":"Hello","content":"x"}`, "")

	assertStatus(t, rec, http.StatusUnauthorized)
}

// เจ้าของโพสต์ต้องมาจาก token ไม่ใช่จาก body ที่ client ส่งมา
func TestCreateBlogUsesUserFromToken(t *testing.T) {
	app := newApp(t)
	owner := uuid.New()
	id := uuid.New()

	var gotUserID uuid.UUID
	app.blogs.create = func(_ context.Context, b *model.Blog) error {
		gotUserID = b.UserID
		b.ID = id
		return nil
	}
	app.blogs.findByID = func(_ context.Context, got uuid.UUID) (*model.Blog, error) {
		return sampleBlog(got, owner), nil
	}

	rec := app.do(t, http.MethodPost, "/api/v1/blogs",
		`{"title":"Hello","content":"First post","user_id":"`+uuid.New().String()+`"}`,
		app.tokenFor(t, owner))

	assertStatus(t, rec, http.StatusCreated)

	if gotUserID != owner {
		t.Fatal("เจ้าของโพสต์ไม่ได้มาจาก token — client ยัด user_id มาใน body แล้วมีผล")
	}
}

func TestCreateBlogValidationFails(t *testing.T) {
	app := newApp(t)

	rec := app.do(t, http.MethodPost, "/api/v1/blogs", `{"title":"","content":""}`,
		app.tokenFor(t, uuid.New()))

	body := assertStatus(t, rec, http.StatusUnprocessableEntity)

	errs, ok := body["errors"].(map[string]any)
	if !ok || errs["title"] == nil || errs["content"] == nil {
		t.Fatalf("errors = %v", body["errors"])
	}
}

// ไม่ใช่เจ้าของต้องได้ 403 ไม่ใช่ 404 — blog มีอยู่จริง แค่ไม่มีสิทธิ์
func TestUpdateBlogByNonOwner(t *testing.T) {
	app := newApp(t)
	id := uuid.New()

	app.blogs.findByID = func(context.Context, uuid.UUID) (*model.Blog, error) {
		return sampleBlog(id, uuid.New()), nil
	}
	// ไม่เซ็ต update ไว้ ถ้าถูกเรียกจะ panic

	rec := app.do(t, http.MethodPut, "/api/v1/blogs/"+id.String(),
		`{"title":"แอบแก้","content":"hack"}`, app.tokenFor(t, uuid.New()))

	body := assertStatus(t, rec, http.StatusForbidden)
	if body["message"] != "Permission denied" {
		t.Fatalf("message = %v", body["message"])
	}
}

func TestUpdateBlogByOwner(t *testing.T) {
	app := newApp(t)
	owner := uuid.New()
	id := uuid.New()
	updated := false

	app.blogs.findByID = func(context.Context, uuid.UUID) (*model.Blog, error) {
		return sampleBlog(id, owner), nil
	}
	app.blogs.update = func(context.Context, *model.Blog) error {
		updated = true
		return nil
	}

	rec := app.do(t, http.MethodPut, "/api/v1/blogs/"+id.String(),
		`{"title":"Hello แก้แล้ว","content":"edited"}`, app.tokenFor(t, owner))

	body := assertStatus(t, rec, http.StatusOK)
	if body["message"] != "Blog updated successfully" {
		t.Fatalf("message = %v", body["message"])
	}
	if !updated {
		t.Fatal("เจ้าของแก้แล้วแต่ repository.Update ไม่ถูกเรียก")
	}
}

// PUT คือ replace เต็ม ส่งมาไม่ครบต้องไม่ผ่าน
func TestUpdateBlogRequiresAllFields(t *testing.T) {
	app := newApp(t)

	rec := app.do(t, http.MethodPut, "/api/v1/blogs/"+uuid.New().String(),
		`{"title":"มีแต่ title"}`, app.tokenFor(t, uuid.New()))

	assertStatus(t, rec, http.StatusUnprocessableEntity)
}

func TestDeleteBlogByNonOwner(t *testing.T) {
	app := newApp(t)
	id := uuid.New()

	app.blogs.findByID = func(context.Context, uuid.UUID) (*model.Blog, error) {
		return sampleBlog(id, uuid.New()), nil
	}
	// ไม่เซ็ต delete ไว้ ถ้าถูกเรียกจะ panic

	rec := app.do(t, http.MethodDelete, "/api/v1/blogs/"+id.String(), "", app.tokenFor(t, uuid.New()))

	assertStatus(t, rec, http.StatusForbidden)
}

// ลบแล้วยังต้องอยู่ใน envelope เดิม ไม่ใช่ 204 ที่ไม่มี body
func TestDeleteBlogByOwnerKeepsEnvelope(t *testing.T) {
	app := newApp(t)
	owner := uuid.New()
	id := uuid.New()
	deleted := false

	app.blogs.findByID = func(context.Context, uuid.UUID) (*model.Blog, error) {
		return sampleBlog(id, owner), nil
	}
	app.blogs.delete = func(context.Context, uuid.UUID) error {
		deleted = true
		return nil
	}

	rec := app.do(t, http.MethodDelete, "/api/v1/blogs/"+id.String(), "", app.tokenFor(t, owner))

	body := assertStatus(t, rec, http.StatusOK)
	if body["message"] != "Blog deleted successfully" {
		t.Fatalf("message = %v", body["message"])
	}
	if !deleted {
		t.Fatal("เจ้าของลบแล้วแต่ repository.Delete ไม่ถูกเรียก")
	}
}

// error ที่ไม่ได้ตั้งใจต้องกลายเป็น 500 ที่ไม่คายรายละเอียดออกไป
func TestUnexpectedRepositoryErrorBecomes500(t *testing.T) {
	app := newApp(t)
	app.blogs.count = func(context.Context, model.BlogFilter) (int64, error) {
		return 0, errBoom
	}

	rec := app.do(t, http.MethodGet, "/api/v1/blogs", "", "")

	body := assertStatus(t, rec, http.StatusInternalServerError)
	if body["message"] != "Internal server error" {
		t.Fatalf("message = %v", body["message"])
	}
	if strings.Contains(rec.Body.String(), "boom") {
		t.Fatalf("error จริงหลุดออก response: %s", rec.Body.String())
	}
}

func TestBlogWriteEndpointsRejectBadInput(t *testing.T) {
	cases := []struct {
		name    string
		method  string
		target  string
		body    string
		status  int
		message string
	}{
		{"create body พัง", http.MethodPost, "/api/v1/blogs", `{oops`, http.StatusBadRequest, "Invalid request body"},
		{
			"update id ไม่ใช่ UUID", http.MethodPut, "/api/v1/blogs/not-a-uuid",
			`{"title":"x","content":"y"}`, http.StatusBadRequest, "Invalid blog id",
		},
		{
			"update body พัง", http.MethodPut, "/api/v1/blogs/" + uuid.New().String(),
			`{oops`, http.StatusBadRequest, "Invalid request body",
		},
		{
			"delete id ไม่ใช่ UUID", http.MethodDelete, "/api/v1/blogs/not-a-uuid",
			"", http.StatusBadRequest, "Invalid blog id",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := newApp(t)
			// ไม่เซ็ต mock ไว้เลย ถ้าหลุดไปถึง repository จะ panic
			rec := app.do(t, tc.method, tc.target, tc.body, app.tokenFor(t, uuid.New()))

			body := assertStatus(t, rec, tc.status)
			if body["message"] != tc.message {
				t.Fatalf("message = %v อยากได้ %q", body["message"], tc.message)
			}
		})
	}
}

// page ที่ไม่ใช่ตัวเลข = bind ไม่ผ่าน = 400
func TestListBlogsRejectsNonNumericPage(t *testing.T) {
	app := newApp(t)

	rec := app.do(t, http.MethodGet, "/api/v1/blogs?page=abc", "", "")

	body := assertStatus(t, rec, http.StatusBadRequest)
	if body["message"] != "Invalid query parameter" {
		t.Fatalf("message = %v", body["message"])
	}
}

func TestUpdateAndDeleteRequireToken(t *testing.T) {
	app := newApp(t)
	id := uuid.New().String()

	rec := app.do(t, http.MethodPut, "/api/v1/blogs/"+id, `{"title":"x","content":"y"}`, "")
	assertStatus(t, rec, http.StatusUnauthorized)

	rec = app.do(t, http.MethodDelete, "/api/v1/blogs/"+id, "", "")
	assertStatus(t, rec, http.StatusUnauthorized)
}

func TestGetBlogRepositoryError(t *testing.T) {
	app := newApp(t)
	app.blogs.findByID = func(context.Context, uuid.UUID) (*model.Blog, error) { return nil, errBoom }

	rec := app.do(t, http.MethodGet, "/api/v1/blogs/"+uuid.New().String(), "", "")

	assertStatus(t, rec, http.StatusInternalServerError)
	if strings.Contains(rec.Body.String(), "boom") {
		t.Fatalf("error จริงหลุดออก response: %s", rec.Body.String())
	}
}

func TestCreateBlogRepositoryError(t *testing.T) {
	app := newApp(t)
	app.blogs.create = func(context.Context, *model.Blog) error { return errBoom }

	rec := app.do(t, http.MethodPost, "/api/v1/blogs", `{"title":"Hello","content":"x"}`,
		app.tokenFor(t, uuid.New()))

	assertStatus(t, rec, http.StatusInternalServerError)
	if strings.Contains(rec.Body.String(), "boom") {
		t.Fatalf("error จริงหลุดออก response: %s", rec.Body.String())
	}
}

func TestGetBlogSuccess(t *testing.T) {
	app := newApp(t)
	id := uuid.New()
	owner := uuid.New()

	app.blogs.findByID = func(_ context.Context, got uuid.UUID) (*model.Blog, error) {
		return sampleBlog(got, owner), nil
	}

	rec := app.do(t, http.MethodGet, "/api/v1/blogs/"+id.String(), "", "")

	body := assertStatus(t, rec, http.StatusOK)
	data := dataOf(t, body)

	if data["id"] != id.String() || data["title"] != "Hello" {
		t.Fatalf("data = %v", data)
	}

	author, ok := data["author"].(map[string]any)
	if !ok || author["name"] != "Daew" {
		t.Fatalf("author = %v", data["author"])
	}
	if _, leaked := author["email"]; leaked {
		t.Fatal("email ของ author หลุดออกทาง /blogs/:id ที่เป็น public ด้วย")
	}
}
