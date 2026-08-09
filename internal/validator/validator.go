// Package validator ห่อ go-playground/validator แล้วแปลง error เป็น map ที่ client อ่านรู้เรื่อง
package validator

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
)

// Error ถือ error ราย field ที่ validate ไม่ผ่าน — global error handler แปลงเป็น 422
type Error struct {
	Fields map[string]string
}

func (e *Error) Error() string { return "validation failed" }

type Validator struct {
	v *validator.Validate
}

func New() *Validator {
	v := validator.New()

	// ใช้ชื่อจาก json tag เป็น key ของ error เพื่อให้ตรงกับ field ที่ client ส่งมาจริง
	v.RegisterTagNameFunc(func(f reflect.StructField) string {
		name := strings.Split(f.Tag.Get("json"), ",")[0]
		if name == "" || name == "-" {
			return f.Name
		}
		return name
	})

	return &Validator{v: v}
}

// Validate ทำตาม interface ของ echo — ไม่ผ่านคืน *Error เสมอ
func (val *Validator) Validate(i any) error {
	err := val.v.Struct(i)
	if err == nil {
		return nil
	}

	var validationErrs validator.ValidationErrors
	if !errors.As(err, &validationErrs) {
		return err
	}

	fields := make(map[string]string, len(validationErrs))
	for _, fe := range validationErrs {
		fields[fe.Field()] = message(fe)
	}

	return &Error{Fields: fields}
}

func message(fe validator.FieldError) string {
	label := humanize(fe.Field())

	switch fe.Tag() {
	case "required":
		return label + " is required"
	case "email":
		return label + " must be a valid email"
	case "min":
		return fmt.Sprintf("%s must be at least %s characters", label, fe.Param())
	case "max":
		return fmt.Sprintf("%s must be at most %s characters", label, fe.Param())
	case "oneof":
		return fmt.Sprintf("%s must be one of: %s", label, strings.ReplaceAll(fe.Param(), " ", ", "))
	case "uuid":
		return label + " must be a valid UUID"
	default:
		return label + " is invalid"
	}
}

// humanize แปลง json tag เป็นคำที่เอาไปขึ้นต้นประโยคได้ เช่น created_at → Created at
func humanize(field string) string {
	words := strings.Split(field, "_")
	if len(words) == 0 || words[0] == "" {
		return field
	}

	words[0] = strings.ToUpper(words[0][:1]) + words[0][1:]

	return strings.Join(words, " ")
}
