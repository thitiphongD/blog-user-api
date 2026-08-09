package response

import (
	"time"

	"github.com/google/uuid"

	"github.com/thitiphongD/blog-user-api/internal/model"
)

type CommentResponse struct {
	ID        uuid.UUID      `json:"id"`
	Content   string         `json:"content"`
	BlogID    uuid.UUID      `json:"blog_id"`
	Author    AuthorResponse `json:"author"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

func NewCommentResponse(c model.Comment) CommentResponse {
	return CommentResponse{
		ID:        c.ID,
		Content:   c.Content,
		BlogID:    c.BlogID,
		Author:    NewAuthorResponse(c.User),
		CreatedAt: c.CreatedAt.UTC(),
		UpdatedAt: c.UpdatedAt.UTC(),
	}
}

func NewCommentResponses(comments []model.Comment) []CommentResponse {
	out := make([]CommentResponse, 0, len(comments))
	for _, c := range comments {
		out = append(out, NewCommentResponse(c))
	}

	return out
}
