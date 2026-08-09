package request

// UpdateCommentRequest ใช้กับ PUT = replace เต็ม เหมือน blog
type CreateCommentRequest struct {
	Content string `json:"content" validate:"required,min=1,max=2000"`
}

type UpdateCommentRequest struct {
	Content string `json:"content" validate:"required,min=1,max=2000"`
}
