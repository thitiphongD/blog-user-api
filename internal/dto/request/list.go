package request

const (
	defaultPage  = 1
	defaultLimit = 20
	maxLimit     = 100
)

// ListQuery ใช้ร่วมกันทั้ง /blogs และ /users
// search / sort / order / user_id เป็นของ P2 ยังไม่มีในนี้
type ListQuery struct {
	Page  int `query:"page"`
	Limit int `query:"limit"`
}

// Normalize เติม default แล้ว clamp limit — ไม่งั้น ?limit=999999 คือยิงตัวเอง
func (q *ListQuery) Normalize() {
	if q.Page < 1 {
		q.Page = defaultPage
	}
	if q.Limit < 1 {
		q.Limit = defaultLimit
	}
	if q.Limit > maxLimit {
		q.Limit = maxLimit
	}
}

func (q ListQuery) Offset() int {
	return (q.Page - 1) * q.Limit
}
