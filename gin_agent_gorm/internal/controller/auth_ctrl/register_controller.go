package auth_ctrl

import (
	"github.com/gin-gonic/gin"

	"gin-biz-web-api/internal/requests/auth_request"
	"gin-biz-web-api/internal/service/auth_svc"
	"gin-biz-web-api/pkg/errcode"
	"gin-biz-web-api/pkg/responses"
	"gin-biz-web-api/pkg/validator"
	"gin-biz-web-api/pkg/verifycode"
)

// RegisterController 处理用户注册相关 HTTP 请求。
type RegisterController struct {
}

// SendEmailVerifyCode 发送注册邮箱验证码。
// POST /api/auth/register/email-verify-code
//
// 请求参数:
//   - Email: 邮箱地址
//
// 返回数据:
//   - email: 接收验证码的邮箱
func (ctrl *RegisterController) SendEmailVerifyCode(c *gin.Context) {
	response := responses.New(c)

	request := auth_request.SendRegisterEmailVerifyCodeRequest{}
	if ok := validator.BindAndValidate(c, &request, auth_request.SendRegisterEmailVerifyCode); !ok {
		return
	}

	if err := verifycode.NewVerifyCode().SendEmailVerifyCode(request.Email); err != nil {
		response.ToErrorResponse(errcode.InternalServerError.WithDetails(err.Error()), "邮箱验证码发送失败")
		return
	}

	response.ToResponse(gin.H{
		"email": request.Email,
	})
}

// SignupUsingEmail 使用邮箱进行注册。
// POST /api/auth/register/using-email
//
// 请求参数:
//   - account: 账号
//   - email: 邮箱
//   - password: 密码
//   - password_confirm: 确认密码
//   - verify_code: 验证码
//
// 返回数据:
//   - token: JWT 令牌
func (ctrl *RegisterController) SignupUsingEmail(c *gin.Context) {
	response := responses.New(c)

	request := auth_request.SignupUsingEmailRequest{}
	if ok := validator.BindAndValidate(c, &request, auth_request.SignupUsingEmail); !ok {
		return
	}

	token := auth_svc.NewRegisterService().CreateUserToken(c, request)

	if "" == token {
		response.ToErrorResponse(errcode.DBError)
		return
	}

	response.ToResponse(gin.H{
		"token": token,
	})
}
