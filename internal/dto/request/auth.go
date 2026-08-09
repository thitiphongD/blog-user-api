package request

type RegisterRequest struct {
	Name string `json:"name"  validate:"required,min=2,max=100"`
	// max=72 เพราะ bcrypt กินได้แค่ 72 bytes เกินกว่านั้นมันตัดทิ้งเงียบๆ
	Email    string `json:"email"    validate:"required,email,max=255"`
	Password string `json:"password" validate:"required,min=8,max=72"`
}

type LoginRequest struct {
	Email    string `json:"email"    validate:"required,email"`
	Password string `json:"password" validate:"required"`
}
