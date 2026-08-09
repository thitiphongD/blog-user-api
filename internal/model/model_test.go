package model_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/thitiphongD/blog-user-api/internal/model"
)

// id สร้างฝั่ง app ไม่ได้พึ่ง DB default — hook ต้องเติมให้เมื่อยังว่าง
func TestBeforeCreateFillsID(t *testing.T) {
	user := &model.User{Name: "Daew"}
	if err := user.BeforeCreate(nil); err != nil {
		t.Fatalf("user hook: %v", err)
	}
	if user.ID == uuid.Nil {
		t.Fatal("user ไม่ได้ id")
	}

	blog := &model.Blog{Title: "Hello"}
	if err := blog.BeforeCreate(nil); err != nil {
		t.Fatalf("blog hook: %v", err)
	}
	if blog.ID == uuid.Nil {
		t.Fatal("blog ไม่ได้ id")
	}
}

// ตั้ง id มาเองแล้วห้ามถูกทับ ไม่งั้น seed หรือ import ข้อมูลจะเพี้ยน
func TestBeforeCreateKeepsExistingID(t *testing.T) {
	id := uuid.New()

	user := &model.User{ID: id}
	if err := user.BeforeCreate(nil); err != nil {
		t.Fatalf("hook: %v", err)
	}
	if user.ID != id {
		t.Fatalf("id ถูกทับเป็น %v", user.ID)
	}
}

func TestSortWhitelist(t *testing.T) {
	ok := []string{model.SortCreatedAt, model.SortTitle}
	for _, s := range ok {
		if !model.IsValidSort(s) {
			t.Fatalf("%q ควรผ่าน", s)
		}
	}

	bad := []string{"", "password", "created_at desc", "title; DROP TABLE blogs", "CREATED_AT"}
	for _, s := range bad {
		if model.IsValidSort(s) {
			t.Fatalf("%q ไม่ควรผ่าน — มันจะไปโผล่ใน ORDER BY", s)
		}
	}
}

func TestOrderWhitelist(t *testing.T) {
	for _, o := range []string{model.OrderAsc, model.OrderDesc} {
		if !model.IsValidOrder(o) {
			t.Fatalf("%q ควรผ่าน", o)
		}
	}

	for _, o := range []string{"", "ASC", "sideways", "desc; DELETE FROM blogs"} {
		if model.IsValidOrder(o) {
			t.Fatalf("%q ไม่ควรผ่าน", o)
		}
	}
}

func TestRefreshTokenBeforeCreateFillsID(t *testing.T) {
	token := &model.RefreshToken{TokenHash: "hash"}
	if err := token.BeforeCreate(nil); err != nil {
		t.Fatalf("hook: %v", err)
	}
	if token.ID == uuid.Nil {
		t.Fatal("ไม่ได้ id")
	}

	fixed := uuid.New()
	kept := &model.RefreshToken{ID: fixed}
	if err := kept.BeforeCreate(nil); err != nil {
		t.Fatalf("hook: %v", err)
	}
	if kept.ID != fixed {
		t.Fatal("id ที่ตั้งมาเองถูกทับ")
	}
}

func TestRefreshTokenUsable(t *testing.T) {
	now := time.Now()
	revoked := now.Add(-time.Minute)

	cases := []struct {
		name  string
		token model.RefreshToken
		want  bool
	}{
		{"ยังไม่หมดอายุและไม่ถูกเพิกถอน", model.RefreshToken{ExpiresAt: now.Add(time.Hour)}, true},
		{"หมดอายุแล้ว", model.RefreshToken{ExpiresAt: now.Add(-time.Hour)}, false},
		{"ถูกเพิกถอนแล้ว", model.RefreshToken{ExpiresAt: now.Add(time.Hour), RevokedAt: &revoked}, false},
		{"หมดอายุพอดี", model.RefreshToken{ExpiresAt: now}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.token.Usable(now); got != tc.want {
				t.Fatalf("Usable = %v อยากได้ %v", got, tc.want)
			}
		})
	}
}

func TestCommentBeforeCreateFillsID(t *testing.T) {
	comment := &model.Comment{Content: "x"}
	if err := comment.BeforeCreate(nil); err != nil {
		t.Fatalf("hook: %v", err)
	}
	if comment.ID == uuid.Nil {
		t.Fatal("ไม่ได้ id")
	}

	fixed := uuid.New()
	kept := &model.Comment{ID: fixed}
	if err := kept.BeforeCreate(nil); err != nil {
		t.Fatalf("hook: %v", err)
	}
	if kept.ID != fixed {
		t.Fatal("id ที่ตั้งมาเองถูกทับ")
	}
}
