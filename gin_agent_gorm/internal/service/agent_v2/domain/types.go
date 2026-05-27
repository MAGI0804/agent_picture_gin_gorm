package domain

import "context"

// 运行状态常量
const (
	RunStatusCreated   = "created"
	RunStatusQueued    = "queued"
	RunStatusRunning   = "running"
	RunStatusWaiting   = "waiting_user"
	RunStatusCompleted = "completed"
	RunStatusFailed    = "failed"
	RunStatusCancelled = "cancelled"

	// 步骤状态常量
	StepStatusPending   = "pending"
	StepStatusRunning   = "running"
	StepStatusCompleted = "completed"
	StepStatusFailed    = "failed"
	StepStatusSkipped   = "skipped"
	StepStatusRetrying  = "retrying"
	StepStatusCancelled = "cancelled"
)

// RunState 运行状态，包含一次 Agent 运行的完整上下文
type RunState struct {
	RunID           uint                `json:"run_id"`
	CurrentStepID   uint                `json:"-"`
	UserID          uint                `json:"user_id"`
	ConversationID  uint                `json:"conversation_id"`
	TaskType        string              `json:"task_type"`
	Intent          string              `json:"intent"`
	UserRequest     string              `json:"user_request"`
	Requirements    ImageRequirements   `json:"requirements"`
	MemoryContext   []MemoryItem        `json:"memory_context"`
	Prompts         PromptBundle        `json:"prompts"`
	GeneratedImages []GeneratedImageRef `json:"generated_images"`
	Artifacts       []ArtifactRef       `json:"artifacts"`
	Review          ReviewResult        `json:"review"`
	Budget          RunBudget           `json:"budget"`
	Metadata        map[string]string   `json:"metadata"`
}

// ImageRequirements 图片需求配置
type ImageRequirements struct {
	Subject           string   `json:"subject"`
	Style             string   `json:"style"`
	AspectRatio       string   `json:"aspect_ratio"`
	MustInclude       []string `json:"must_include"`
	MustAvoid         []string `json:"must_avoid"`
	NeedClarification bool     `json:"need_clarification"`
	Questions         []string `json:"questions"`
	Scene             string   `json:"scene"`
	Composition       string   `json:"composition"`
	TextPolicy        string   `json:"text_policy"`
	LayoutHints       []string `json:"layout_hints"`
	TargetUse         string   `json:"target_use"`
}

// MemoryItem 记忆项
type MemoryItem struct {
	ID         uint    `json:"id"`
	Kind       string  `json:"kind"`
	Content    string  `json:"content"`
	Confidence float64 `json:"confidence"`
}

// PromptBundle 提示词包
type PromptBundle struct {
	PositivePrompt       string            `json:"positive_prompt"`
	NegativePrompt       string            `json:"negative_prompt"`
	RenderTextSeparately bool              `json:"render_text_separately"`
	Params               map[string]string `json:"params"`
}

// GeneratedImageRef is the structured handoff from Image Agent to Artifact Agent.
type GeneratedImageRef struct {
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	MimeType   string `json:"mime_type"`
	ObjectKey  string `json:"object_key"`
	PreviewURL string `json:"preview_url"`
	SizeBytes  int64  `json:"size_bytes"`
	Hash       string `json:"hash"`
}

// ArtifactRef 产物引用
type ArtifactRef struct {
	ID         uint   `json:"id"`
	VersionID  uint   `json:"version_id"`
	Kind       string `json:"kind"`
	PreviewURL string `json:"preview_url"`
}

// ReviewResult 审核结果
type ReviewResult struct {
	OverallScore float64  `json:"overall_score"`
	Issues       []string `json:"issues"`
	ShouldRefine bool     `json:"should_refine"`
	Reviewer     string   `json:"reviewer"`
}

// RunBudget 运行预算配置
type RunBudget struct {
	MaxSteps            int `json:"max_steps"`
	MaxImageGenerations int `json:"max_image_generations"`
	MaxToolCalls        int `json:"max_tool_calls"`
	TimeoutSeconds      int `json:"timeout_seconds"`
}

// MemoryWrite 记忆写入操作
type MemoryWrite struct {
	Namespace  string   `json:"namespace"`
	Kind       string   `json:"kind"`
	Content    string   `json:"content"`
	Tags       []string `json:"tags"`
	Confidence float64  `json:"confidence"`
}

// StepResult 步骤执行结果
type StepResult struct {
	Status       string                 `json:"status"`
	Summary      string                 `json:"summary"`
	Output       map[string]interface{} `json:"output"`
	MemoryWrites []MemoryWrite          `json:"memory_writes"`
	Artifacts    []ArtifactRef          `json:"artifacts"`
	Next         string                 `json:"next"`
}

// AgentNode Agent 节点接口，定义了工作流中单个 Agent 的行为
type AgentNode interface {
	Key() string
	Run(ctx context.Context, state RunState) (StepResult, error)
}
