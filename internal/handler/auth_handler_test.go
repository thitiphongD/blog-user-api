package handler_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/thitiphongD/blog-user-api/internal/apperr"
	"github.com/thitiphongD/blog-user-api/internal/auth"
	"github.com/thitiphongD/blog-user-api/internal/model"
)

const registerBody = `{"name":"Daew","email":"daew@example.com","password":"password123"}`

func TestRegisterCreated(t *testing.T) {
	app := newApp(t)
	app.users.create = func(_ context.Context, u *model.User) error {
		u.ID = uuid.New()
		return nil
	}

	rec := app.do(t, http.MethodPost, "/api/v1/auth/register", registerBody, "")

	body := assertStatus(t, rec, http.StatusCreated)
	if body["message"] != "User created successfully" {
		t.Fatalf("message = %v", body["message"])
	}

	data := dataOf(t, body)
	if data["email"] != "daew@example.com" {
		t.Fatalf("email = %v", data["email"])
	}
	if _, leaked := data["password"]; leaked {
		t.Fatal("password หลุดออก response")
	}
}

func TestRegisterDuplicateEmail(t *testing.T) {
	app := newApp(t)
	app.users.create = func(context.Context, *model.User) error { return apperr.ErrEmailTaken }

	rec := app.do(t, http.MethodPost, "/api/v1/auth/register", registerBody, "")

	body := assertStatus(t, rec, http.StatusConflict)
	if body["message"] != "Email already exists" {
		t.Fatalf("message = %v", body["message"])
	}
}

// JSON ถูกแต่ค่าไม่ผ่านกติกา = 422 คนละเคสกับ 400
func TestRegisterValidationFails(t *testing.T) {
	app := newApp(t)

	rec := app.do(t, http.MethodPost, "/api/v1/auth/register",
		`{"name":"D","email":"ไม่ใช่อีเมล","password":"123"}`, "")

	body := assertStatus(t, rec, http.StatusUnprocessableEntity)

	errs, ok := body["errors"].(map[string]any)
	if !ok {
		t.Fatalf("ไม่มี errors map: %s", rec.Body.String())
	}

	for _, field := range []string{"name", "email", "password"} {
		if errs[field] == nil {
			t.Fatalf("ไม่ได้บอกว่า %s ผิด: %v", field, errs)
		}
	}
}

// password ยาวเกิน 72 bytes ต้องโดนปฏิเสธ ไม่ใช่ปล่อยให้ bcrypt ตัดทิ้งเงียบๆ
// แล้วรหัสสองอันที่ 72 ตัวแรกเหมือนกันกลายเป็นใช้แทนกันได้
func TestRegisterRejectsPasswordOver72Bytes(t *testing.T) {
	app := newApp(t)
	long := strings.Repeat("a", 73)

	rec := app.do(t, http.MethodPost, "/api/v1/auth/register",
		`{"name":"Daew","email":"daew@example.com","password":"`+long+`"}`, "")

	assertStatus(t, rec, http.StatusUnprocessableEntity)
}

// JSON พังคือ parse ไม่ได้ = 400 ไม่ใช่ 422
func TestRegisterBrokenJSON(t *testing.T) {
	app := newApp(t)

	rec := app.do(t, http.MethodPost, "/api/v1/auth/register", `{oops`, "")

	body := assertStatus(t, rec, http.StatusBadRequest)
	if body["message"] != "Invalid request body" {
		t.Fatalf("message = %v", body["message"])
	}
}

func TestLoginReturnsUsableToken(t *testing.T) {
	app := newApp(t)

	hashed, err := auth.HashPassword("password123")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}

	id := uuid.New()
	app.users.findByEmail = func(context.Context, string) (*model.User, error) {
		return &model.User{ID: id, Email: "daew@example.com", Password: hashed}, nil
	}

	rec := app.do(t, http.MethodPost, "/api/v1/auth/login",
		`{"email":"daew@example.com","password":"password123"}`, "")

	body := assertStatus(t, rec, http.StatusOK)
	if body["message"] != "Login successfully" {
		t.Fatalf("message = %v", body["message"])
	}

	data := dataOf(t, body)

	token, ok := data["token"].(string)
	if !ok || token == "" {
		t.Fatalf("ไม่ได้ token: %v", data)
	}
	if data["expired_at"] == nil {
		t.Fatal("ไม่มี expired_at")
	}

	// token ที่ออกมาต้องใช้กับ endpoint ที่ป้องกันได้จริง ไม่ใช่แค่เป็น string
	app.users.findByID = func(context.Context, uuid.UUID) (*model.User, error) {
		return &model.User{ID: id, Email: "daew@example.com"}, nil
	}

	me := app.do(t, http.MethodGet, "/api/v1/auth/me", "", token)
	assertStatus(t, me, http.StatusOK)
}

func TestLoginWrongPassword(t *testing.T) {
	app := newApp(t)

	hashed, err := auth.HashPassword("password123")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}

	app.users.findByEmail = func(context.Context, string) (*model.User, error) {
		return &model.User{ID: uuid.New(), Password: hashed}, nil
	}

	rec := app.do(t, http.MethodPost, "/api/v1/auth/login",
		`{"email":"daew@example.com","password":"wrong-password"}`, "")

	body := assertStatus(t, rec, http.StatusUnauthorized)
	if body["message"] != "Invalid email or password" {
		t.Fatalf("message = %v", body["message"])
	}
}

// email ที่ไม่มีในระบบต้องได้ 401 ข้อความเดียวกับรหัสผิดเป๊ะ
// ถ้าหลุดเป็น 404 หรือข้อความอื่น = บอกใบ้ว่า email ไหนสมัครไว้
func TestLoginUnknownEmailLooksIdenticalToWrongPassword(t *testing.T) {
	app := newApp(t)
	app.users.findByEmail = func(context.Context, string) (*model.User, error) {
		return nil, apperr.NotFound("User")
	}

	rec := app.do(t, http.MethodPost, "/api/v1/auth/login",
		`{"email":"nobody@example.com","password":"password123"}`, "")

	body := assertStatus(t, rec, http.StatusUnauthorized)
	if body["message"] != "Invalid email or password" {
		t.Fatalf("message = %v — ต่างจากเคสรหัสผิดคือบอกใบ้ว่า email นี้ไม่มีในระบบ", body["message"])
	}
}

func TestMeWithoutToken(t *testing.T) {
	app := newApp(t)

	rec := app.do(t, http.MethodGet, "/api/v1/auth/me", "", "")

	assertStatus(t, rec, http.StatusUnauthorized)
}

func TestMeWithGarbageToken(t *testing.T) {
	app := newApp(t)

	rec := app.do(t, http.MethodGet, "/api/v1/auth/me", "", "abc.def.ghi")

	assertStatus(t, rec, http.StatusUnauthorized)
}

// token ที่เซ็นด้วย secret อื่นต้องใช้ไม่ได้
func TestMeRejectsTokenSignedWithOtherSecret(t *testing.T) {
	app := newApp(t)

	other := auth.NewJWT("secret-ของคนอื่น", time.Hour)
	token, _, err := other.Generate(uuid.New())
	if err != nil {
		t.Fatalf("gen token: %v", err)
	}

	rec := app.do(t, http.MethodGet, "/api/v1/auth/me", "", token)

	assertStatus(t, rec, http.StatusUnauthorized)
}
