package routers

import (
	"github.com/gin-gonic/gin"

	"gin-biz-web-api/internal/controller/example_ctrl"
)

// registerExampleRoutes 注册脚手架自带的示例能力路由。
// 路由前缀: /api/example
func registerExampleRoutes(api *gin.RouterGroup) {
	exampleGroup := api.Group("/example")
	{
		captchaCtrl := new(example_ctrl.CaptchaController)
		// 验证码相关路由
		exampleGroup.GET("/show-captcha", captchaCtrl.ShowCaptcha)           // GET /api/example/show-captcha - 显示图形验证码
		exampleGroup.POST("/verify-captcha-code", captchaCtrl.VerifyCaptchaCode) // POST /api/example/verify-captcha-code - 验证验证码

		emailCtrl := new(example_ctrl.EmailController)
		// 邮件相关路由
		exampleGroup.POST("/send-email", emailCtrl.SendEmail)                 // POST /api/example/send-email - 发送简单邮件
		exampleGroup.POST("/send-mailer", emailCtrl.SendMailer)               // POST /api/example/send-mailer - 使用 Mailer 发送邮件
		exampleGroup.POST("/send-email-verify-code", emailCtrl.SendEmailVerifyCode) // POST /api/example/send-email-verify-code - 发送邮箱验证码

		uploadCtrl := new(example_ctrl.UploadController)
		// 文件上传相关路由
		exampleGroup.POST("/upload-file", uploadCtrl.UploadFile)             // POST /api/example/upload-file - 上传文件
		exampleGroup.POST("/upload-avatar", uploadCtrl.UploadAvatar)         // POST /api/example/upload-avatar - 上传头像

		pagerCtrl := new(example_ctrl.PagerController)
		// 分页示例路由
		exampleGroup.GET("/pager", pagerCtrl.Pager)                          // GET /api/example/pager - 分页查询示例

		asyncQueueJobCtrl := new(example_ctrl.AsyncQueueJobController)
		// 异步任务示例路由
		exampleGroup.GET("/job", asyncQueueJobCtrl.Job)                      // GET /api/example/job - 异步队列任务示例
	}
}
