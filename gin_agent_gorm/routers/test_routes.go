package routers

import (
	"time"

	"github.com/gin-gonic/gin"

	"gin-biz-web-api/internal/controller"
	"gin-biz-web-api/internal/middleware"
	"gin-biz-web-api/pkg/limiter"
)

// methodTokenBucketLimiters 定义测试接口使用的方法级令牌桶限流规则。
// - "/api/test": 每分钟填充 3 个令牌，每次获取 1 个
// - "abc": 每 5 秒填充 5 个令牌，每次获取 1 个
var methodTokenBucketLimiters = limiter.NewTokenBucketMethodLimiter().AddBuckets(
	limiter.TokenBucketLimiterRule{
		Key:          "/api/test",
		FillInterval: time.Minute,
		Capacity:     3,
		Quantum:      1,
	},
	limiter.TokenBucketLimiterRule{
		Key:          "abc",
		FillInterval: time.Second * 5,
		Capacity:     5,
		Quantum:      1,
	},
)

// registerTestRoutes 注册脚手架自带的测试路由。
// 路由前缀: /api/test
func registerTestRoutes(api *gin.RouterGroup) {
	testGroup := api.Group("/test")

	testCtrl := new(controller.TestController)
	testGroup.GET("", middleware.LimitMethodTokenBucket(methodTokenBucketLimiters), testCtrl.Test) // GET /api/test - 测试接口（限流：每分钟 3 次）
	testGroup.GET("/tt", middleware.LimitMethodTokenBucket(methodTokenBucketLimiters, "abc"), testCtrl.Tt) // GET /api/test/tt - 测试接口（限流：每 5 秒 5 次）
	testGroup.POST("", testCtrl.Test) // POST /api/test - 测试接口（POST 方法）
}
