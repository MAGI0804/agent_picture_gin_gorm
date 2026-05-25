package agent_v2_dao

// AgentV2DAO封装 Agent V2 需要的数据库访问。
type AgentV2DAO struct{}

// NewAgentV2DAO 创建 Agent V2 数据访问对象。
func NewAgentV2DAO() *AgentV2DAO {
	return &AgentV2DAO{}
}
