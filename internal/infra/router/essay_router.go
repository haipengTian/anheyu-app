/*
 * @Description: 即刻短文路由（[custom] 二开新增）
 * 独立函数注册，不改 Router 结构体和 NewRouter 签名，避免与上游合并冲突。
 */
package router

import (
	"github.com/anzhiyu-c/anheyu-app/internal/app/middleware"
	essay_handler "github.com/anzhiyu-c/anheyu-app/pkg/handler/essay"
	"github.com/gin-gonic/gin"
)

// RegisterEssayRoutes 注册即刻短文的公开接口和后台管理接口
func RegisterEssayRoutes(engine *gin.Engine, h *essay_handler.Handler, mw *middleware.Middleware) {
	api := engine.Group("/api")
	api.Use(NoCacheMiddleware())

	api.GET("/public/essays", h.ListPublic)

	admin := api.Group("/essays").Use(mw.JWTAuth(), mw.AdminAuth())
	{
		admin.GET("", h.ListAdmin)
		admin.GET("/:id", h.Get)
		admin.POST("", h.Create)
		admin.PUT("/:id", h.Update)
		admin.DELETE("/:id", h.Delete)
	}
}
