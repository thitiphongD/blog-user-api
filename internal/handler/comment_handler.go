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

type CommentHandler struct {
	comments *service.CommentService
}

func NewCommentHandler(comments *service.CommentService) *CommentHandler {
	return &CommentHandler{comments: comments}
}

// List คอมเมนต์ของ blog หนึ่งอัน เรียงเก่าไปใหม่ — public เหมือนตัว blog เอง
//
//	@Summary	list comment ของ blog
//	@Tags		comment
//	@Produce	json
//	@Param		id		path		string	true	"blog id (UUID)"
//	@Param		page	query		int		false	"หน้า เริ่มที่ 1"	default(1)
//	@Param		limit	query		int		false	"ต่อหน้า สูงสุด 100"	default(20)
//	@Success	200		{object}	response.Body{data=[]dto.CommentResponse}
//	@Failure	400		{object}	response.Body	"id ไม่ใช่ UUID"
//	@Failure	404		{object}	response.Body	"ไม่มี blog นี้"
//	@Router		/api/v1/blogs/{id}/comments [get]
func (h *CommentHandler) List(c echo.Context) error {
	blogID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return response.BadRequest(c, "Invalid blog id")
	}

	var q request.ListQuery
	if err := c.Bind(&q); err != nil {
		return response.BadRequest(c, "Invalid query parameter")
	}
	q.Normalize()

	comments, total, err := h.comments.GetComments(c.Request().Context(), blogID, q)
	if err != nil {
		return err
	}

	return response.SuccessWithPagination(
		c,
		dto.NewCommentResponses(comments),
		response.NewPagination(q.Page, q.Limit, total),
	)
}

// Create เขียนคอมเมนต์ใต้ blog — ผู้เขียนมาจาก token
//
//	@Summary	สร้าง comment
//	@Tags		comment
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id		path		string							true	"blog id (UUID)"
//	@Param		body	body		request.CreateCommentRequest	true	"เนื้อหาคอมเมนต์"
//	@Success	201		{object}	response.Body{data=dto.CommentResponse}
//	@Failure	401		{object}	response.Body
//	@Failure	404		{object}	response.Body	"ไม่มี blog นี้"
//	@Failure	422		{object}	response.Body
//	@Router		/api/v1/blogs/{id}/comments [post]
func (h *CommentHandler) Create(c echo.Context) error {
	userID, err := middleware.UserID(c)
	if err != nil {
		return err
	}

	blogID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return response.BadRequest(c, "Invalid blog id")
	}

	var req request.CreateCommentRequest
	if err := c.Bind(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	if err := c.Validate(&req); err != nil {
		return err
	}

	comment, err := h.comments.CreateComment(c.Request().Context(), userID, blogID, req)
	if err != nil {
		return err
	}

	return response.Created(c, "Comment created successfully", dto.NewCommentResponse(*comment))
}

// Update แก้คอมเมนต์ตัวเอง — เจ้าของคอมเมนต์เท่านั้น
//
//	@Summary	แก้ comment
//	@Tags		comment
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id		path		string							true	"comment id (UUID)"
//	@Param		body	body		request.UpdateCommentRequest	true	"เนื้อหาใหม่"
//	@Success	200		{object}	response.Body{data=dto.CommentResponse}
//	@Failure	401		{object}	response.Body
//	@Failure	403		{object}	response.Body	"ไม่ใช่เจ้าของ"
//	@Failure	404		{object}	response.Body
//	@Failure	422		{object}	response.Body
//	@Router		/api/v1/comments/{id} [put]
func (h *CommentHandler) Update(c echo.Context) error {
	userID, err := middleware.UserID(c)
	if err != nil {
		return err
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return response.BadRequest(c, "Invalid comment id")
	}

	var req request.UpdateCommentRequest
	if err := c.Bind(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	if err := c.Validate(&req); err != nil {
		return err
	}

	comment, err := h.comments.UpdateComment(c.Request().Context(), userID, id, req)
	if err != nil {
		return err
	}

	return response.SuccessWithMessage(c, "Comment updated successfully", dto.NewCommentResponse(*comment))
}

// Delete ลบคอมเมนต์ตัวเอง (soft delete) — เจ้าของคอมเมนต์เท่านั้น
//
//	@Summary	ลบ comment
//	@Tags		comment
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id	path		string	true	"comment id (UUID)"
//	@Success	200	{object}	response.Body
//	@Failure	401	{object}	response.Body
//	@Failure	403	{object}	response.Body	"ไม่ใช่เจ้าของ"
//	@Failure	404	{object}	response.Body
//	@Router		/api/v1/comments/{id} [delete]
func (h *CommentHandler) Delete(c echo.Context) error {
	userID, err := middleware.UserID(c)
	if err != nil {
		return err
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return response.BadRequest(c, "Invalid comment id")
	}

	if err := h.comments.DeleteComment(c.Request().Context(), userID, id); err != nil {
		return err
	}

	return response.SuccessWithMessage(c, "Comment deleted successfully", nil)
}
