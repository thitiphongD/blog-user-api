package auth

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// BcryptCost 12 — request ที่ validate ผ่านจะ hash ด้วยค่านี้เสมอ
const BcryptCost = 12

// dummyHash ไว้เผา CPU ตอน login ด้วย email ที่ไม่มีในระบบ ให้ใช้เวลาพอๆ กับกรณีเจอ user จริง
// ไม่งั้นวัด response time แล้วเดาได้ว่า email ไหนสมัครไว้ — ต้อง cost เดียวกับของจริง
var dummyHash []byte

func init() {
	h, err := bcrypt.GenerateFromPassword([]byte("dummy-password-for-timing"), BcryptCost)
	if err != nil {
		panic(fmt.Sprintf("generate dummy hash: %v", err))
	}
	dummyHash = h
}

func HashPassword(plain string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(plain), BcryptCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hashed), nil
}

func ComparePassword(hashed, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hashed), []byte(plain)) == nil
}

// BurnCompare เรียกตอนหา user ไม่เจอ เพื่อให้เวลาที่ใช้ใกล้เคียงกับตอนเจอ
func BurnCompare(plain string) {
	_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(plain))
}
