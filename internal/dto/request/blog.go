package request

// UpdateBlogRequest ใช้กับ PUT = replace เต็ม ต้องส่งครบทั้งสอง field
// อยากแก้บางส่วนต้องทำเป็น PATCH ซึ่งไม่อยู่ใน scope

type CreateBlogRequest struct {
	Title   string `json:"title"   validate:"required,min=1,max=200"`
	Content string `json:"content" validate:"required,min=1"`
}

type UpdateBlogRequest struct {
	Title   string `json:"title"   validate:"required,min=1,max=200"`
	Content string `json:"content" validate:"required,min=1"`
}
