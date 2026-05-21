package agent_ctrl

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"gin-biz-web-api/internal/service/agent_svc"
	"gin-biz-web-api/pkg/auth"
	"gin-biz-web-api/pkg/errcode"
	"gin-biz-web-api/pkg/responses"
)

// ModelConfigController 处理全局模型配置相关 HTTP 请求。
// 提供模型配置的增删改查功能。
type ModelConfigController struct {
}

// ListModelConfigs 获取全局模型配置列表。
// GET /api/model-configs
//
// 查询参数:
//   - page: 页码（默认 1）
//   - per_page: 每页数量（默认 20）
//   - is_text_model: 是否文本模型（可选过滤）
//   - is_image_model: 是否图片模型（可选过滤）
//
// 返回数据:
//   - configs: 模型配置列表
//   - total: 总数
func (ctrl *ModelConfigController) ListModelConfigs(c *gin.Context) {
	_ = auth.CurrentUserID(c)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))

	isTextModel := c.Query("is_text_model")
	isImageModel := c.Query("is_image_model")

	configs, total, err := agent_svc.NewAgentService().ListModelConfigs(page, perPage, isTextModel, isImageModel)
	if err != nil {
		responses.New(c).ToErrorResponse(errcode.DBError.WithDetails(err.Error()))
		return
	}

	responses.New(c).ToResponse(gin.H{
		"configs": configs,
		"total":   total,
	})
}

// GetModelConfig 获取单个模型配置详情。
// GET /api/model-configs/:id
//
// 路径参数:
//   - id: 模型配置 ID
//
// 返回数据:
//   - config: 模型配置详情
func (ctrl *ModelConfigController) GetModelConfig(c *gin.Context) {
	_ = auth.CurrentUserID(c)

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		responses.New(c).ToErrorResponse(errcode.BadRequest, "ID 参数错误")
		return
	}

	config, err := agent_svc.NewAgentService().GetModelConfigByID(uint(id))
	if err != nil {
		responses.New(c).ToErrorResponse(errcode.NotFound.WithDetails(err.Error()), "模型配置不存在")
		return
	}

	responses.New(c).ToResponse(gin.H{"config": config})
}

// CreateModelConfig 创建新的模型配置。
// POST /api/model-configs
//
// 请求参数:
//   - model_name: 模型名称（必填）
//   - request_url: API 请求地址（必填）
//   - is_text_model: 是否文本模型（默认 false）
//   - is_image_model: 是否图片模型（默认 false）
//   - support_thinking: 是否支持思考模式（默认 false）
//   - config_info: 额外配置信息（JSON，可选）
//
// 返回数据:
//   - config: 创建的模型配置
func (ctrl *ModelConfigController) CreateModelConfig(c *gin.Context) {
	_ = auth.CurrentUserID(c)

	var request struct {
		ModelName       string                 `json:"model_name" form:"model_name"`
		RequestURL      string                 `json:"request_url" form:"request_url"`
		IsTextModel     bool                   `json:"is_text_model" form:"is_text_model"`
		IsImageModel    bool                   `json:"is_image_model" form:"is_image_model"`
		SupportThinking bool                   `json:"support_thinking" form:"support_thinking"`
		ConfigInfo      map[string]interface{} `json:"config_info" form:"config_info"`
	}

	if err := c.ShouldBind(&request); err != nil {
		responses.New(c).ToErrorResponse(errcode.BadRequest.WithDetails(err.Error()), "请求参数错误")
		return
	}

	if request.ModelName == "" {
		responses.New(c).ToErrorResponse(errcode.BadRequest, "模型名称不能为空")
		return
	}

	if request.RequestURL == "" {
		responses.New(c).ToErrorResponse(errcode.BadRequest, "请求地址不能为空")
		return
	}

	config, err := agent_svc.NewAgentService().CreateModelConfig(request)
	if err != nil {
		responses.New(c).ToErrorResponse(errcode.DBError.WithDetails(err.Error()))
		return
	}

	responses.New(c).ToResponse(gin.H{"config": config})
}

// UpdateModelConfig 更新模型配置。
// PUT /api/model-configs/:id
//
// 路径参数:
//   - id: 模型配置 ID
//
// 请求参数:
//   - model_name: 模型名称
//   - request_url: API 请求地址
//   - is_text_model: 是否文本模型
//   - is_image_model: 是否图片模型
//   - support_thinking: 是否支持思考模式
//   - config_info: 额外配置信息（JSON）
//
// 返回数据:
//   - config: 更新后的模型配置
func (ctrl *ModelConfigController) UpdateModelConfig(c *gin.Context) {
	_ = auth.CurrentUserID(c)

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		responses.New(c).ToErrorResponse(errcode.BadRequest, "ID 参数错误")
		return
	}

	var request struct {
		ModelName       string                 `json:"model_name" form:"model_name"`
		RequestURL      string                 `json:"request_url" form:"request_url"`
		IsTextModel     bool                   `json:"is_text_model" form:"is_text_model"`
		IsImageModel    bool                   `json:"is_image_model" form:"is_image_model"`
		SupportThinking bool                   `json:"support_thinking" form:"support_thinking"`
		ConfigInfo      map[string]interface{} `json:"config_info" form:"config_info"`
	}

	if err := c.ShouldBind(&request); err != nil {
		responses.New(c).ToErrorResponse(errcode.BadRequest.WithDetails(err.Error()), "请求参数错误")
		return
	}

	config, err := agent_svc.NewAgentService().UpdateModelConfig(uint(id), request)
	if err != nil {
		responses.New(c).ToErrorResponse(errcode.DBError.WithDetails(err.Error()))
		return
	}

	responses.New(c).ToResponse(gin.H{"config": config})
}

// DeleteModelConfig 删除模型配置。
// DELETE /api/model-configs/:id
//
// 路径参数:
//   - id: 模型配置 ID
//
// 返回: 成功返回空响应
func (ctrl *ModelConfigController) DeleteModelConfig(c *gin.Context) {
	_ = auth.CurrentUserID(c)

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		responses.New(c).ToErrorResponse(errcode.BadRequest, "ID 参数错误")
		return
	}

	err = agent_svc.NewAgentService().DeleteModelConfig(uint(id))
	if err != nil {
		responses.New(c).ToErrorResponse(errcode.DBError.WithDetails(err.Error()))
		return
	}

	responses.New(c).ToResponse(gin.H{})
}

// ListTextModels 获取所有文本模型配置。
// GET /api/model-configs/text-models
//
// 返回数据:
//   - configs: 文本模型配置列表
func (ctrl *ModelConfigController) ListTextModels(c *gin.Context) {
	_ = auth.CurrentUserID(c)

	configs, err := agent_svc.NewAgentService().ListTextModels()
	if err != nil {
		responses.New(c).ToErrorResponse(errcode.DBError.WithDetails(err.Error()))
		return
	}

	responses.New(c).ToResponse(gin.H{"configs": configs})
}

// ListImageModels 获取所有图片模型配置。
// GET /api/model-configs/image-models
//
// 返回数据:
//   - configs: 图片模型配置列表
func (ctrl *ModelConfigController) ListImageModels(c *gin.Context) {
	_ = auth.CurrentUserID(c)

	configs, err := agent_svc.NewAgentService().ListImageModels()
	if err != nil {
		responses.New(c).ToErrorResponse(errcode.DBError.WithDetails(err.Error()))
		return
	}

	responses.New(c).ToResponse(gin.H{"configs": configs})
}