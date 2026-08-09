//go:build integration

package repository_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/thitiphongD/blog-user-api/internal/apperr"
	"github.com/thitiphongD/blog-user-api/internal/model"
	"github.com/thitiphongD/blog-user-api/internal/repository"
)

func newBlog(t *testing.T, owner uuid.UUID, title, content string) *model.Blog {
	t.Helper()

	blog := &model.Blog{Title: title, Content: content, UserID: owner}
	if err := repository.NewBlogRepository(testDB).Create(t.Context(), blog); err != nil {
		t.Fatalf("create blog: %v", err)
	}

	// created_at ปั้นจากเวลาจริง ถ้าสร้างรวดเดียวติดกันจะเรียงลำดับไม่แน่นอน
	time.Sleep(time.Millisecond)

	return blog
}

func allBlogs(t *testing.T, repo *repository.BlogRepository, f model.BlogFilter) []model.Blog {
	t.Helper()

	if f.Limit == 0 {
		f.Limit = 100
	}

	blogs, err := repo.FindAll(t.Context(), f)
	if err != nil {
		t.Fatalf("find all: %v", err)
	}

	return blogs
}

func titles(blogs []model.Blog) []string {
	out := make([]string, 0, len(blogs))
	for _, b := range blogs {
		out = append(out, b.Title)
	}

	return out
}

// author ต้องมากับ blog เสมอ ไม่งั้น response จะไม่มีชื่อคนเขียน (และถ้าไป query ทีละอัน = N+1)
func TestBlogPreloadsAuthor(t *testing.T) {
	reset(t)
	owner := newUser(t, "daew@example.com")
	created := newBlog(t, owner.ID, "Hello", "First post")

	repo := repository.NewBlogRepository(testDB)

	found, err := repo.FindByID(t.Context(), created.ID)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if found.User.Name != "Daew" {
		t.Fatalf("author ไม่ได้ถูก preload: %+v", found.User)
	}

	list := allBlogs(t, repo, model.BlogFilter{})
	if len(list) != 1 || list[0].User.Name != "Daew" {
		t.Fatalf("FindAll ไม่ได้ preload author: %+v", list)
	}
}

func TestBlogFindByIDNotFound(t *testing.T) {
	reset(t)

	_, err := repository.NewBlogRepository(testDB).FindByID(t.Context(), uuid.New())
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("อยากได้ ErrNotFound ได้ %v", err)
	}
}

// ลบแล้วแถวต้องยังอยู่ใน DB แต่ต้องหายจากทุก query
func TestBlogSoftDelete(t *testing.T) {
	reset(t)
	owner := newUser(t, "daew@example.com")
	blog := newBlog(t, owner.ID, "Hello", "First post")

	repo := repository.NewBlogRepository(testDB)

	if err := repo.Delete(t.Context(), blog.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if _, err := repo.FindByID(t.Context(), blog.ID); !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("ลบแล้วยังหาเจอ: %v", err)
	}

	total, err := repo.Count(t.Context(), model.BlogFilter{})
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if total != 0 {
		t.Fatalf("count = %d — นับแถวที่ลบแล้วด้วย pagination จะเพี้ยน", total)
	}

	if got := allBlogs(t, repo, model.BlogFilter{}); len(got) != 0 {
		t.Fatalf("FindAll ยังคืนแถวที่ลบแล้ว: %v", titles(got))
	}

	// แต่แถวต้องยังอยู่จริง ไม่ได้ถูกลบถาวร
	var rows int64
	if err := testDB.Raw("SELECT count(*) FROM blogs WHERE deleted_at IS NOT NULL").Scan(&rows).Error; err != nil {
		t.Fatalf("นับแถวที่ลบ: %v", err)
	}
	if rows != 1 {
		t.Fatalf("แถวที่ลบเหลือ %d — นี่คือ hard delete ไม่ใช่ soft delete", rows)
	}
}

func TestBlogDeleteMissingRow(t *testing.T) {
	reset(t)

	err := repository.NewBlogRepository(testDB).Delete(t.Context(), uuid.New())
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("อยากได้ ErrNotFound ได้ %v", err)
	}
}

func TestBlogUpdateOnlyTouchesContentFields(t *testing.T) {
	reset(t)
	owner := newUser(t, "daew@example.com")
	blog := newBlog(t, owner.ID, "Hello", "First post")

	repo := repository.NewBlogRepository(testDB)

	blog.Title = "แก้แล้ว"
	blog.Content = "edited"
	blog.UserID = uuid.New() // ต้องไม่ถูกเขียนลง DB — เจ้าของเปลี่ยนไม่ได้ผ่าน Update

	if err := repo.Update(t.Context(), blog); err != nil {
		t.Fatalf("update: %v", err)
	}

	found, err := repo.FindByID(t.Context(), blog.ID)
	if err != nil {
		t.Fatalf("find: %v", err)
	}

	if found.Title != "แก้แล้ว" || found.Content != "edited" {
		t.Fatalf("ไม่ได้อัปเดต: %+v", found)
	}
	if found.UserID != owner.ID {
		t.Fatal("เจ้าของถูกเปลี่ยนผ่าน Update ได้ — Updates ต้องรับ map ไม่ใช่ struct")
	}
	if !found.UpdatedAt.After(found.CreatedAt) {
		t.Fatalf("updated_at ไม่ขยับ: created=%v updated=%v", found.CreatedAt, found.UpdatedAt)
	}
}

func TestBlogSearchIsCaseInsensitiveOnTitleAndContent(t *testing.T) {
	reset(t)
	owner := newUser(t, "daew@example.com")
	newBlog(t, owner.ID, "Golang basics", "อธิบายพื้นฐาน")
	newBlog(t, owner.ID, "Rust ก็ดีนะ", "เทียบกับ golang นิดหน่อย")
	newBlog(t, owner.ID, "Postgres tips", "ไม่เกี่ยวกัน")

	repo := repository.NewBlogRepository(testDB)

	got := allBlogs(t, repo, model.BlogFilter{Search: "GOLANG"})
	if len(got) != 2 {
		t.Fatalf("search เจอ %v — อยากได้ 2 อัน (ชนที่ title 1 content 1 และไม่สนตัวพิมพ์)", titles(got))
	}

	total, err := repo.Count(t.Context(), model.BlogFilter{Search: "GOLANG"})
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if total != 2 {
		t.Fatalf("count = %d ไม่ตรงกับจำนวนแถวที่คืน — pagination จะเพี้ยน", total)
	}
}

func TestBlogFilterByUser(t *testing.T) {
	reset(t)
	daew := newUser(t, "daew@example.com")
	somchai := newUser(t, "somchai@example.com")

	newBlog(t, daew.ID, "ของ Daew", "x")
	newBlog(t, somchai.ID, "ของ Somchai", "y")

	repo := repository.NewBlogRepository(testDB)

	got := allBlogs(t, repo, model.BlogFilter{UserID: &somchai.ID})
	if len(got) != 1 || got[0].Title != "ของ Somchai" {
		t.Fatalf("กรองตามผู้เขียนไม่ถูก: %v", titles(got))
	}
}

func TestBlogSort(t *testing.T) {
	reset(t)
	owner := newUser(t, "daew@example.com")
	newBlog(t, owner.ID, "Banana", "x")
	newBlog(t, owner.ID, "Apple", "y")
	newBlog(t, owner.ID, "Cherry", "z")

	repo := repository.NewBlogRepository(testDB)

	cases := []struct {
		name  string
		f     model.BlogFilter
		first string
	}{
		{"created_at desc (default)", model.BlogFilter{}, "Cherry"},
		{"created_at asc", model.BlogFilter{Sort: model.SortCreatedAt, Order: model.OrderAsc}, "Banana"},
		{"title asc", model.BlogFilter{Sort: model.SortTitle, Order: model.OrderAsc}, "Apple"},
		{"title desc", model.BlogFilter{Sort: model.SortTitle, Order: model.OrderDesc}, "Cherry"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := allBlogs(t, repo, tc.f)
			if len(got) == 0 || got[0].Title != tc.first {
				t.Fatalf("เรียงได้ %v — อันแรกควรเป็น %s", titles(got), tc.first)
			}
		})
	}
}

func TestBlogPagination(t *testing.T) {
	reset(t)
	owner := newUser(t, "daew@example.com")
	for _, title := range []string{"หนึ่ง", "สอง", "สาม"} {
		newBlog(t, owner.ID, title, "x")
	}

	repo := repository.NewBlogRepository(testDB)

	got := allBlogs(t, repo, model.BlogFilter{Offset: 1, Limit: 1})
	if len(got) != 1 || got[0].Title != "สอง" {
		t.Fatalf("หน้า 2 ได้ %v อยากได้ [สอง]", titles(got))
	}

	total, err := repo.Count(t.Context(), model.BlogFilter{})
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if total != 3 {
		t.Fatalf("total = %d — Count ต้องไม่สน offset/limit", total)
	}
}

// transaction ต้อง rollback จริง ไม่ใช่แค่ห่อไว้เฉยๆ
func TestTransactionRollsBack(t *testing.T) {
	reset(t)
	owner := newUser(t, "daew@example.com")

	repo := repository.NewBlogRepository(testDB)
	tx := repository.NewTxManager(testDB)

	boom := errors.New("boom")

	err := tx.Do(t.Context(), func(ctx context.Context) error {
		if err := repo.Create(ctx, &model.Blog{Title: "ไม่ควรอยู่รอด", Content: "x", UserID: owner.ID}); err != nil {
			return err
		}

		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("อยากได้ boom ได้ %v", err)
	}

	if got := allBlogs(t, repo, model.BlogFilter{}); len(got) != 0 {
		t.Fatalf("rollback ไม่ทำงาน ยังเหลือ %v", titles(got))
	}
}

// เขียนใน transaction แล้วอ่านกลับใน transaction เดียวกันต้องเห็นของที่เพิ่งเขียน
// นี่คือสิ่งที่ conn(ctx) ต้องทำได้ ถ้าหยิบ connection ผิดตัวจะอ่านไม่เจอ
func TestTransactionReadsOwnWrite(t *testing.T) {
	reset(t)
	owner := newUser(t, "daew@example.com")

	repo := repository.NewBlogRepository(testDB)
	tx := repository.NewTxManager(testDB)

	err := tx.Do(t.Context(), func(ctx context.Context) error {
		blog := &model.Blog{Title: "Hello", Content: "x", UserID: owner.ID}
		if err := repo.Create(ctx, blog); err != nil {
			return err
		}

		found, err := repo.FindByID(ctx, blog.ID)
		if err != nil {
			return err
		}
		if found.User.Name != "Daew" {
			t.Fatalf("อ่านกลับใน tx แล้ว author หาย: %+v", found.User)
		}

		return nil
	})
	if err != nil {
		t.Fatalf("tx: %v", err)
	}

	if got := allBlogs(t, repo, model.BlogFilter{}); len(got) != 1 {
		t.Fatalf("commit แล้วแต่หาไม่เจอ: %v", titles(got))
	}
}
