package auth_ctrl

import (
	"github.com/gin-gonic/gin"

	"gin-biz-web-api/internal/requests/agent_request"
	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/auth"
	"gin-biz-web-api/pkg/database"
	"gin-biz-web-api/pkg/errcode"
	"gin-biz-web-api/pkg/hash"
	"gin-biz-web-api/pkg/jwt"
	"gin-biz-web-api/pkg/paginator"
	"gin-biz-web-api/pkg/responses"
)

// UserController 处理用户登录和用户信息相关 HTTP 请求。
type UserController struct {
}

// Index 获取用户列表（分页）。
// GET /api/auth/user?page=3&per_page=2&order_by=id,desc|created_at,asc
//
// 查询参数:
//   - page: 页码（默认 1）
//   - per_page: 每页数量（默认 10）
//   - order_by: 排序字段（格式: 字段名,排序方向）
//
// 返回数据:
//   - users: 用户列表
//   - paginate: 分页信息
func (ctrl *UserController) Index(c *gin.Context) {
	response := responses.New(c)

	var users []model.User
	query := database.DB.Model(model.User{}).Where("id >= ?", 3)
	paginate := paginator.Paginate(c, query, &users, 3)

	if len(users) == 0 {
		response.ToErrorResponse(errcode.NotFound.Msgf("用户"))
		return
	}

	response.ToResponse(gin.H{
		"users":    users,
		"paginate": paginate,
	})
}

// Profile 获取当前登录用户的个人信息。
// GET /api/auth/me
//
// 要求: 需携带 JWT token
//
// 返回数据: 当前登录用户对象
func (ctrl *UserController) Profile(c *gin.Context) {
	profile := auth.CurrentUser(c)
	responses.New(c).ToResponse(profile)
}

// Login 用户登录，支持账号或邮箱登录。
// POST /api/auth/login
//
// 请求参数:
//   - Account: 账号（与 Email 二选一）
//   - Email: 邮箱（与 Account 二选一）
//   - Password: 密码
//
// 返回数据:
//   - token: JWT 令牌
//   - user: 用户信息
func (ctrl *UserController) Login(c *gin.Context) {
	response := responses.New(c)

	var request agent_request.LoginRequest
	if err := c.ShouldBind(&request); err != nil {
		response.ToErrorResponse(errcode.BadRequest.WithDetails(err.Error()), "请求参数错误")
		return
	}

	var user model.User
	query := database.DB.Model(&model.User{})
	if request.Account != "" {
		query = query.Where("account = ?", request.Account)
	} else if request.Email != "" {
		query = query.Where("email = ?", request.Email)
	} else {
		response.ToErrorResponse(errcode.BadRequest, "账号或邮箱不能为空")
		return
	}

	if err := query.First(&user).Error; err != nil {
		response.ToErrorResponse(errcode.Unauthorized, "账号或密码错误")
		return
	}
	if !hash.BcryptCheck(request.Password, user.Password) {
		response.ToErrorResponse(errcode.Unauthorized, "账号或密码错误")
		return
	}

	response.ToResponse(gin.H{
		"token": jwt.NewJWT().GenerateToken(user.GetStringID()),
		"user":  user,
	})
}
