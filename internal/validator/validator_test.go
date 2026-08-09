package validator_test

import (
	"errors"
	"testing"

	"github.com/thitiphongD/blog-user-api/internal/validator"
)

type sample struct {
	Name      string `json:"name"       validate:"required,min=2,max=100"`
	Email     string `json:"email"      validate:"required,email"`
	Password  string `json:"password"   validate:"required,min=8,max=72"`
	Order     string `json:"order"      validate:"omitempty,oneof=asc desc"`
	CreatedBy string `json:"created_by" validate:"required"`
}

func valid() sample {
	return sample{
		Name:      "Daew",
		Email:     "daew@example.com",
		Password:  "password123",
		Order:     "desc",
		CreatedBy: "someone",
	}
}

func TestValidatePasses(t *testing.T) {
	s := valid()

	if err := validator.New().Validate(&s); err != nil {
		t.Fatalf("ของถูกแต่ไม่ผ่าน: %v", err)
	}
}

// key ของ errors ต้องเป็นชื่อจาก json tag ไม่ใช่ชื่อ field ใน Go
// ไม่งั้น client ที่ส่ง created_by มา จะได้ error ชี้ไปที่ "CreatedBy" ที่ไม่มีอยู่จริงในฝั่งเขา
func TestValidateUsesJSONTagAsKey(t *testing.T) {
	s := valid()
	s.CreatedBy = ""

	err := validator.New().Validate(&s)

	var verr *validator.Error
	if !errors.As(err, &verr) {
		t.Fatalf("อยากได้ *validator.Error ได้ %T", err)
	}

	if verr.Fields["created_by"] == "" {
		t.Fatalf("ไม่มี key created_by: %v", verr.Fields)
	}
	if _, wrong := verr.Fields["CreatedBy"]; wrong {
		t.Fatal("ใช้ชื่อ field ของ Go เป็น key")
	}
}

func TestValidateMessages(t *testing.T) {
	cases := []struct {
		name  string
		mut   func(*sample)
		field string
		want  string
	}{
		{"required", func(s *sample) { s.Name = "" }, "name", "Name is required"},
		{"email", func(s *sample) { s.Email = "ไม่ใช่อีเมล" }, "email", "Email must be a valid email"},
		{"min", func(s *sample) { s.Password = "123" }, "password", "Password must be at least 8 characters"},
		{"max", func(s *sample) { s.Name = longName() }, "name", "Name must be at most 100 characters"},
		{"oneof", func(s *sample) { s.Order = "sideways" }, "order", "Order must be one of: asc, desc"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := valid()
			tc.mut(&s)

			var verr *validator.Error
			if !errors.As(validator.New().Validate(&s), &verr) {
				t.Fatal("ควรไม่ผ่าน แต่ผ่าน")
			}

			if got := verr.Fields[tc.field]; got != tc.want {
				t.Fatalf("ข้อความ = %q อยากได้ %q", got, tc.want)
			}
		})
	}
}

// ผิดหลาย field ต้องบอกครบในทีเดียว ไม่ใช่ให้ client ไล่แก้ทีละรอบ
func TestValidateReportsEveryField(t *testing.T) {
	s := sample{}

	var verr *validator.Error
	if !errors.As(validator.New().Validate(&s), &verr) {
		t.Fatal("ควรไม่ผ่าน แต่ผ่าน")
	}

	for _, field := range []string{"name", "email", "password", "created_by"} {
		if verr.Fields[field] == "" {
			t.Fatalf("ไม่ได้บอกว่า %s ผิด: %v", field, verr.Fields)
		}
	}
}

func longName() string {
	name := make([]byte, 101)
	for i := range name {
		name[i] = 'a'
	}

	return string(name)
}

func TestErrorImplementsError(t *testing.T) {
	var err error = &validator.Error{Fields: map[string]string{"name": "Name is required"}}

	if err.Error() != "validation failed" {
		t.Fatalf("Error() = %q", err.Error())
	}
}

// field ที่ไม่มี json tag หรือเป็น "-" ต้องถอยไปใช้ชื่อ field ของ Go
// ไม่งั้น key จะกลายเป็นค่าว่างแล้ว error ทับกันหมด
type noTag struct {
	Plain   string `validate:"required"`
	Skipped string `json:"-"        validate:"required"`
	Weird   string `json:"_secret"  validate:"required"`
}

func TestFieldNameFallback(t *testing.T) {
	var verr *validator.Error
	if !errors.As(validator.New().Validate(&noTag{}), &verr) {
		t.Fatal("ควรไม่ผ่าน แต่ผ่าน")
	}

	for _, key := range []string{"Plain", "Skipped"} {
		if verr.Fields[key] == "" {
			t.Fatalf("ไม่มี key %s: %v", key, verr.Fields)
		}
	}

	// ชื่อที่ขึ้นต้นด้วย _ แปลงเป็นคำขึ้นต้นประโยคไม่ได้ ต้องคืนตามเดิมไม่ใช่พัง
	if got := verr.Fields["_secret"]; got != "_secret is required" {
		t.Fatalf("ข้อความ = %q", got)
	}
}

type tagVariants struct {
	ID     string `json:"id"     validate:"uuid"`
	Amount string `json:"amount" validate:"numeric"`
}

func TestMessagesForOtherTags(t *testing.T) {
	var verr *validator.Error
	if !errors.As(validator.New().Validate(&tagVariants{ID: "not-a-uuid", Amount: "abc"}), &verr) {
		t.Fatal("ควรไม่ผ่าน แต่ผ่าน")
	}

	if got := verr.Fields["id"]; got != "Id must be a valid UUID" {
		t.Fatalf("ข้อความของ uuid = %q", got)
	}

	// tag ที่ยังไม่ได้เขียนข้อความไว้ ต้องได้ข้อความกลางๆ ไม่ใช่ค่าว่าง
	if got := verr.Fields["amount"]; got != "Amount is invalid" {
		t.Fatalf("ข้อความของ tag ที่ไม่รู้จัก = %q", got)
	}
}

// ส่งของที่ไม่ใช่ struct เข้ามา = ใช้ผิดวิธี ต้องเด้ง error เดิมออกไป (กลายเป็น 500)
// ไม่ใช่แปลงเป็น 422 ทั้งที่ client ไม่ได้ทำอะไรผิด
func TestValidateNonStructReturnsRawError(t *testing.T) {
	err := validator.New().Validate("ไม่ใช่ struct")
	if err == nil {
		t.Fatal("ควร error แต่ผ่านไปได้")
	}

	var verr *validator.Error
	if errors.As(err, &verr) {
		t.Fatal("กลายเป็น validation error ทั้งที่เป็นความผิดของโค้ดเราเอง")
	}
}
