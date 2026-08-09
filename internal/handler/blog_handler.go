package handler

import (
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/thitiphongD/blog-user-api/internal/dto/request"
	dto "github.com/thitiphongD/blog-user-api/internal/dto/response"
	"github.com/thitiphongD/blog-user-api/internal/middleware"
	"github.com/thitiphongD/blog-user-api/internal/response"
	"github.com/thitiphongD/blog-user-api/internal/service"
)

type BlogHandler struct {
	blogs *service.BlogService
}

func NewBlogHandler(blogs *service.BlogService) *BlogHandler {
	return &BlogHandler{blogs: blogs}
}

func (h *BlogHandler) List(c echo.Context) error {
	var q request.BlogQuery
	if err := c.Bind(&q); err != nil {
		return response.BadRequest(c, "Invalid query parameter")
	}

	if err := q.Normalize(); err != nil {
		return response.BadRequest(c, err.Error())
	}

	blogs, total, err := h.blogs.GetBlogs(c.Request().Context(), q)
	if err != nil {
		return err
	}

	return response.SuccessWithPagination(
		c,
		dto.NewBlogResponses(blogs),
		response.NewPagination(q.Page, q.Limit, total),
	)
}

func (h *BlogHandler) Get(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return response.BadRequest(c, "Invalid blog id")
	}

	blog, err := h.blogs.GetBlogByID(c.Request().Context(), id)
	if err != nil {
		return err
	}

	return response.Success(c, dto.NewBlogResponse(*blog))
}

func (h *BlogHandler) Create(c echo.Context) error {
	userID, err := middleware.UserID(c)
	if err != nil {
		return err
	}

	var req request.CreateBlogRequest
	if err := c.Bind(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	if err := c.Validate(&req); err != nil {
		return err
	}

	blog, err := h.blogs.CreateBlog(c.Request().Context(), userID, req)
	if err != nil {
		return err
	}

	return response.Created(c, "Blog created successfully", dto.NewBlogResponse(*blog))
}

func (h *BlogHandler) Update(c echo.Context) error {
	userID, err := middleware.UserID(c)
	if err != nil {
		return err
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return response.BadRequest(c, "Invalid blog id")
	}

	var req request.UpdateBlogRequest
	if err := c.Bind(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	if err := c.Validate(&req); err != nil {
		return err
	}

	blog, err := h.blogs.UpdateBlog(c.Request().Context(), userID, id, req)
	if err != nil {
		return err
	}

	return response.SuccessWithMessage(c, "Blog updated successfully", dto.NewBlogResponse(*blog))
}

func (h *BlogHandler) Delete(c echo.Context) error {
	userID, err := middleware.UserID(c)
	if err != nil {
		return err
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return response.BadRequest(c, "Invalid blog id")
	}

	if err := h.blogs.DeleteBlog(c.Request().Context(), userID, id); err != nil {
		return err
	}

	// ไม่คืน 204 เพราะจะหลุด envelope กลางไปก้อนเดียว — ทุก response ต้องหน้าตาเดียวกัน
	return response.SuccessWithMessage(c, "Blog deleted successfully", nil)
}
