package response

import (
	"time"

	"github.com/google/uuid"

	"github.com/thitiphongD/blog-user-api/internal/model"
)

type BlogResponse struct {
	ID        uuid.UUID      `json:"id"`
	Title     string         `json:"title"`
	Content   string         `json:"content"`
	Author    AuthorResponse `json:"author"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

func NewBlogResponse(b model.Blog) BlogResponse {
	return BlogResponse{
		ID:        b.ID,
		Title:     b.Title,
		Content:   b.Content,
		Author:    NewAuthorResponse(b.User),
		CreatedAt: b.CreatedAt.UTC(),
		UpdatedAt: b.UpdatedAt.UTC(),
	}
}

func NewBlogResponses(blogs []model.Blog) []BlogResponse {
	out := make([]BlogResponse, 0, len(blogs))
	for _, b := range blogs {
		out = append(out, NewBlogResponse(b))
	}
	return out
}
