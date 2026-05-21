package agent_svc

import (
	"gin-biz-web-api/internal/dao/agent_dao"
	"gin-biz-web-api/model"
)

// AgentService 封装 AI Agent 会话、消息、编排和产物生成业务。
type AgentService struct {
	dao               *agent_dao.AgentDAO // 数据访问对象。
	store             ObjectStore         // 产物对象存储，首版使用本地文件实现。
	userConfigCache   map[uint]model.UserModelConfig
	globalConfigCache map[uint]model.ModelConfig
}

// NewAgentService 创建 AI Agent 业务服务。
func NewAgentService() *AgentService {
	return &AgentService{
		dao:               agent_dao.NewAgentDAO(),
		store:             NewObjectStore(),
		userConfigCache:   map[uint]model.UserModelConfig{},
		globalConfigCache: map[uint]model.ModelConfig{},
	}
}