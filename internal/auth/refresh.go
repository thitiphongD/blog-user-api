package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

const refreshTokenBytes = 32

// NewRefreshToken สุ่ม token ดิบ 32 bytes — ไม่ใช่ JWT เพราะไม่ต้องการให้อ่านอะไรจากตัวมันได้
// ความปลอดภัยมาจากความสุ่มล้วนๆ และการเทียบกับแถวใน DB
func NewRefreshToken() (string, error) {
	buf := make([]byte, refreshTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("random refresh token: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// HashRefreshToken ใช้ SHA-256 ไม่ใช่ bcrypt — ตัว token สุ่มมา 256 bit อยู่แล้ว ไม่มีอะไรให้เดา
// และต้องหาแถวด้วย hash ตรงๆ ซึ่ง bcrypt ทำไม่ได้ (salt ต่างกันทุกครั้ง)
func HashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))

	return hex.EncodeToString(sum[:])
}
