package auth_test

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/thitiphongD/blog-user-api/internal/auth"
)

// token ต้องสุ่มใหม่ทุกครั้ง ไม่งั้นคนที่เคยได้ token จะเดาของคนอื่นได้
func TestNewRefreshTokenIsRandom(t *testing.T) {
	seen := map[string]bool{}

	for range 100 {
		token, err := auth.NewRefreshToken()
		if err != nil {
			t.Fatalf("generate: %v", err)
		}

		if seen[token] {
			t.Fatal("สุ่มออกมาซ้ำ")
		}
		seen[token] = true

		raw, err := base64.RawURLEncoding.DecodeString(token)
		if err != nil {
			t.Fatalf("token ไม่ใช่ base64url ที่ใส่ใน JSON/header ได้: %v", err)
		}
		if len(raw) != 32 {
			t.Fatalf("เอนโทรปี %d bytes อยากได้ 32", len(raw))
		}
		if strings.ContainsAny(token, "+/=") {
			t.Fatalf("มีอักขระที่ต้อง escape ใน URL: %q", token)
		}
	}
}

func TestHashRefreshToken(t *testing.T) {
	token := "some-refresh-token"
	hash := auth.HashRefreshToken(token)

	if hash == token {
		t.Fatal("ไม่ได้ hash เลย")
	}
	if hash != auth.HashRefreshToken(token) {
		t.Fatal("hash ไม่คงที่ — หาแถวใน DB ไม่เจอแน่")
	}
	if hash == auth.HashRefreshToken(token+"x") {
		t.Fatal("token คนละตัวได้ hash เดียวกัน")
	}
	if len(hash) != 64 {
		t.Fatalf("ความยาว hash = %d อยากได้ 64 (sha256 hex)", len(hash))
	}
}
