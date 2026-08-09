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

// List รายการ blog — public ไม่ต้อง login
//
//	@Summary	list blog
//	@Tags		blog
//	@Produce	json
//	@Param		page	query		int		false	"หน้า เริ่มที่ 1"	default(1)
//	@Param		limit	query		int		false	"ต่อหน้า สูงสุด 100"	default(20)
//	@Param		search	query		string	false	"ค้นจาก title/content"
//	@Param		sort	query		string	false	"คอลัมน์ที่ใช้เรียง"	Enums(created_at, title)	default(created_at)
//	@Param		order	query		string	false	"ทิศทางการเรียง"	Enums(asc, desc)	default(desc)
//	@Param		user_id	query		string	false	"กรองตามผู้เขียน (UUID)"
//	@Success	200		{object}	response.Body{data=[]dto.BlogResponse}
//	@Failure	400		{object}	response.Body	"sort/order/user_id นอก whitelist"
//	@Router		/api/v1/blogs [get]
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

// Get ดู blog รายอัน — public ไม่ต้อง login
//
//	@Summary	ดู blog ตาม id
//	@Tags		blog
//	@Produce	json
//	@Param		id	path		string	true	"blog id (UUID)"
//	@Success	200	{object}	response.Body{data=dto.BlogResponse}
//	@Failure	400	{object}	response.Body	"id ไม่ใช่ UUID"
//	@Failure	404	{object}	response.Body
//	@Router		/api/v1/blogs/{id} [get]
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

// Create เขียน blog ใหม่ ผู้เขียนคือเจ้าของ token
//
//	@Summary	สร้าง blog
//	@Tags		blog
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		body	body		request.CreateBlogRequest	true	"title กับ content"
//	@Success	201		{object}	response.Body{data=dto.BlogResponse}
//	@Failure	401		{object}	response.Body
//	@Failure	422		{object}	response.Body
//	@Router		/api/v1/blogs [post]
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

// Update แก้ blog แบบ replace เต็ม เจ้าของเท่านั้น
//
//	@Summary	แก้ blog
//	@Tags		blog
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id		path		string						true	"blog id (UUID)"
//	@Param		body	body		request.UpdateBlogRequest	true	"ต้องส่ง title กับ content ครบ"
//	@Success	200		{object}	response.Body{data=dto.BlogResponse}
//	@Failure	401		{object}	response.Body
//	@Failure	403		{object}	response.Body	"ไม่ใช่เจ้าของ"
//	@Failure	404		{object}	response.Body
//	@Failure	422		{object}	response.Body
//	@Router		/api/v1/blogs/{id} [put]
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

// Delete ลบ blog แบบ soft delete เจ้าของเท่านั้น
//
//	@Summary	ลบ blog
//	@Tags		blog
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id	path		string	true	"blog id (UUID)"
//	@Success	200	{object}	response.Body
//	@Failure	401	{object}	response.Body
//	@Failure	403	{object}	response.Body	"ไม่ใช่เจ้าของ"
//	@Failure	404	{object}	response.Body
//	@Router		/api/v1/blogs/{id} [delete]
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
