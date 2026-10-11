/*
 * @Description: 即刻短文 HTTP 处理器（[custom] 二开新增）
 */
package essay

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/anzhiyu-c/anheyu-app/pkg/domain/model"
	"github.com/anzhiyu-c/anheyu-app/pkg/response"
	essay_service "github.com/anzhiyu-c/anheyu-app/pkg/service/essay"
	"github.com/gin-gonic/gin"
)

// Handler 封装即刻短文的 HTTP 处理器
type Handler struct {
	svc *essay_service.Service
}

// NewHandler 是 Handler 的构造函数
func NewHandler(svc *essay_service.Service) *Handler {
	return &Handler{svc: svc}
}

// ListPublic 前台短文列表
// @Summary  获取公开的即刻短文列表
// @Tags     即刻短文
// @Produce  json
// @Param    page query int false "页码" default(1)
// @Param    pageSize query int false "每页数量" default(20)
// @Success  200 {object} response.Response{data=model.EssayListResponse}
// @Router   /public/essays [get]
func (h *Handler) ListPublic(c *gin.Context) {
	page, pageSize := pagingFromQuery(c)
	result, err := h.svc.ListPublic(c.Request.Context(), page, pageSize)
	if err != nil {
		h.fail(c, err, "获取短文列表")
		return
	}
	response.Success(c, result, "获取成功")
}

// ListAdmin 后台短文列表（含未公开）
// @Summary  获取全部即刻短文（后台）
// @Tags     即刻短文
// @Security BearerAuth
// @Produce  json
// @Success  200 {object} response.Response{data=model.EssayListResponse}
// @Router   /essays [get]
func (h *Handler) ListAdmin(c *gin.Context) {
	page, pageSize := pagingFromQuery(c)
	result, err := h.svc.ListAdmin(c.Request.Context(), page, pageSize)
	if err != nil {
		h.fail(c, err, "获取短文列表")
		return
	}
	response.Success(c, result, "获取成功")
}

// Get 获取单条短文
// @Summary  获取单条即刻短文（后台）
// @Tags     即刻短文
// @Security BearerAuth
// @Produce  json
// @Param    id path int true "短文ID"
// @Success  200 {object} response.Response{data=model.EssayResponse}
// @Router   /essays/{id} [get]
func (h *Handler) Get(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	result, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		h.fail(c, err, "获取短文")
		return
	}
	response.Success(c, result, "获取成功")
}

// Create 创建短文
// @Summary  创建即刻短文
// @Tags     即刻短文
// @Security BearerAuth
// @Accept   json
// @Produce  json
// @Param    essay body model.EssayRequest true "短文内容"
// @Success  200 {object} response.Response{data=model.EssayResponse}
// @Router   /essays [post]
func (h *Handler) Create(c *gin.Context) {
	var req model.EssayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数格式不正确")
		return
	}
	result, err := h.svc.Create(c.Request.Context(), &req)
	if err != nil {
		h.fail(c, err, "创建短文")
		return
	}
	response.Success(c, result, "创建成功")
}

// Update 更新短文
// @Summary  更新即刻短文
// @Tags     即刻短文
// @Security BearerAuth
// @Accept   json
// @Produce  json
// @Param    id path int true "短文ID"
// @Param    essay body model.EssayRequest true "短文内容"
// @Success  200 {object} response.Response{data=model.EssayResponse}
// @Router   /essays/{id} [put]
func (h *Handler) Update(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req model.EssayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数格式不正确")
		return
	}
	result, err := h.svc.Update(c.Request.Context(), id, &req)
	if err != nil {
		h.fail(c, err, "更新短文")
		return
	}
	response.Success(c, result, "更新成功")
}

// Delete 删除短文
// @Summary  删除即刻短文
// @Tags     即刻短文
// @Security BearerAuth
// @Produce  json
// @Param    id path int true "短文ID"
// @Success  200 {object} response.Response
// @Router   /essays/{id} [delete]
func (h *Handler) Delete(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		h.fail(c, err, "删除短文")
		return
	}
	response.Success(c, nil, "删除成功")
}

func (h *Handler) fail(c *gin.Context, err error, action string) {
	var vErr *essay_service.ValidationError
	switch {
	case errors.As(err, &vErr):
		response.Fail(c, http.StatusBadRequest, vErr.Error())
	case essay_service.IsNotFound(err):
		response.Fail(c, http.StatusNotFound, essay_service.ErrEssayNotFound.Error())
	default:
		log.Printf("[Essay] %s失败: %v", action, err)
		response.Fail(c, http.StatusInternalServerError, action+"失败，请稍后重试")
	}
}

func parseID(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.Fail(c, http.StatusBadRequest, "短文 ID 不合法")
		return 0, false
	}
	return uint(id), true
}

func pagingFromQuery(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	return page, pageSize
}
