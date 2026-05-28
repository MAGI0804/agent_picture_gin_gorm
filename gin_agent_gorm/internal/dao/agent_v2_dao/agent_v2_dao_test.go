package agent_v2_dao

import "gin-biz-web-api/model"

type runDAOContract interface {
	FindConversation(userID uint, conversationID uint) (model.Conversation, error)
	CreateMessage(message *model.Message) error
	UpdateMessageAgentRunID(messageID uint, agentRunID uint) error
	CreateRun(run *model.AgentRun) error
	CreateMessageAndRun(message *model.Message, run *model.AgentRun) error
	UpdateRun(runID uint, attrs map[string]interface{}) error
	FindRun(userID uint, runID uint) (model.AgentRun, error)
	FindRunByIdempotencyKey(userID uint, idempotencyKey string) (model.AgentRun, error)
	MarkTimedOutRunningRuns(cutoffUnix int, reason string) (int64, error)
}

type stepDAOContract interface {
	CreateStep(step *model.AgentStep) error
	UpdateStep(stepID uint, attrs map[string]interface{}) error
	FindReusableStep(runID uint, stepKey string, inputHash string) (model.AgentStep, bool, error)
	MaxStepAttempt(runID uint, stepKey string, inputHash string) (int, error)
	CountStepAttempts(runID uint) (int, error)
	CountStepAttemptsByKey(runID uint, stepKey string) (int, error)
	ListSteps(userID uint, runID uint) ([]model.AgentStep, error)
}

type artifactDAOContract interface {
	CreateArtifact(artifact *model.Artifact) error
	FindArtifact(userID uint, artifactID uint) (model.Artifact, error)
	UpdateArtifact(artifactID uint, attrs map[string]interface{}) error
	ListArtifacts(userID uint, conversationID uint) ([]model.Artifact, error)
	CreateArtifactVersion(version *model.ArtifactVersion) error
	UpdateArtifactVersion(artifactID uint, versionID uint, attrs map[string]interface{}) error
	ListArtifactVersions(userID uint, artifactID uint) ([]model.ArtifactVersion, error)
	CreateArtifactFeedback(feedback *model.ArtifactFeedback) error
}

type memoryDAOContract interface {
	CreateMemory(memory *model.ContextMemory) error
	FindMemory(userID uint, memoryID uint) (model.ContextMemory, error)
	ListMemories(filter MemoryFilter) ([]model.ContextMemory, error)
	UpdateMemoryUsage(memoryID uint) error
	UpdateMemory(memoryID uint, attrs map[string]interface{}) error
	SoftDeleteMemory(userID uint, memoryID uint) error
	CreateMemoryEvent(event *model.MemoryEvent) error
}

type ledgerDAOContract interface {
	CreateTaskLedgerItem(item *model.TaskLedgerItem) error
	FindTaskLedgerItem(runID uint, taskKey string) (model.TaskLedgerItem, bool, error)
	UpdateTaskLedgerItem(itemID uint, attrs map[string]interface{}) error
	ListTaskLedgerItems(runID uint) ([]model.TaskLedgerItem, error)
}

type toolDAOContract interface {
	CreateToolInvocation(invocation *model.ToolInvocation) error
	UpdateToolInvocation(invocationID uint, attrs map[string]interface{}) error
	ListToolInvocationsByRun(userID uint, runID uint) ([]model.ToolInvocation, error)
}

type evalDAOContract interface {
	ListReflections(agentName string, limit int) ([]model.AgentReflection, error)
	CreatePromptVersion(version *model.AgentPromptVersion) error
	ListPromptVersions(agentName string, limit int) ([]model.AgentPromptVersion, error)
	FindPromptVersion(versionID uint) (model.AgentPromptVersion, error)
	UpdatePromptVersion(versionID uint, attrs map[string]interface{}) error
	ArchiveActivePromptVersions(agentName string, exceptID uint) error
	CreateReflection(reflection *model.AgentReflection) error
	CreateEvalCase(evalCase *model.EvalCase) error
	ListEvalCases(agentName string, limit int) ([]model.EvalCase, error)
	CreateEvalRun(run *model.EvalRun) error
	ListEvalRuns(agentName string, limit int) ([]model.EvalRun, error)
}

var (
	_ runDAOContract      = (*AgentV2DAO)(nil)
	_ stepDAOContract     = (*AgentV2DAO)(nil)
	_ artifactDAOContract = (*AgentV2DAO)(nil)
	_ memoryDAOContract   = (*AgentV2DAO)(nil)
	_ ledgerDAOContract   = (*AgentV2DAO)(nil)
	_ toolDAOContract     = (*AgentV2DAO)(nil)
	_ evalDAOContract     = (*AgentV2DAO)(nil)
)
