package auth_test

import (
	"encoding/base64"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/thitiphongD/blog-user-api/internal/auth"
)

const secret = "test-secret"

func TestGenerateThenVerify(t *testing.T) {
	j := auth.NewJWT(secret, time.Hour)
	id := uuid.New()

	token, expiredAt, err := j.Generate(id)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	if expiredAt.Before(time.Now()) {
		t.Fatal("expired_at เป็นอดีตตั้งแต่เพิ่งออก")
	}
	if diff := time.Until(expiredAt); diff > time.Hour+time.Minute {
		t.Fatalf("อายุ token = %v ยาวกว่าที่ตั้งไว้", diff)
	}

	got, err := j.Verify(token)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if got != id {
		t.Fatalf("user id = %v อยากได้ %v", got, id)
	}
}

func TestVerifyRejects(t *testing.T) {
	j := auth.NewJWT(secret, time.Hour)

	valid, _, err := j.Generate(uuid.New())
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	other := auth.NewJWT("secret-ของคนอื่น", time.Hour)
	forged, _, err := other.Generate(uuid.New())
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	expiredJWT := auth.NewJWT(secret, -time.Hour)
	expired, _, err := expiredJWT.Generate(uuid.New())
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	cases := []struct {
		name  string
		token string
	}{
		{"ว่าง", ""},
		{"ขยะ", "abc.def.ghi"},
		{"เซ็นด้วย secret อื่น", forged},
		{"หมดอายุแล้ว", expired},
		{"แก้ payload แล้วเซ็นเดิม", tamper(t, valid)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := j.Verify(tc.token); err == nil {
				t.Fatal("ผ่านทั้งที่ไม่ควรผ่าน")
			}
		})
	}
}

// alg: none คือท่าคลาสสิกของการปลอม JWT — ถ้า WithValidMethods หลุดเมื่อไหร่ เทสต์นี้จะจับได้
func TestVerifyRejectsAlgNone(t *testing.T) {
	header := b64(t, `{"alg":"none","typ":"JWT"}`)
	payload := b64(t, `{"user_id":"`+uuid.New().String()+`","exp":`+
		strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10)+`}`)

	forged := header + "." + payload + "."

	if _, err := auth.NewJWT(secret, time.Hour).Verify(forged); err == nil {
		t.Fatal("รับ token ที่ alg เป็น none — ใครก็ปลอมได้")
	}
}

// token ที่เซ็นถูกต้องแต่ไม่มี user_id ต้องไม่ผ่าน ไม่งั้นจะได้ uuid.Nil ไปทั้งระบบ
func TestVerifyRejectsTokenWithoutUserID(t *testing.T) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	})

	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	got, err := auth.NewJWT(secret, time.Hour).Verify(signed)
	if err == nil {
		t.Fatalf("ผ่านไปได้ พร้อม user id = %v", got)
	}
}

func b64(t *testing.T, s string) string {
	t.Helper()

	return base64.RawURLEncoding.EncodeToString([]byte(s))
}

// tamper แก้ payload แต่คง signature เดิมไว้ — ต้อง verify ไม่ผ่าน
func tamper(t *testing.T, token string) string {
	t.Helper()

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("token หน้าตาผิด: %s", token)
	}

	parts[1] = b64(t, `{"user_id":"`+uuid.New().String()+`","exp":`+
		strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10)+`}`)

	return strings.Join(parts, ".")
}
