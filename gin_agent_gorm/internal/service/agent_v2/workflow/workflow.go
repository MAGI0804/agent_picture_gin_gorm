package workflow

import "gin-biz-web-api/internal/service/agent_v2/domain"

// Workflow 工作流定义，包含多个 Agent 节点
type Workflow struct {
	Name    string
	Version string
	Nodes   []domain.AgentNode
}

// Sequential 创建一个顺序执行的工作流
func Sequential(name string, version string, nodes ...domain.AgentNode) Workflow {
	return Workflow{
		Name:    name,
		Version: version,
		Nodes:   nodes,
	}
}
