package request

import (
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/thitiphongD/blog-user-api/internal/model"
)

// BlogQuery = ListQuery + ตัวกรองเฉพาะของ blog
type BlogQuery struct {
	ListQuery
	Search string `query:"search"`
	Sort   string `query:"sort"`
	Order  string `query:"order"`
	UserID string `query:"user_id"`

	userID *uuid.UUID
}

// Normalize เติม default + ตรวจ whitelist
// ค่าที่ไม่อยู่ใน whitelist คืน error ไม่เงียบๆ ถอยไปใช้ default — client ส่งผิดควรได้รู้
func (q *BlogQuery) Normalize() error {
	q.ListQuery.Normalize()

	if q.Sort == "" {
		q.Sort = model.SortCreatedAt
	}
	if !model.IsValidSort(q.Sort) {
		return fmt.Errorf("sort must be one of: %s, %s", model.SortCreatedAt, model.SortTitle)
	}

	if q.Order == "" {
		q.Order = model.OrderDesc
	}
	if !model.IsValidOrder(q.Order) {
		return fmt.Errorf("order must be one of: %s, %s", model.OrderAsc, model.OrderDesc)
	}

	if q.UserID != "" {
		id, err := uuid.Parse(q.UserID)
		if err != nil {
			return errors.New("user_id must be a valid UUID")
		}
		q.userID = &id
	}

	return nil
}

func (q BlogQuery) Filter() model.BlogFilter {
	return model.BlogFilter{
		Search: q.Search,
		Sort:   q.Sort,
		Order:  q.Order,
		UserID: q.userID,
		Offset: q.Offset(),
		Limit:  q.Limit,
	}
}
