// Package apperr เก็บ error กลางของ domain
//
// repository แปลง error ของ GORM เป็นตัวพวกนี้ก่อนคืนเสมอ — service กับ handler
// ไม่ต้องรู้จัก gorm.ErrRecordNotFound หรือ pgconn error code
package apperr

import "errors"

var (
	ErrNotFound          = errors.New("not found")
	ErrEmailTaken        = errors.New("email already exists")
	ErrInvalidCredential = errors.New("invalid email or password")
	ErrForbidden         = errors.New("permission denied")
	ErrUnauthorized      = errors.New("unauthorized")
	ErrInvalidRefresh    = errors.New("invalid or expired refresh token")
)

// NotFound ห่อ ErrNotFound พร้อมชื่อ resource เพื่อให้ message ตรงกับของที่หาไม่เจอจริง
// เช่น NotFound("Blog") → "Blog not found"
func NotFound(resource string) error {
	return &notFoundError{resource: resource}
}

type notFoundError struct {
	resource string
}

func (e *notFoundError) Error() string { return e.resource + " not found" }

func (e *notFoundError) Is(target error) bool { return target == ErrNotFound }

func (e *notFoundError) Unwrap() error { return ErrNotFound }
