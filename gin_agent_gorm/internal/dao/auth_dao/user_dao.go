package auth_dao

import (
	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/database"
)

// UserDao 封装用户数据访问操作。
type UserDao struct {
}

// NewUserDao 创建用户数据访问对象。
func NewUserDao() *UserDao {
	return &UserDao{}
}

// GetUsers 查询所有用户列表。
func (d *UserDao) GetUsers() (users []model.User, count int64) {
	database.DB.Where("id >= ?", 0).Find(&users).Count(&count)
	return
}
