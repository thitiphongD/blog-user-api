package handler

import (
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"github.com/thitiphongD/blog-user-api/internal/response"
)

type HealthHandler struct {
	db *gorm.DB
}

func NewHealthHandler(db *gorm.DB) *HealthHandler {
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
	sqlDB, err := h.db.DB()
	if err != nil {
		return response.InternalServerError(c)
	}

	if err := sqlDB.PingContext(c.Request().Context()); err != nil {
		return response.InternalServerError(c)
	}

	return response.Success(c, map[string]string{"database": "up"})
}
