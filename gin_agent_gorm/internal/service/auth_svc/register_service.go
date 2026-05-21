package auth_svc

import (
	"github.com/gin-gonic/gin"

	"gin-biz-web-api/internal/requests/auth_request"
	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/database"
	"gin-biz-web-api/pkg/jwt"
	"gin-biz-web-api/pkg/logger"
)

// RegisterService 封装用户注册业务逻辑。
type RegisterService struct {
}

func NewRegisterService() *RegisterService {
	return &RegisterService{}
}

// CreateUserToken 创建用户并返回 token
func (svc *RegisterService) CreateUserToken(c *gin.Context, request auth_request.SignupUsingEmailRequest) string {

	user := model.User{
		Account:  request.Account,
		Email:    request.Email,
		Password: request.Password,
	}

	err := database.DB.Model(&model.User{}).Select("account", "email", "password").Create(&user).Error
	if err != nil {
		logger.LogErrorIf(err)
		return ""
	}

	if user.BaseModel != nil && user.ID > 0 {
		return jwt.NewJWT().GenerateToken(user.GetStringID())
	} else {
		return ""
	}

}
