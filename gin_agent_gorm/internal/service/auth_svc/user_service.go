package auth_svc

import (
	"github.com/gin-gonic/gin"

	"gin-biz-web-api/internal/dao/auth_dao"
	"gin-biz-web-api/model"
)

// UserService 封装用户相关业务逻辑。
type UserService struct {
}

// NewUserService 创建用户服务对象。
func NewUserService() *UserService {
	return &UserService{}
}

// GetUsers 查询所有用户列表。
func (svc *UserService) GetUsers(c *gin.Context) ([]model.User, int64) {
	return auth_dao.NewUserDao().GetUsers()
}
