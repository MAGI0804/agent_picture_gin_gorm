package controller

import (
	"github.com/gin-gonic/gin"

	"gin-biz-web-api/pkg/responses"
)

// TestController 处理测试相关 HTTP 请求。
type TestController struct {
}

// Test 基础测试接口。
// curl --location --request GET '0.0.0.0:3000/api/test'
func (ctrl *TestController) Test(c *gin.Context) {
	response := responses.New(c)
	response.ToResponse(nil)
}

// Tt 另一个测试接口。
// curl --location --request GET '0.0.0.0:3000/api/test/tt'
func (ctrl *TestController) Tt(c *gin.Context) {
	response := responses.New(c)
	response.ToResponse(nil)
}
