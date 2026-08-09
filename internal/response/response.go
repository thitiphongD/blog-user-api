// Package response คุม shape ของ response ทั้งระบบไว้ที่เดียว
//
// ทุกก้อนมี request_id กับ timestamp ติดไปด้วยเสมอ เอาไว้ตามใน log ตอน 500
package response

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

type Body struct {
	Status     int               `json:"status"`
	Success    bool              `json:"success"`
	Message    string            `json:"message"`
	Data       any               `json:"data,omitempty"`
	Errors     map[string]string `json:"errors,omitempty"`
	Pagination *Pagination       `json:"pagination,omitempty"`
	RequestID  string            `json:"request_id"`
	Timestamp  time.Time         `json:"timestamp"`
}

type Pagination struct {
	Page      int   `json:"page"`
	Limit     int   `json:"limit"`
	Total     int64 `json:"total"`
	TotalPage int   `json:"total_page"`
}

func NewPagination(page, limit int, total int64) Pagination {
	totalPage := 0
	if limit > 0 && total > 0 {
		totalPage = int((total + int64(limit) - 1) / int64(limit))
	}

	return Pagination{Page: page, Limit: limit, Total: total, TotalPage: totalPage}
}

func Success(c echo.Context, data any) error {
	return write(c, http.StatusOK, true, "Success", data, nil, nil)
}

func SuccessWithMessage(c echo.Context, message string, data any) error {
	return write(c, http.StatusOK, true, message, data, nil, nil)
}

func SuccessWithPagination(c echo.Context, data any, p Pagination) error {
	return write(c, http.StatusOK, true, "Success", data, nil, &p)
}

func Created(c echo.Context, message string, data any) error {
	return write(c, http.StatusCreated, true, message, data, nil, nil)
}

func BadRequest(c echo.Context, message string) error {
	return write(c, http.StatusBadRequest, false, message, nil, nil, nil)
}

func Unauthorized(c echo.Context, message string) error {
	return write(c, http.StatusUnauthorized, false, message, nil, nil, nil)
}

func Forbidden(c echo.Context, message string) error {
	return write(c, http.StatusForbidden, false, message, nil, nil, nil)
}

func NotFound(c echo.Context, message string) error {
	return write(c, http.StatusNotFound, false, message, nil, nil, nil)
}

func Conflict(c echo.Context, message string) error {
	return write(c, http.StatusConflict, false, message, nil, nil, nil)
}

func Validation(c echo.Context, errs map[string]string) error {
	return write(c, http.StatusUnprocessableEntity, false, "Validation failed", nil, errs, nil)
}

func InternalServerError(c echo.Context) error {
	return write(c, http.StatusInternalServerError, false, "Internal server error", nil, nil, nil)
}

func write(
	c echo.Context,
	status int,
	success bool,
	message string,
	data any,
	errs map[string]string,
	p *Pagination,
) error {
	return c.JSON(status, Body{
		Status:     status,
		Success:    success,
		Message:    message,
		Data:       data,
		Errors:     errs,
		Pagination: p,
		RequestID:  c.Response().Header().Get(echo.HeaderXRequestID),
		Timestamp:  time.Now().UTC(),
	})
}
