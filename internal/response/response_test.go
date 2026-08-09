package response

import "testing"

func TestNewPagination(t *testing.T) {
	cases := []struct {
		name          string
		page, limit   int
		total         int64
		wantTotalPage int
	}{
		{"ไม่มีข้อมูลเลย", 1, 20, 0, 0},
		{"หารลงตัว", 2, 20, 120, 6},
		{"เศษต้องปัดขึ้น", 1, 20, 121, 7},
		{"น้อยกว่าหนึ่งหน้า", 1, 20, 3, 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := NewPagination(tc.page, tc.limit, tc.total)

			if got.TotalPage != tc.wantTotalPage {
				t.Fatalf("total_page = %d อยากได้ %d", got.TotalPage, tc.wantTotalPage)
			}
			if got.Page != tc.page || got.Limit != tc.limit || got.Total != tc.total {
				t.Fatalf("pagination ไม่ตรงกับที่ส่งเข้าไป: %+v", got)
			}
		})
	}
}
