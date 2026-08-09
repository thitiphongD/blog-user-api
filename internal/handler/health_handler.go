package handler

import (
	"context"

	"github.com/labstack/echo/v4"

	"github.com/thitiphongD/blog-user-api/internal/response"
)

// Pinger คือสิ่งที่ health ต้องการจริงๆ — แค่ ping ได้ ไม่ต้องรู้ว่าข้างล่างเป็น gorm หรืออะไร
// (*sql.DB ใส่ได้ตรงๆ และ mock ในเทสต์ก็เขียนไม่กี่บรรทัด)
type Pinger interface {
	PingContext(ctx context.Context) error
}

type HealthHandler struct {
	db Pinger
}

func NewHealthHandler(db Pinger) *HealthHandler {
	return &HealthHandler{db: db}
}

// Health ping DB ด้วย ไม่ใช่แค่ตอบ 200 ลอยๆ — docker healthcheck ยิงตัวนี้
// api ที่ต่อ DB ไม่ได้ก็ไม่ควรนับว่า healthy
//
//	@Summary	เช็คว่า API กับ DB พร้อม
//	@Tags		health
//	@Produce	json
//	@Success	200	{object}	response.Body
//	@Failure	500	{object}	response.Body
//	@Router		/health [get]
func (h *HealthHandler) Health(c echo.Context) error {
	if err := h.db.PingContext(c.Request().Context()); err != nil {
		return response.InternalServerError(c)
	}

	return response.Success(c, map[string]string{"database": "up"})
}
