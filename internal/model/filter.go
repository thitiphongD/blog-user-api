package model

import "github.com/google/uuid"

// Sort/Order เป็น whitelist — ค่าที่หลุดเข้ามาใน ORDER BY ต้องมาจาก constant พวกนี้เท่านั้น
// ห้ามเอา string จาก query มาต่อ SQL ตรงๆ
const (
	SortCreatedAt = "created_at"
	SortTitle     = "title"

	OrderAsc  = "asc"
	OrderDesc = "desc"
)

// BlogFilter อยู่ใน model เพราะทั้ง service และ repository ต้องใช้ร่วมกัน
// ถ้าไปวางไว้ฝั่ง repository service จะต้อง import repository = ผูกกลับหัวกลับหาง
type BlogFilter struct {
	Search string
	Sort   string
	Order  string
	UserID *uuid.UUID
	Offset int
	Limit  int
}

func IsValidSort(s string) bool {
	return s == SortCreatedAt || s == SortTitle
}

func IsValidOrder(o string) bool {
	return o == OrderAsc || o == OrderDesc
}
