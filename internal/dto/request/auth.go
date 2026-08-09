package request

type RegisterRequest struct {
	Name string `json:"name"  validate:"required,min=2,max=100"`
	// max=72 เพราะ bcrypt กินได้แค่ 72 bytes เกินกว่านั้นมันตัดทิ้งเงียบๆ
	Email    string `json:"email"    validate:"required,email,max=255"`
	Password string `json:"password" validate:"required,min=8,max=72"`
}

type LoginRequest struct {
	Email string `json:"email" validate:"required,email"`
	// max=72 ฝั่ง login ด้วย เพราะ bcrypt.CompareHashAndPassword ยังตัดที่ 72 bytes อยู่
	// ไม่กั้นแล้วรหัสยาว 72 ตัวจะยอมรับส่วนท้ายอะไรต่อก็ได้
	Password string `json:"password" validate:"required,max=72"`
}
