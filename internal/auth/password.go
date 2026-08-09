package auth

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// BcryptCost 12 — request ที่ validate ผ่านจะ hash ด้วยค่านี้เสมอ
const BcryptCost = 12

// dummyHash ไว้เผา CPU ตอน login ด้วย email ที่ไม่มีในระบบ ให้ใช้เวลาพอๆ กับกรณีเจอ user จริง
// ไม่งั้นวัด response time แล้วเดาได้ว่า email ไหนสมัครไว้
//
// ปักเป็นค่าคงที่ไว้เลยแทนที่จะ hash ตอน start — cost ฝังอยู่ในตัว hash อยู่แล้ว ($2a$12$)
// เวลาที่ใช้เลยเท่าของจริงเป๊ะ แถมไม่ต้องเผา 250ms ตอน boot ทุกครั้ง
// (ไม่ใช่ความลับ เป็นแค่เป้าให้ bcrypt เทียบทิ้ง)
const dummyHash = "$2a$12$HOhWZUGQEX4jMxagkpEQQOHxfOIVT4YgM6WQ5KZx4HbIwidhLEq2O"

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
	_ = bcrypt.CompareHashAndPassword([]byte(dummyHash), []byte(plain))
}
