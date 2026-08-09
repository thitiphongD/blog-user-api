package handler

import (
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/thitiphongD/blog-user-api/internal/dto/request"
	dto "github.com/thitiphongD/blog-user-api/internal/dto/response"
	"github.com/thitiphongD/blog-user-api/internal/response"
	"github.com/thitiphongD/blog-user-api/internal/service"
)

type UserHandler struct {
	users *service.UserService
}

func NewUserHandler(users *service.UserService) *UserHandler {
	return &UserHandler{users: users}
}

// List รายชื่อ user แบบแบ่งหน้า
//
//	@Summary	list user
//	@Tags		user
//	@Produce	json
//	@Security	BearerAuth
//	@Param		page	query		int	false	"หน้า เริ่มที่ 1"	default(1)
//	@Param		limit	query		int	false	"ต่อหน้า สูงสุด 100"	default(20)
//	@Success	200		{object}	response.Body{data=[]dto.UserResponse}
//	@Failure	401		{object}	response.Body
//	@Router		/api/v1/users [get]
func (h *UserHandler) List(c echo.Context) error {
	var q request.ListQuery
	if err := c.Bind(&q); err != nil {
		return response.BadRequest(c, "Invalid query parameter")
	}
	q.Normalize()

	users, total, err := h.users.GetUsers(c.Request().Context(), q)
	if err != nil {
		return err
	}

	return response.SuccessWithPagination(
		c,
		dto.NewUserResponses(users),
		response.NewPagination(q.Page, q.Limit, total),
	)
}

// Get ดู user รายคน
//
//	@Summary	ดู user ตาม id
//	@Tags		user
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id	path		string	true	"user id (UUID)"
//	@Success	200	{object}	response.Body{data=dto.UserResponse}
//	@Failure	400	{object}	response.Body	"id ไม่ใช่ UUID"
//	@Failure	401	{object}	response.Body
//	@Failure	404	{object}	response.Body
//	@Router		/api/v1/users/{id} [get]
func (h *UserHandler) Get(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return response.BadRequest(c, "Invalid user id")
	}

	user, err := h.users.GetUserByID(c.Request().Context(), id)
	if err != nil {
		return err
	}

	return response.Success(c, dto.NewUserResponse(*user))
}
