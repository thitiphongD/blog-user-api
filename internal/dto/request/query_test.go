package request

import (
	"testing"

	"github.com/google/uuid"

	"github.com/thitiphongD/blog-user-api/internal/model"
)

func TestListQueryNormalizeClamps(t *testing.T) {
	cases := []struct {
		name                string
		page, limit         int
		wantPage, wantLimit int
		wantOffset          int
	}{
		{"ค่าว่างได้ default", 0, 0, 1, 20, 0},
		{"ติดลบเด้งกลับเป็น 1", -5, -5, 1, 20, 0},
		{"limit เกิน max โดน clamp", 1, 999999, 1, 100, 0},
		{"offset คำนวณถูก", 3, 10, 3, 10, 20},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q := ListQuery{Page: tc.page, Limit: tc.limit}
			q.Normalize()

			if q.Page != tc.wantPage || q.Limit != tc.wantLimit {
				t.Fatalf("page/limit = %d/%d อยากได้ %d/%d", q.Page, q.Limit, tc.wantPage, tc.wantLimit)
			}
			if q.Offset() != tc.wantOffset {
				t.Fatalf("offset = %d อยากได้ %d", q.Offset(), tc.wantOffset)
			}
		})
	}
}

func TestBlogQueryDefaults(t *testing.T) {
	q := BlogQuery{}
	if err := q.Normalize(); err != nil {
		t.Fatalf("normalize: %v", err)
	}

	f := q.Filter()
	if f.Sort != model.SortCreatedAt || f.Order != model.OrderDesc {
		t.Fatalf("default = %s/%s อยากได้ created_at/desc", f.Sort, f.Order)
	}
	if f.UserID != nil {
		t.Fatal("ไม่ได้ส่ง user_id มาแต่ filter ดันมีค่า")
	}
}

func TestBlogQueryRejectsBadInput(t *testing.T) {
	cases := []struct {
		name string
		q    BlogQuery
	}{
		{"sort นอก whitelist", BlogQuery{Sort: "password"}},
		{"sort แถม SQL", BlogQuery{Sort: "title; DROP TABLE blogs"}},
		{"order นอก whitelist", BlogQuery{Order: "sideways"}},
		{"user_id ไม่ใช่ uuid", BlogQuery{UserID: "hack"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q := tc.q
			if err := q.Normalize(); err == nil {
				t.Fatal("ควรถูกปฏิเสธ แต่ผ่านไปได้")
			}
		})
	}
}

func TestBlogQueryCarriesUserID(t *testing.T) {
	id := uuid.New()

	q := BlogQuery{UserID: id.String()}
	if err := q.Normalize(); err != nil {
		t.Fatalf("normalize: %v", err)
	}

	f := q.Filter()
	if f.UserID == nil || *f.UserID != id {
		t.Fatalf("filter user_id = %v อยากได้ %v", f.UserID, id)
	}
}
