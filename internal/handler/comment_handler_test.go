package handler_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/thitiphongD/blog-user-api/internal/apperr"
	"github.com/thitiphongD/blog-user-api/internal/model"
)

func blogFound(app *app, owner uuid.UUID) {
	app.blogs.findByID = func(_ context.Context, id uuid.UUID) (*model.Blog, error) {
		return sampleBlog(id, owner), nil
	}
}

func sampleComment(id, blogID, owner uuid.UUID) *model.Comment {
	return &model.Comment{
		ID:      id,
		Content: "คอมเมนต์แรก",
		BlogID:  blogID,
		UserID:  owner,
		User:    model.User{ID: owner, Name: "Daew", Email: "daew@example.com"},
	}
}

// comment เปิดอ่านได้เหมือน blog — author ต้องไม่มี email เหมือนกัน
func TestListCommentsIsPublicAndHidesAuthorEmail(t *testing.T) {
	app := newApp(t)
	blogID := uuid.New()
	owner := uuid.New()

	blogFound(app, owner)
	app.comments.countByBlog = func(context.Context, uuid.UUID) (int64, error) { return 1, nil }
	app.comments.findAllByBlog = func(context.Context, uuid.UUID, int, int) ([]model.Comment, error) {
		return []model.Comment{*sampleComment(uuid.New(), blogID, owner)}, nil
	}

	rec := app.do(t, http.MethodGet, "/api/v1/blogs/"+blogID.String()+"/comments", "", "")

	body := assertStatus(t, rec, http.StatusOK)

	items, ok := body["data"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("data = %v", body["data"])
	}

	comment, _ := items[0].(map[string]any)
	author, ok := comment["author"].(map[string]any)
	if !ok {
		t.Fatalf("ไม่มี author: %v", comment)
	}
	if _, leaked := author["email"]; leaked {
		t.Fatal("email ของ author หลุดออกทาง comment ที่เป็น public")
	}

	if pagination, ok := body["pagination"].(map[string]any); !ok || pagination["total"] != float64(1) {
		t.Fatalf("pagination = %v", body["pagination"])
	}
}

func TestListCommentsOfMissingBlog(t *testing.T) {
	app := newApp(t)
	app.blogs.findByID = func(context.Context, uuid.UUID) (*model.Blog, error) {
		return nil, apperr.NotFound("Blog")
	}

	rec := app.do(t, http.MethodGet, "/api/v1/blogs/"+uuid.New().String()+"/comments", "", "")

	body := assertStatus(t, rec, http.StatusNotFound)
	if body["message"] != "Blog not found" {
		t.Fatalf("message = %v", body["message"])
	}
}

func TestCommentEndpointsRejectBadInput(t *testing.T) {
	id := uuid.New().String()

	cases := []struct {
		name    string
		method  string
		target  string
		body    string
		token   bool
		status  int
		message string
	}{
		{
			"list blog id ไม่ใช่ UUID", http.MethodGet, "/api/v1/blogs/not-a-uuid/comments",
			"", false, http.StatusBadRequest, "Invalid blog id",
		},
		{
			"list page ไม่ใช่ตัวเลข", http.MethodGet, "/api/v1/blogs/" + id + "/comments?page=abc",
			"", false, http.StatusBadRequest, "Invalid query parameter",
		},
		{
			"create ไม่มี token", http.MethodPost, "/api/v1/blogs/" + id + "/comments",
			`{"content":"x"}`, false, http.StatusUnauthorized, "Unauthorized",
		},
		{
			"create blog id ไม่ใช่ UUID", http.MethodPost, "/api/v1/blogs/not-a-uuid/comments",
			`{"content":"x"}`, true, http.StatusBadRequest, "Invalid blog id",
		},
		{
			"create body พัง", http.MethodPost, "/api/v1/blogs/" + id + "/comments",
			`{oops`, true, http.StatusBadRequest, "Invalid request body",
		},
		{
			"update comment id ไม่ใช่ UUID", http.MethodPut, "/api/v1/comments/not-a-uuid",
			`{"content":"x"}`, true, http.StatusBadRequest, "Invalid comment id",
		},
		{
			"update body พัง", http.MethodPut, "/api/v1/comments/" + id,
			`{oops`, true, http.StatusBadRequest, "Invalid request body",
		},
		{
			"delete comment id ไม่ใช่ UUID", http.MethodDelete, "/api/v1/comments/not-a-uuid",
			"", true, http.StatusBadRequest, "Invalid comment id",
		},
		{
			"delete ไม่มี token", http.MethodDelete, "/api/v1/comments/" + id,
			"", false, http.StatusUnauthorized, "Unauthorized",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := newApp(t)
			// ไม่เซ็ต mock ไว้ ถ้าหลุดไปถึง service/repository จะ panic
			token := ""
			if tc.token {
				token = app.tokenFor(t, uuid.New())
			}

			body := assertStatus(t, app.do(t, tc.method, tc.target, tc.body, token), tc.status)
			if body["message"] != tc.message {
				t.Fatalf("message = %v อยากได้ %q", body["message"], tc.message)
			}
		})
	}
}

func TestCreateCommentValidation(t *testing.T) {
	app := newApp(t)
	blogID := uuid.New()

	rec := app.do(t, http.MethodPost, "/api/v1/blogs/"+blogID.String()+"/comments",
		`{"content":""}`, app.tokenFor(t, uuid.New()))

	body := assertStatus(t, rec, http.StatusUnprocessableEntity)
	if errs, ok := body["errors"].(map[string]any); !ok || errs["content"] == nil {
		t.Fatalf("errors = %v", body["errors"])
	}
}

// ผู้เขียนคอมเมนต์ต้องมาจาก token ไม่ใช่จาก body
func TestCreateCommentUsesUserFromToken(t *testing.T) {
	app := newApp(t)
	blogID := uuid.New()
	author := uuid.New()

	blogFound(app, uuid.New())

	var gotUserID, gotBlogID uuid.UUID
	app.comments.create = func(_ context.Context, c *model.Comment) error {
		gotUserID, gotBlogID = c.UserID, c.BlogID
		c.ID = uuid.New()

		return nil
	}
	app.comments.findByID = func(_ context.Context, id uuid.UUID) (*model.Comment, error) {
		return sampleComment(id, blogID, author), nil
	}

	rec := app.do(t, http.MethodPost, "/api/v1/blogs/"+blogID.String()+"/comments",
		`{"content":"สวัสดี","user_id":"`+uuid.New().String()+`"}`, app.tokenFor(t, author))

	body := assertStatus(t, rec, http.StatusCreated)
	if body["message"] != "Comment created successfully" {
		t.Fatalf("message = %v", body["message"])
	}

	if gotUserID != author {
		t.Fatal("ผู้เขียนคอมเมนต์ไม่ได้มาจาก token")
	}
	if gotBlogID != blogID {
		t.Fatal("ผูกกับ blog ผิดอัน")
	}
}

func TestCreateCommentOnMissingBlog(t *testing.T) {
	app := newApp(t)
	app.blogs.findByID = func(context.Context, uuid.UUID) (*model.Blog, error) {
		return nil, apperr.NotFound("Blog")
	}

	rec := app.do(t, http.MethodPost, "/api/v1/blogs/"+uuid.New().String()+"/comments",
		`{"content":"x"}`, app.tokenFor(t, uuid.New()))

	assertStatus(t, rec, http.StatusNotFound)
}

func TestUpdateCommentByNonOwner(t *testing.T) {
	app := newApp(t)
	id := uuid.New()

	app.comments.findByID = func(_ context.Context, got uuid.UUID) (*model.Comment, error) {
		return sampleComment(got, uuid.New(), uuid.New()), nil
	}
	// ไม่เซ็ต update → ถ้าถูกเรียกจะ panic

	rec := app.do(t, http.MethodPut, "/api/v1/comments/"+id.String(),
		`{"content":"แอบแก้"}`, app.tokenFor(t, uuid.New()))

	body := assertStatus(t, rec, http.StatusForbidden)
	if body["message"] != "Permission denied" {
		t.Fatalf("message = %v", body["message"])
	}
}

func TestUpdateCommentByOwner(t *testing.T) {
	app := newApp(t)
	owner := uuid.New()
	id := uuid.New()
	updated := false

	app.comments.findByID = func(_ context.Context, got uuid.UUID) (*model.Comment, error) {
		return sampleComment(got, uuid.New(), owner), nil
	}
	app.comments.update = func(context.Context, *model.Comment) error {
		updated = true

		return nil
	}

	rec := app.do(t, http.MethodPut, "/api/v1/comments/"+id.String(),
		`{"content":"แก้แล้ว"}`, app.tokenFor(t, owner))

	body := assertStatus(t, rec, http.StatusOK)
	if body["message"] != "Comment updated successfully" {
		t.Fatalf("message = %v", body["message"])
	}
	if !updated {
		t.Fatal("เจ้าของแก้แล้วแต่ repository.Update ไม่ถูกเรียก")
	}
}

func TestDeleteCommentByNonOwner(t *testing.T) {
	app := newApp(t)

	app.comments.findByID = func(_ context.Context, id uuid.UUID) (*model.Comment, error) {
		return sampleComment(id, uuid.New(), uuid.New()), nil
	}
	// ไม่เซ็ต delete → ถ้าถูกเรียกจะ panic

	rec := app.do(t, http.MethodDelete, "/api/v1/comments/"+uuid.New().String(), "",
		app.tokenFor(t, uuid.New()))

	assertStatus(t, rec, http.StatusForbidden)
}

func TestDeleteCommentByOwner(t *testing.T) {
	app := newApp(t)
	owner := uuid.New()
	deleted := false

	app.comments.findByID = func(_ context.Context, id uuid.UUID) (*model.Comment, error) {
		return sampleComment(id, uuid.New(), owner), nil
	}
	app.comments.delete = func(context.Context, uuid.UUID) error {
		deleted = true

		return nil
	}

	rec := app.do(t, http.MethodDelete, "/api/v1/comments/"+uuid.New().String(), "",
		app.tokenFor(t, owner))

	body := assertStatus(t, rec, http.StatusOK)
	if body["message"] != "Comment deleted successfully" {
		t.Fatalf("message = %v", body["message"])
	}
	if !deleted {
		t.Fatal("เจ้าของลบแล้วแต่ repository.Delete ไม่ถูกเรียก")
	}
}

func TestUpdateCommentValidation(t *testing.T) {
	app := newApp(t)

	rec := app.do(t, http.MethodPut, "/api/v1/comments/"+uuid.New().String(),
		`{"content":""}`, app.tokenFor(t, uuid.New()))

	body := assertStatus(t, rec, http.StatusUnprocessableEntity)
	if errs, ok := body["errors"].(map[string]any); !ok || errs["content"] == nil {
		t.Fatalf("errors = %v", body["errors"])
	}
}
