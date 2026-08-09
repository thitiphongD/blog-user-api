package auth_test

import (
	"strings"
	"testing"
	"time"

	"github.com/thitiphongD/blog-user-api/internal/auth"
)

func TestHashPasswordRoundTrip(t *testing.T) {
	hashed, err := auth.HashPassword("password123")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}

	if hashed == "password123" {
		t.Fatal("เก็บเป็น plain text")
	}
	if !auth.ComparePassword(hashed, "password123") {
		t.Fatal("compare กับรหัสเดิมไม่ผ่าน")
	}
	if auth.ComparePassword(hashed, "password124") {
		t.Fatal("compare กับรหัสผิดดันผ่าน")
	}
}

// salt ต้องต่างกันทุกครั้ง ไม่งั้นรหัสเดียวกันจะได้ hash เดียวกัน = ทำ rainbow table ได้
func TestHashPasswordIsSalted(t *testing.T) {
	first, err := auth.HashPassword("password123")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}

	second, err := auth.HashPassword("password123")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}

	if first == second {
		t.Fatal("hash ซ้ำกันสองครั้ง = ไม่ได้ใส่ salt")
	}
}

// เอกสารพฤติกรรมจริงของ bcrypt เวอร์ชันที่ใช้อยู่ — นี่คือเหตุผลที่ validator ปัก max=72
// ทั้งฝั่ง register และ login
func TestBcrypt72ByteLimit(t *testing.T) {
	t.Run("hash รหัสเกิน 72 bytes = error ไม่ใช่ตัดเงียบ", func(t *testing.T) {
		if _, err := auth.HashPassword(strings.Repeat("a", 73)); err == nil {
			t.Fatal("ไม่ error — ถ้า validator ไม่กั้น register จะกลายเป็น 500")
		}
	})

	t.Run("compare ยังตัดที่ 72 อยู่", func(t *testing.T) {
		exactly72 := strings.Repeat("a", 72)

		hashed, err := auth.HashPassword(exactly72)
		if err != nil {
			t.Fatalf("hash: %v", err)
		}

		// ถ้าอันนี้กลายเป็น false เมื่อไหร่ แปลว่า bcrypt เปลี่ยนพฤติกรรม
		// เอา max=72 ออกจาก LoginRequest ได้แล้ว
		if !auth.ComparePassword(hashed, exactly72+"ต่อท้ายอะไรก็ได้") {
			t.Skip("bcrypt ไม่ตัดที่ 72 แล้ว — ทบทวนกฎ max=72 ฝั่ง login ได้")
		}
	})
}

func TestBcryptCostIs12(t *testing.T) {
	if auth.BcryptCost != 12 {
		t.Fatalf("cost = %d — ต่ำกว่านี้ brute force ง่ายขึ้น สูงกว่านี้ login ช้าจนน่ารำคาญ", auth.BcryptCost)
	}
}

// BurnCompare ต้องเผา CPU จริง ไม่ใช่ no-op ไม่งั้นวัดเวลา login แล้วเดาได้ว่า email ไหนมีในระบบ
func TestBurnCompareActuallyCostsTime(t *testing.T) {
	start := time.Now()
	auth.BurnCompare("password123")
	elapsed := time.Since(start)

	// bcrypt cost 12 ใช้เวลาหลักร้อย ms บนเครื่องจริง ตั้งเพดานล่างต่ำๆ ไว้กัน flaky
	if elapsed < 10*time.Millisecond {
		t.Fatalf("ใช้เวลาแค่ %v — เร็วเกินกว่าจะเป็น bcrypt จริง", elapsed)
	}
}
