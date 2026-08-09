//go:build integration

package repository_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/thitiphongD/blog-user-api/internal/apperr"
	"github.com/thitiphongD/blog-user-api/internal/model"
	"github.com/thitiphongD/blog-user-api/internal/repository"
)

func newComment(t *testing.T, blogID, userID uuid.UUID, content string) *model.Comment {
	t.Helper()

	comment := &model.Comment{Content: content, BlogID: blogID, UserID: userID}
	if err := repository.NewCommentRepository(testDB).Create(t.Context(), comment); err != nil {
		t.Fatalf("create comment: %v", err)
	}

	time.Sleep(time.Millisecond)

	return comment
}

func contents(comments []model.Comment) []string {
	out := make([]string, 0, len(comments))
	for _, c := range comments {
		out = append(out, c.Content)
	}

	return out
}

// comment เรียงเก่าไปใหม่ ต่างจาก blog — บทสนทนาต้องอ่านไล่จากบนลงล่าง
func TestCommentsAreOldestFirstAndPreloadAuthor(t *testing.T) {
	reset(t)
	user := newUser(t, "daew@example.com")
	blog := newBlog(t, user.ID, "Hello", "x")

	newComment(t, blog.ID, user.ID, "หนึ่ง")
	newComment(t, blog.ID, user.ID, "สอง")
	newComment(t, blog.ID, user.ID, "สาม")

	repo := repository.NewCommentRepository(testDB)

	got, err := repo.FindAllByBlog(t.Context(), blog.ID, 0, 100)
	if err != nil {
		t.Fatalf("find: %v", err)
	}

	if want := []string{"หนึ่ง", "สอง", "สาม"}; !equal(contents(got), want) {
		t.Fatalf("เรียงได้ %v อยากได้ %v", contents(got), want)
	}
	if got[0].User.Name != "Daew" {
		t.Fatalf("ไม่ได้ preload author: %+v", got[0].User)
	}
}

// comment ของ blog อื่นต้องไม่ปน
func TestCommentsAreScopedToBlog(t *testing.T) {
	reset(t)
	user := newUser(t, "daew@example.com")
	first := newBlog(t, user.ID, "อันแรก", "x")
	second := newBlog(t, user.ID, "อันสอง", "y")

	newComment(t, first.ID, user.ID, "ของอันแรก")
	newComment(t, second.ID, user.ID, "ของอันสอง")

	repo := repository.NewCommentRepository(testDB)

	got, err := repo.FindAllByBlog(t.Context(), second.ID, 0, 100)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if len(got) != 1 || got[0].Content != "ของอันสอง" {
		t.Fatalf("ได้ %v", contents(got))
	}

	total, err := repo.CountByBlog(t.Context(), second.ID)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if total != 1 {
		t.Fatalf("count = %d อยากได้ 1", total)
	}
}

func TestCommentPagination(t *testing.T) {
	reset(t)
	user := newUser(t, "daew@example.com")
	blog := newBlog(t, user.ID, "Hello", "x")

	for _, content := range []string{"หนึ่ง", "สอง", "สาม"} {
		newComment(t, blog.ID, user.ID, content)
	}

	repo := repository.NewCommentRepository(testDB)

	got, err := repo.FindAllByBlog(t.Context(), blog.ID, 1, 1)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if len(got) != 1 || got[0].Content != "สอง" {
		t.Fatalf("หน้า 2 ได้ %v", contents(got))
	}
}

func TestCommentSoftDelete(t *testing.T) {
	reset(t)
	user := newUser(t, "daew@example.com")
	blog := newBlog(t, user.ID, "Hello", "x")
	comment := newComment(t, blog.ID, user.ID, "เดี๋ยวลบ")

	repo := repository.NewCommentRepository(testDB)

	if err := repo.Delete(t.Context(), comment.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if _, err := repo.FindByID(t.Context(), comment.ID); !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("ลบแล้วยังหาเจอ: %v", err)
	}

	total, err := repo.CountByBlog(t.Context(), blog.ID)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if total != 0 {
		t.Fatalf("count = %d — นับของที่ลบแล้วด้วย", total)
	}

	var rows int64
	if err := testDB.Raw("SELECT count(*) FROM comments WHERE deleted_at IS NOT NULL").Scan(&rows).Error; err != nil {
		t.Fatalf("นับแถวที่ลบ: %v", err)
	}
	if rows != 1 {
		t.Fatalf("แถวที่ลบเหลือ %d — นี่คือ hard delete", rows)
	}
}

func TestCommentDeleteMissingRow(t *testing.T) {
	reset(t)

	err := repository.NewCommentRepository(testDB).Delete(t.Context(), uuid.New())
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("อยากได้ ErrNotFound ได้ %v", err)
	}
}

func TestCommentUpdateOnlyTouchesContent(t *testing.T) {
	reset(t)
	user := newUser(t, "daew@example.com")
	blog := newBlog(t, user.ID, "Hello", "x")
	comment := newComment(t, blog.ID, user.ID, "ของเดิม")

	repo := repository.NewCommentRepository(testDB)

	comment.Content = "แก้แล้ว"
	comment.UserID = uuid.New() // ต้องไม่ถูกเขียนลง DB
	comment.BlogID = uuid.New()

	if err := repo.Update(t.Context(), comment); err != nil {
		t.Fatalf("update: %v", err)
	}

	found, err := repo.FindByID(t.Context(), comment.ID)
	if err != nil {
		t.Fatalf("find: %v", err)
	}

	if found.Content != "แก้แล้ว" {
		t.Fatalf("ไม่ได้อัปเดต: %+v", found)
	}
	if found.UserID != user.ID || found.BlogID != blog.ID {
		t.Fatal("เจ้าของหรือ blog ถูกเปลี่ยนผ่าน Update ได้")
	}
	if !found.UpdatedAt.After(found.CreatedAt) {
		t.Fatal("updated_at ไม่ขยับ")
	}
}

// ลบ blog แบบถาวรแล้ว comment ต้องหายตาม (FK CASCADE)
func TestCommentsDieWithBlog(t *testing.T) {
	reset(t)
	user := newUser(t, "daew@example.com")
	blog := newBlog(t, user.ID, "Hello", "x")
	newComment(t, blog.ID, user.ID, "อยู่ได้ไม่นาน")

	if err := testDB.Exec("DELETE FROM blogs WHERE id = ?", blog.ID).Error; err != nil {
		t.Fatalf("ลบ blog: %v", err)
	}

	var rows int64
	if err := testDB.Raw("SELECT count(*) FROM comments").Scan(&rows).Error; err != nil {
		t.Fatalf("นับแถว: %v", err)
	}
	if rows != 0 {
		t.Fatalf("ลบ blog แล้วยังเหลือ comment %d แถว", rows)
	}
}

// user ที่ยังมีคอมเมนต์ค้างต้องลบไม่ได้ (FK RESTRICT เหมือน blogs)
func TestUserWithCommentsCannotBeDeleted(t *testing.T) {
	reset(t)
	author := newUser(t, "daew@example.com")
	other := newUser(t, "somchai@example.com")
	blog := newBlog(t, other.ID, "Hello", "x")
	newComment(t, blog.ID, author.ID, "คอมเมนต์ค้าง")

	if err := testDB.Exec("DELETE FROM users WHERE id = ?", author.ID).Error; err == nil {
		t.Fatal("ลบ user ที่ยังมีคอมเมนต์ค้างได้")
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}
