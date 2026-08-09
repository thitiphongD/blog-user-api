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

// Register สมัครสมาชิก ไม่ auto-login
//
//	@Summary	สมัครสมาชิก
//	@Tags		auth
//	@Accept		json
//	@Produce	json
//	@Param		body	body		request.RegisterRequest	true	"ข้อมูลสมัคร"
//	@Success	201		{object}	response.Body{data=dto.UserResponse}
//	@Failure	400		{object}	response.Body
//	@Failure	409		{object}	response.Body	"email ซ้ำ"
//	@Failure	422		{object}	response.Body	"validate ไม่ผ่าน"
//	@Router		/api/v1/auth/register [post]
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

// Login แลก email/password เป็น JWT
//
//	@Summary	ล็อกอิน
//	@Tags		auth
//	@Accept		json
//	@Produce	json
//	@Param		body	body		request.LoginRequest	true	"email กับ password"
//	@Success	200		{object}	response.Body{data=dto.AuthResponse}
//	@Failure	401		{object}	response.Body	"email หรือ password ผิด"
//	@Failure	422		{object}	response.Body
//	@Router		/api/v1/auth/login [post]
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

	return response.SuccessWithMessage(c, "Login successfully", authResponse(result))
}

// Refresh แลก refresh token เป็นชุดใหม่ ตัวเดิมใช้ต่อไม่ได้ทันที
//
//	@Summary	ต่ออายุ session
//	@Tags		auth
//	@Accept		json
//	@Produce	json
//	@Param		body	body		request.RefreshRequest	true	"refresh token ที่ได้ตอน login"
//	@Success	200		{object}	response.Body{data=dto.AuthResponse}
//	@Failure	401		{object}	response.Body	"token ผิด หมดอายุ หรือถูกใช้ไปแล้ว"
//	@Failure	422		{object}	response.Body
//	@Router		/api/v1/auth/refresh [post]
func (h *AuthHandler) Refresh(c echo.Context) error {
	var req request.RefreshRequest
	if err := c.Bind(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	if err := c.Validate(&req); err != nil {
		return err
	}

	result, err := h.auth.Refresh(c.Request().Context(), req.RefreshToken)
	if err != nil {
		return err
	}

	return response.SuccessWithMessage(c, "Token refreshed", authResponse(result))
}

// Logout เพิกถอนเฉพาะ session ที่ยื่นมา เครื่องอื่นที่ login ค้างไว้ไม่โดนด้วย
//
//	@Summary	ออกจากระบบ
//	@Tags		auth
//	@Accept		json
//	@Produce	json
//	@Param		body	body		request.RefreshRequest	true	"refresh token ของ session ที่จะออก"
//	@Success	200		{object}	response.Body
//	@Failure	401		{object}	response.Body
//	@Failure	422		{object}	response.Body
//	@Router		/api/v1/auth/logout [post]
func (h *AuthHandler) Logout(c echo.Context) error {
	var req request.RefreshRequest
	if err := c.Bind(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	if err := c.Validate(&req); err != nil {
		return err
	}

	if err := h.auth.Logout(c.Request().Context(), req.RefreshToken); err != nil {
		return err
	}

	return response.SuccessWithMessage(c, "Logout successfully", nil)
}

func authResponse(result *service.LoginResult) dto.AuthResponse {
	return dto.NewAuthResponse(
		result.Token, result.ExpiredAt,
		result.RefreshToken, result.RefreshExpiredAt,
		*result.User,
	)
}

// Me ข้อมูลของเจ้าของ token
//
//	@Summary	ดูข้อมูลตัวเอง
//	@Tags		auth
//	@Produce	json
//	@Security	BearerAuth
//	@Success	200	{object}	response.Body{data=dto.UserResponse}
//	@Failure	401	{object}	response.Body
//	@Router		/api/v1/auth/me [get]
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
