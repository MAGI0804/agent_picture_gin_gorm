package routers

import (
	"github.com/gin-gonic/gin"

	"gin-biz-web-api/internal/controller/auth_ctrl"
	"gin-biz-web-api/internal/middleware"
)

// registerAuthRoutes 注册认证、注册、用户信息相关路由。
// 路由前缀: /api/auth
func registerAuthRoutes(api *gin.RouterGroup) {
	authGroup := api.Group("/auth")
	authGroup.Use(middleware.LimitIP("500-H")) // IP 级别限流：每小时 500 次
	{
		registerCtrl := new(auth_ctrl.RegisterController)
		// 注册相关路由
		authGroup.POST("/register/email-verify-code", middleware.LimitRoute("10-H"), registerCtrl.SendEmailVerifyCode) // POST /api/auth/register/email-verify-code - 发送注册邮箱验证码（每小时 10 次）
		authGroup.POST("/register/using-email", middleware.LimitRoute("30-H"), registerCtrl.SignupUsingEmail)          // POST /api/auth/register/using-email - 使用邮箱注册（每小时 30 次）

		userCtrl := new(auth_ctrl.UserController)
		// 用户相关路由
		authGroup.POST("/login", middleware.LimitRoute("200-H"), userCtrl.Login) // POST /api/auth/login - 用户登录（每小时 200 次）
		authGroup.GET("/user", userCtrl.Index)                                   // GET /api/auth/user - 获取用户列表（分页）
		authGroup.GET("/me", middleware.AuthJWT(), userCtrl.Profile)             // GET /api/auth/me - 获取当前登录用户信息（需 JWT）
	}
}
