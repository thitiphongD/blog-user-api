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
