package auth

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// dummyHash ปักเป็นค่าคงที่ ถ้า cost ไม่ตรงกับของจริง เวลาที่ใช้ตอน login fail จะต่างจาก
// ตอน login สำเร็จ แล้ว BurnCompare ก็กัน timing attack ไม่ได้จริง
func TestDummyHashCostMatchesProduction(t *testing.T) {
	cost, err := bcrypt.Cost([]byte(dummyHash))
	if err != nil {
		t.Fatalf("อ่าน cost จาก dummyHash ไม่ได้ — hash เสียหรือเปล่า: %v", err)
	}

	if cost != BcryptCost {
		t.Fatalf("cost ของ dummyHash = %d แต่ของจริงใช้ %d", cost, BcryptCost)
	}
}
