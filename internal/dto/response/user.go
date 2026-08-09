package response

import (
	"time"

	"github.com/google/uuid"

	"github.com/thitiphongD/blog-user-api/internal/model"
)

type UserResponse struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

// AuthorResponse ใช้ฝังใน BlogResponse เท่านั้น — ไม่มี email เพราะ /blogs เป็น public
// ส่วน /users ปิดไว้ ถ้าใส่ email ตรงนี้ก็เท่ากับปิดประตูหน้าแล้วเปิดประตูหลัง
type AuthorResponse struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

// NewUserResponse บังคับเวลาเป็น UTC — pgx คืน time.Time เป็น Local เสมอ
// ถ้าไม่แปลง created_at จะเป็น +07:00 ขณะที่ timestamp ของ envelope เป็น UTC
func NewUserResponse(u model.User) UserResponse {
	return UserResponse{
		ID:        u.ID,
		Name:      u.Name,
		Email:     u.Email,
		CreatedAt: u.CreatedAt.UTC(),
	}
}

func NewUserResponses(users []model.User) []UserResponse {
	out := make([]UserResponse, 0, len(users))
	for _, u := range users {
		out = append(out, NewUserResponse(u))
	}
	return out
}

func NewAuthorResponse(u model.User) AuthorResponse {
	return AuthorResponse{ID: u.ID, Name: u.Name}
}
