// Package handler ทำแค่ รับ request → validate → เรียก service → คืน response
// business logic ห้ามโผล่ในนี้ และ error ไม่ต้อง map เอง ปล่อยขึ้นไปให้ ErrorHandler
package handler

import (
	"github.com/labstack/echo/v4"

	"github.com/thitiphongD/blog-user-api/internal/dto/request"
	dto "github.com/thitiphongD/blog-user-api/internal/dto/response"
	"github.com/thitiphongD/blog-user-api/internal/middleware"
	"github.com/thitiphongD/blog-user-api/internal/response"
	"github.com/thitiphongD/blog-user-api/internal/service"
)

type AuthHandler struct {
	auth *service.AuthService
}

func NewAuthHandler(auth *service.AuthService) *AuthHandler {
	return &AuthHandler{auth: auth}
}

func (h *AuthHandler) Register(c echo.Context) error {
	var req request.RegisterRequest
	if err := c.Bind(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	if err := c.Validate(&req); err != nil {
		return err
	}

	user, err := h.auth.Register(c.Request().Context(), req)
	if err != nil {
		return err
	}

	return response.Created(c, "User created successfully", dto.NewUserResponse(*user))
}

func (h *AuthHandler) Login(c echo.Context) error {
	var req request.LoginRequest
	if err := c.Bind(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	if err := c.Validate(&req); err != nil {
		return err
	}

	result, err := h.auth.Login(c.Request().Context(), req)
	if err != nil {
		return err
	}

	return response.SuccessWithMessage(c, "Login successfully",
		dto.NewAuthResponse(result.Token, result.ExpiredAt, *result.User))
}

func (h *AuthHandler) Me(c echo.Context) error {
	userID, err := middleware.UserID(c)
	if err != nil {
		return err
	}

	user, err := h.auth.GetMe(c.Request().Context(), userID)
	if err != nil {
		return err
	}

	return response.Success(c, dto.NewUserResponse(*user))
}
