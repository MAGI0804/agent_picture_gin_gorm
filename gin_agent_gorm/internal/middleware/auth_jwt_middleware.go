// 授权中间件
package middleware

import (
	"sync"
	"time"

	"gin-biz-web-api/constant"

	"github.com/gin-gonic/gin"

	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/database"
	"gin-biz-web-api/pkg/errcode"
	"gin-biz-web-api/pkg/jwt"
	"gin-biz-web-api/pkg/responses"
)

type authUserCacheItem struct {
	user      model.User
	expiresAt time.Time
}

var authUserCache sync.Map

func AuthJWT() gin.HandlerFunc {
	return func(c *gin.Context) {

		response := responses.New(c)

		// 自动获取 token，并解析 token
		claims, err := jwt.NewJWT().ParseToken(c)

		// jwt 解析失败
		if err != nil {
			response.ToErrorResponse(errcode.Unauthorized.WithDetails(err.Error()), err.Error())
			c.Abort() // 终止后续中间件和处理函数的执行
			return
		}

		user, ok := findAuthUser(claims.UserID)
		if !ok {
			response.ToErrorResponse(errcode.Unauthorized, "找不到对应用户")
			c.Abort()
			return
		}

		// 将用户信息存入 gin.context 上下文中，方便后续直接从上下文中拿到用户信息
		c.Set(constant.CurrentUserID, user.GetStringID())
		c.Set(constant.CurrentUserInfo, user)

		c.Next() // 继续执行后续中间件和处理函数
	}
}

func findAuthUser(userID string) (model.User, bool) {
	if cached, ok := authUserCache.Load(userID); ok {
		item, _ := cached.(authUserCacheItem)
		if time.Now().Before(item.expiresAt) && item.user.ID != 0 {
			return item.user, true
		}
		authUserCache.Delete(userID)
	}
	var user model.User
	err := database.DB.
		Select("id", "account", "email", "phone", "nickname", "introduction", "avatar", "created_at", "updated_at").
		First(&user, userID).Error
	if err != nil || user.ID == 0 {
		return user, false
	}
	authUserCache.Store(userID, authUserCacheItem{
		user:      user,
		expiresAt: time.Now().Add(2 * time.Minute),
	})
	return user, true
}
