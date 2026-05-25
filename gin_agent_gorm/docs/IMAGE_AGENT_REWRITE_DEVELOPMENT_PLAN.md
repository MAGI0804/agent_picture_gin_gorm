# 图片 AI Agent 平台重写开发执行手册

生成日期：2026-05-25  
当前分支：`Again`  
适用范围：允许重写当前项目，但目标是把项目重构成可长期演进的图片 AI Agent 平台  
前置参考：[IMAGE_AGENT_DEVELOPMENT_GUIDE.md](./IMAGE_AGENT_DEVELOPMENT_GUIDE.md)

## 1. 最优开发路线

当前项目已经有登录、模型配置、会话、消息、Agent Run、Agent Step、Artifact、Provider、前端工作台等基础。既然已经迁出新分支，可以大胆重写核心 Agent，但不建议把整个项目从零推倒。

最佳路线是：

```text
保留外围能力
  - JWT / 用户 / 权限
  - Gin / GORM / MySQL / Redis / Asynq
  - 模型配置管理
  - 对象存储基础能力
  - Vue + Vite 前端壳

重写核心域
  - Agent Runtime
  - Workflow / DAG
  - Memory Service
  - Tool / Provider Registry
  - Artifact Versioning
  - Vision Review
  - Evolution / Feedback

通过 v2 接口渐进替换旧流程
  - 旧接口短期可用
  - 新接口承载新工作流
  - 前端逐步切到 v2
```

这样做能达到最好的效果：

- 每一阶段都能运行，不会进入长时间不可用状态。
- 可以复用已有登录、模型配置、数据库和前端页面，开发速度更快。
- Agent 核心可以按生产级结构重写，不受旧 `agent_workflow_service.go` 大文件约束。
- 后续接入多模型、多 Agent、视觉审查、记忆和进化时不会继续堆代码。

## 2. 最终目标

目标不是做一个“图片生成 API 调用器”，而是做一个图片 Agent 工作台：

```text
用户输入需求
  -> 需求理解
  -> 记忆检索
  -> Prompt 生成
  -> 图片生成/编辑
  -> 视觉审查
  -> 自动改进
  -> 产物版本化
  -> 用户反馈
  -> 经验沉淀
```

最终系统必须具备：

| 能力 | 目标 |
| --- | --- |
| 可恢复工作流 | 每个 run、step、artifact 都可追踪、可重试、可恢复 |
| 多 Agent 协同 | Planner、Memory、Prompt、Image、Review、Refiner、Artifact 分工明确 |
| 图片产物中心 | 每张图有版本、父子关系、prompt、参数、模型、评分和反馈 |
| 记忆闭环 | 用户偏好、视觉风格、失败经验可被检索和更新 |
| 单 Agent 进化 | 成功和失败轨迹能变成 prompt 模板、规则和经验 |
| 前端可解释 | 用户能看到过程、候选图、版本、错误和反馈入口 |

## 3. 架构决策

### 3.1 项目结构选择

这是一个 HTTP API + 异步任务 + 前端工作台项目，后端应按服务型 Go 项目组织。当前项目已有 Gin/GORM/Cobra/Viper，不必改模块名和入口方式，重写重点放在 `internal` 下的业务边界。

推荐采用“分层但不过度 DDD”的结构：

```text
gin_agent_gorm/
  cmd/                         # 保留现有 cobra/server 入口
  bootstrap/                   # 保留启动、配置、DB、Redis、路由装配
  routers/                     # 保留路由注册，新增 v2 路由组
  internal/
    controller/
      agent_v2_ctrl/           # 新版 Agent HTTP controller
    dao/
      agent_v2_dao/            # 新版仓储实现，封装 GORM
    service/
      agent_v2/
        app/                   # 用例服务，controller 只调用这里
        domain/                # 核心类型，不依赖 Gin/GORM
        runtime/               # Agent Run 执行器、状态机、step runner
        workflow/              # DAG / workflow 定义
        agents/                # 各类 Agent 节点
        memory/                # 记忆检索、写入、排序、冲突处理
        tools/                 # 文本、图片、视觉、OCR、分割工具接口
        artifact/              # 产物、版本、反馈服务
        eval/                  # 评分、反思、进化服务
        event/                 # SSE event 格式和发布
        prompt/                # prompt 模板和结构化输出解析
  model/                       # GORM model，可保留并新增 v2 表
  pkg/                         # 通用基础设施，谨慎新增
```

为什么不直接把所有代码放到 `agent_svc`：

- 旧 `agent_workflow_service.go` 已经承担过多职责。
- 继续堆文件会让多 Agent、记忆、产物版本和评测混在一起。
- 新增 `agent_v2` 可以逐步迁移，不影响旧功能回滚。

### 3.2 依赖注入选择

首版使用手写构造函数注入，不引入 Wire、Fx、Dig。

原因：

- 当前项目 Go 版本和依赖较旧，额外 DI 框架会增加迁移成本。
- 业务边界还在快速变化，手写构造最直观。
- 后续稳定后再考虑 Wire。

示例：

```go
type AppService struct {
    runs      RunRepository
    artifacts ArtifactService
    runtime   *runtime.Executor
    memory    *memory.Service
}

func NewAppService(deps AppDeps) *AppService {
    return &AppService{
        runs:      deps.RunRepository,
        artifacts: deps.ArtifactService,
        runtime:   deps.RuntimeExecutor,
        memory:    deps.MemoryService,
    }
}
```

### 3.3 接口策略

新增 v2 接口，不立刻删除旧接口：

```text
旧接口：/api/conversations/:id/messages
新接口：/api/v2/conversations/:id/runs
```

前端可以先保留旧页面，再新建 v2 工作台页面。等 v2 稳定后，再删除旧流程。

## 4. 开发阶段总览

```text
Phase 0: 基线冻结和运行验证
Phase 1: 数据模型和迁移
Phase 2: Agent Runtime 重写
Phase 3: Tool / Provider Registry
Phase 4: 文生图 MVP 跑通
Phase 5: Artifact 版本化和候选图
Phase 6: Memory Service MVP
Phase 7: Vision Review 和自动 Refine
Phase 8: 前端工作台重写
Phase 9: Feedback / Evolution 闭环
Phase 10: 稳定性、安全、测试和清理旧代码
```

每个阶段都要有可验收结果。不要连续开发多个阶段后才测试。

## 5. Phase 0：基线冻结和运行验证

目标：知道当前项目哪些能跑，哪些不能跑，先把重写边界定清楚。

要做：

1. 确认后端能启动。
2. 确认前端能构建。
3. 确认登录、模型配置、会话列表、旧图片生成流程是否可用。
4. 记录当前 API 返回结构，避免前端切换时混乱。
5. 为旧接口补最小 smoke test 或 Postman/HTTP 示例。

建议命令：

```bash
cd gin_agent_gorm
go test ./...

cd ../frontend
npm run build
```

验收标准：

- 有一份当前系统可用性记录。
- 明确哪些旧能力保留、哪些旧能力废弃。
- 新增文档不破坏旧代码。

## 6. Phase 1：数据模型和迁移

目标：先把后续需要的数据底座建好。

### 6.1 Agent Run 扩展

当前 `agent_runs` 已有基础字段，建议新增：

| 字段 | 用途 |
| --- | --- |
| `workflow_name` | 使用哪个 workflow，例如 `image_generation_v2` |
| `workflow_version` | workflow 版本 |
| `state_json` | 当前 RunState 快照 |
| `budget_json` | 最大步数、最大生成次数、超时、费用限制 |
| `started_at` | 开始执行时间 |
| `completed_at` | 完成时间 |
| `cancelled_at` | 取消时间 |

### 6.2 Agent Step 扩展

建议新增：

| 字段 | 用途 |
| --- | --- |
| `step_key` | 稳定节点 key，例如 `prompt_agent` |
| `attempt` | 第几次尝试 |
| `provider_name` | 调用 provider |
| `model_name` | 调用模型 |
| `duration_ms` | 步骤耗时 |
| `cost_json` | token、图片次数、费用估算 |
| `input_json` | 结构化输入 |
| `output_json` | 结构化输出 |
| `input_hash` | 幂等判断 |
| `output_hash` | 输出追踪 |

### 6.3 新增 artifact version

必须新增：

```sql
CREATE TABLE artifact_versions (
  id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
  artifact_id BIGINT UNSIGNED NOT NULL,
  parent_version_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
  agent_run_id BIGINT UNSIGNED NOT NULL,
  version_no INT NOT NULL,
  operation VARCHAR(64) NOT NULL,
  prompt TEXT,
  negative_prompt TEXT,
  model_provider VARCHAR(128) NOT NULL DEFAULT '',
  model_name VARCHAR(128) NOT NULL DEFAULT '',
  generation_params JSON NULL,
  source_refs JSON NULL,
  quality_scores JSON NULL,
  object_key VARCHAR(512) NOT NULL,
  preview_url VARCHAR(512) NOT NULL DEFAULT '',
  hash VARCHAR(128) NOT NULL DEFAULT '',
  created_at BIGINT NOT NULL,
  updated_at BIGINT NOT NULL
);
```

### 6.4 新增 artifact feedback

```sql
CREATE TABLE artifact_feedback (
  id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
  artifact_id BIGINT UNSIGNED NOT NULL,
  artifact_version_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
  user_id BIGINT UNSIGNED NOT NULL,
  feedback_type VARCHAR(64) NOT NULL,
  rating INT NOT NULL DEFAULT 0,
  comment TEXT,
  created_at BIGINT NOT NULL
);
```

### 6.5 扩展 context memories

新增字段：

```sql
ALTER TABLE context_memories
  ADD COLUMN namespace VARCHAR(64) NOT NULL DEFAULT 'conversation',
  ADD COLUMN source_type VARCHAR(64) NOT NULL DEFAULT '',
  ADD COLUMN source_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
  ADD COLUMN artifact_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
  ADD COLUMN tags JSON NULL,
  ADD COLUMN confidence DECIMAL(5,4) NOT NULL DEFAULT 0.8000,
  ADD COLUMN embedding_id VARCHAR(128) NOT NULL DEFAULT '',
  ADD COLUMN expires_at BIGINT NOT NULL DEFAULT 0,
  ADD COLUMN last_used_at BIGINT NOT NULL DEFAULT 0,
  ADD COLUMN use_count INT NOT NULL DEFAULT 0;
```

### 6.6 新增进化表

```sql
CREATE TABLE agent_prompt_versions (
  id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
  agent_name VARCHAR(128) NOT NULL,
  version VARCHAR(64) NOT NULL,
  prompt_template TEXT NOT NULL,
  changelog TEXT,
  status VARCHAR(32) NOT NULL DEFAULT 'draft',
  metrics JSON NULL,
  created_at BIGINT NOT NULL,
  updated_at BIGINT NOT NULL
);

CREATE TABLE agent_reflections (
  id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
  agent_run_id BIGINT UNSIGNED NOT NULL,
  agent_name VARCHAR(128) NOT NULL,
  failure_type VARCHAR(128) NOT NULL DEFAULT '',
  reflection TEXT NOT NULL,
  action_item TEXT NOT NULL,
  promoted_to_memory TINYINT(1) NOT NULL DEFAULT 0,
  created_at BIGINT NOT NULL,
  updated_at BIGINT NOT NULL
);
```

验收标准：

- GORM model 已新增。
- AutoMigrate 或迁移脚本可创建新表。
- 不影响旧表读取。
- 能写入一个空 run、一个 step、一个 artifact version、一个 feedback。

## 7. Phase 2：Agent Runtime 重写

目标：先把“可恢复、可观测、可重试”的执行器做出来。

### 7.1 核心类型

放在：

```text
internal/service/agent_v2/domain/
```

建议定义：

```go
type RunState struct {
    RunID          uint
    UserID         uint
    ConversationID uint
    TaskType       string
    Intent         string
    UserRequest    string
    Requirements   ImageRequirements
    MemoryContext  []MemoryItem
    Prompts        PromptBundle
    Artifacts      []ArtifactRef
    Review         ReviewResult
    Budget         RunBudget
}

type StepResult struct {
    Status       string
    Summary      string
    Output       map[string]interface{}
    MemoryWrites []MemoryWrite
    Artifacts    []ArtifactRef
    Next         string
}

type AgentNode interface {
    Key() string
    Run(ctx context.Context, state RunState) (StepResult, error)
}
```

### 7.2 状态机

`agent_runs.status` 固定枚举：

```text
created
queued
running
waiting_user
completed
failed
cancelled
```

`agent_steps.status` 固定枚举：

```text
pending
running
completed
failed
skipped
retrying
```

### 7.3 Executor 职责

`runtime.Executor` 只做这些事：

1. 加载 RunState。
2. 找到 workflow。
3. 按节点顺序执行。
4. 每个节点执行前创建 step。
5. 每个节点执行后更新 step。
6. 失败时记录错误和状态。
7. 成功时保存 RunState 快照。
8. 发布 SSE 事件。
9. 判断是否需要等待用户补充。
10. 判断预算是否耗尽。

不应该把 prompt、图片生成、记忆写入逻辑塞进 Executor。

### 7.4 Workflow 定义

首版固定 DAG：

```text
image_generation_v2:
  intent_router
  requirement_agent
  memory_agent
  prompt_agent
  image_generation_agent
  artifact_agent
```

第二版增加：

```text
vision_review_agent
refiner_agent
evolution_agent
```

Workflow 可以先用 Go 代码定义，不急着做可视化配置：

```go
func ImageGenerationWorkflow(reg *registry.Registry) workflow.Workflow {
    return workflow.Sequential("image_generation_v2",
        reg.Must("intent_router"),
        reg.Must("requirement_agent"),
        reg.Must("memory_agent"),
        reg.Must("prompt_agent"),
        reg.Must("image_generation_agent"),
        reg.Must("artifact_agent"),
    )
}
```

验收标准：

- 可以创建一个 v2 run。
- Executor 可以按节点执行 mock workflow。
- 每个节点都有 step 记录。
- SSE 或轮询接口能看到 step 变化。
- 某个节点失败时 run 状态变为 failed，并记录错误。

## 8. Phase 3：Tool / Provider Registry

目标：把模型调用从业务流程里拆出来，变成可替换工具。

### 8.1 工具接口

放在：

```text
internal/service/agent_v2/tools/
```

建议拆成：

```go
type TextProvider interface {
    Chat(ctx context.Context, req ChatRequest) (ChatResult, error)
}

type ImageGenerationProvider interface {
    GenerateImage(ctx context.Context, req ImageGenerationRequest) ([]GeneratedFile, error)
}

type ImageEditProvider interface {
    EditImage(ctx context.Context, req ImageEditRequest) ([]GeneratedFile, error)
}

type VisionProvider interface {
    AnalyzeImage(ctx context.Context, req ImageAnalysisRequest) (ImageAnalysisResult, error)
}

type OCRProvider interface {
    RecognizeText(ctx context.Context, req OCRRequest) (OCRResult, error)
}
```

### 8.2 能力描述

每个工具注册时声明能力：

```go
type ToolCapability struct {
    Name                 string
    Kind                 string
    Provider             string
    Model                string
    MaxPromptChars       int
    SupportsImageInput   bool
    SupportsMask         bool
    SupportsAspectRatios []string
}
```

### 8.3 首版实现顺序

1. 复用当前 `provider.go` 中已有真实模型请求逻辑。
2. 包一层 `ImageGenerationProvider`。
3. 包一层 `TextProvider`。
4. 先做 mock `VisionProvider`，后续替换真实 VLM。
5. 所有 provider 调用必须带 `context.Context`、timeout、输入摘要和错误包装。

验收标准：

- Prompt Agent 只依赖 `TextProvider`。
- Image Agent 只依赖 `ImageGenerationProvider`。
- 不同 provider 的限制能被读取，例如 prompt 最大 750 字符。

## 9. Phase 4：文生图 MVP 跑通

目标：用 v2 新链路完成一次图片生成。

### 9.1 Intent Router Agent

职责：

- 判断任务类型：文本聊天、文生图、图生图、图片编辑、HTML 海报。
- 首版只需要稳定识别 `image_generation` 和 `text_chat`。

输出：

```json
{
  "task_type": "image_generation",
  "intent": "generate_poster",
  "confidence": 0.86
}
```

### 9.2 Requirement Agent

职责：

- 抽取主体、风格、尺寸、用途、必须包含、禁止出现。
- 判断是否需要追问。
- 最多追问 3 个问题。

输出：

```json
{
  "subject": "科技新品发布会主视觉",
  "style": "冷色科技风",
  "aspect_ratio": "16:9",
  "must_include": ["产品轮廓", "留白标题区"],
  "must_avoid": ["杂乱背景", "不可读文字"],
  "need_clarification": false,
  "questions": []
}
```

### 9.3 Memory Agent

首版可以先不接向量库，用 MySQL 条件检索：

- `namespace=user_profile`
- `namespace=visual_style`
- `namespace=tool_experience`
- `user_id`
- `task_type`
- `tags`

输出：

```json
{
  "memories": [
    {
      "kind": "visual_style",
      "content": "用户偏好冷色、干净、科技感、16:9 横图。",
      "confidence": 0.9
    }
  ]
}
```

### 9.4 Prompt Agent

职责：

- 把结构化需求和记忆合并。
- 根据图片模型限制生成最终 prompt。
- 如果目标模型限制 750 字符，则自动压缩。
- 中文海报默认不让图片模型直接生成复杂中文。

输出：

```json
{
  "positive_prompt": "A clean futuristic commercial poster background...",
  "negative_prompt": "blurry text, unreadable letters, watermark...",
  "render_text_separately": true,
  "params": {
    "aspect_ratio": "16:9",
    "candidate_count": 1
  }
}
```

### 9.5 Image Generation Agent

职责：

- 调用图片模型。
- 保存原始返回文件到对象存储。
- 返回 artifact refs。

注意：

- 图片模型错误不要只返回 `500`，要保留 provider 原始错误摘要。
- 如果 provider 限流，run 可以进入 `failed` 或 `retrying`。
- 生成多个候选图时，一个 run 下多个 artifact。

### 9.6 Artifact Agent

职责：

- 写入 `artifacts`。
- 写入 `artifact_versions`。
- 建立 run、step、artifact、version 关系。
- 生成 preview URL。

验收标准：

- 前端或 HTTP 请求创建 v2 run 后能生成图片。
- 数据库能看到 run、step、artifact、artifact_version。
- 产物可预览和下载。
- 每个 step 有清晰 summary。

## 10. Phase 5：Artifact 版本化和候选图

目标：让图片成为平台的一等公民。

要做：

1. 支持一次 run 生成多张候选图。
2. 候选图有 `artifact_group_id`。
3. 用户可以选择最佳图。
4. 用户可以基于某张图继续编辑。
5. 编辑生成新版本，记录 parent version。

建议 artifact 关系：

```text
artifact
  id=100, kind=image, name=科技海报

artifact_versions
  v1: text_to_image, parent=0
  v2: edit_image, parent=v1
  v3: upscale, parent=v2
```

前端要能展示：

- 当前图。
- 候选图列表。
- 版本列表。
- 当前版本生成参数。
- 继续编辑按钮。

验收标准：

- 同一任务生成 3 张候选图。
- 用户选择第 2 张后，后端记录 `artifact_feedback`。
- 基于第 2 张继续编辑时，创建新 version。

## 11. Phase 6：Memory Service MVP

目标：让系统能记住用户偏好、视觉风格和失败经验。

### 11.1 记忆服务接口

```go
type Service interface {
    Retrieve(ctx context.Context, query RetrieveQuery) ([]MemoryItem, error)
    ProposeWrites(ctx context.Context, trace RunTrace) ([]MemoryWrite, error)
    CommitWrites(ctx context.Context, writes []MemoryWrite) error
    MarkUsed(ctx context.Context, memoryIDs []uint) error
}
```

### 11.2 首版记忆写入来源

只从三种来源写长期记忆：

1. 用户明确说“以后都...”。
2. 用户选择、下载、收藏某张图。
3. Review/Evolution Agent 发现高频失败经验。

### 11.3 记忆不要过度自动化

首版规则：

- 用户偏好可以自动写入，但 confidence 不超过 `0.85`。
- 失败反思先进入 `reflection`，不要直接影响生产 prompt。
- 同一 namespace 下冲突记忆要降权旧记忆，不要物理删除。

验收标准：

- 用户说“以后默认 16:9 冷色科技风”，下一次文生图会自动使用。
- 用户选择某张图后，该风格会提高排序权重。
- 用户可以删除或停用某条偏好。

## 12. Phase 7：Vision Review 和自动 Refine

目标：让系统不只是生成图片，还能判断图片好不好。

### 12.1 Vision Review Agent

首版检查：

- 图片是否存在。
- 图片尺寸是否符合要求。
- 是否有明显空白/损坏。
- OCR 是否存在不可读文字。
- VLM caption 是否和需求主题一致。

输出：

```json
{
  "overall_score": 0.78,
  "requirement_match": 0.82,
  "composition_score": 0.76,
  "text_readability": 0.20,
  "issues": [
    {
      "type": "text_unreadable",
      "message": "图片内中文标题不可读，建议改为后期排版。"
    }
  ],
  "should_refine": true
}
```

### 12.2 Refiner Agent

职责：

- 根据 Review 的问题生成改进策略。
- 控制最多重试次数。
- 不能无限循环。

首版策略：

```text
如果 text_readability 低：
  - 后续 prompt 禁止图片模型生成文字
  - 生成无文字底图
  - 交给 HTML/Canvas 层排版

如果 composition_score 低：
  - 增加 safe margin
  - 强调主体居中或留白

如果 requirement_match 低：
  - 强化主体和用途描述
```

验收标准：

- Review 低分时能产生可解释问题。
- 预算允许时自动重试 1 次。
- 自动重试结果和原始结果都有版本记录。

## 13. Phase 8：前端工作台重写

目标：前端从“聊天页面”升级为“图片 Agent 工作台”。

### 13.1 页面结构

推荐：

```text
frontend/src/
  api/
    agentV2.ts
    artifacts.ts
    memories.ts
  components/
    workspace/
      WorkspaceLayout.vue
      ConversationSidebar.vue
      ChatComposer.vue
      MessageList.vue
      RunTimeline.vue
      ArtifactBoard.vue
      ArtifactViewer.vue
      VersionStrip.vue
      FeedbackBar.vue
      ModelSelector.vue
  composables/
    useAgentRun.ts
    useRunEvents.ts
    useArtifacts.ts
    useModelConfig.ts
  views/
    AgentWorkspaceView.vue
```

当前项目没有 Pinia。首版可以先用 composables 管理状态；如果状态继续复杂，再加 Pinia。

### 13.2 工作台核心交互

用户流程：

1. 输入图片需求。
2. 选择任务模式：自动、文生图、图生图、图片编辑、海报。
3. 选择文本模型和图片模型。
4. 点击生成。
5. 中间显示对话和追问。
6. 右侧显示实时步骤。
7. 右侧主区域显示候选图。
8. 用户选择最佳图或继续编辑。
9. 用户评分或给出不满意原因。

### 13.3 前端必须展示的状态

| 状态 | 展示方式 |
| --- | --- |
| `created/queued/running` | 顶部 run 状态 + 时间线 |
| `waiting_user` | 在对话中显示追问表单 |
| `failed` | 显示失败步骤、错误摘要、重试按钮 |
| `completed` | 显示产物、版本、下载、反馈 |

### 13.4 产物面板

产物面板必须支持：

- 图片预览。
- 候选图切换。
- 版本切换。
- 下载。
- 继续编辑。
- 选择最佳。
- 评分。
- 展示 prompt 和参数摘要。

验收标准：

- 不看数据库也能从前端理解 Agent 做了什么。
- 用户可以在一个页面完成生成、选择、编辑、反馈。

## 14. Phase 9：Feedback / Evolution 闭环

目标：让系统逐步变好。

### 14.1 用户反馈

所有反馈写入 `artifact_feedback`：

| 行为 | feedback_type |
| --- | --- |
| 选择最佳 | `selected` |
| 下载 | `downloaded` |
| 收藏 | `favorited` |
| 差评 | `negative` |
| 重新生成 | `regenerate` |
| 继续编辑 | `edit_requested` |

### 14.2 Evolution Agent

首版每天或手动运行，不要每次请求都跑。

输入：

- 最近失败 run。
- 低评分 artifact。
- 用户重新生成次数多的任务。
- 高评分 artifact。

输出：

- `agent_reflections`
- 候选 `context_memories`
- 候选 `agent_prompt_versions`

### 14.3 Prompt 版本升级

不要自动上线新 prompt。流程：

```text
draft
  -> review
  -> active
  -> archived
```

只有人工确认或离线评测通过后，才把版本设为 `active`。

验收标准：

- 能看到本周失败原因 Top 5。
- 能根据高分图片生成候选 prompt 模板。
- Prompt 模板可回滚。

## 15. Phase 10：稳定性、安全、测试和清理

目标：让系统可以长期维护。

### 15.1 后端测试

必须补：

| 测试 | 覆盖 |
| --- | --- |
| Runtime unit test | step 顺序、失败、重试、等待用户 |
| Memory unit test | 检索、排序、写入、冲突 |
| Prompt unit test | 长度限制、中文海报策略、结构化输出解析 |
| Artifact unit test | 版本链、反馈、权限 |
| Provider mock integration | 模型成功/失败/超时 |
| API integration | 创建 run、事件、产物下载 |

### 15.2 前端测试

最少保证：

- `npm run build` 通过。
- Agent Workspace 在空数据、运行中、失败、完成四种状态下不崩。
- 长 prompt、长错误信息、多个候选图不会撑破布局。

### 15.3 安全要求

- Artifact 预览和下载必须校验用户权限。
- 上传图限制大小、MIME、像素。
- Provider API key 不进日志。
- Agent step 不暴露原始敏感推理内容。
- 对象存储 key 不可预测。
- 用户可删除偏好记忆。

### 15.4 清理旧代码

当 v2 稳定后再做：

1. 前端默认入口切到 v2。
2. 旧 `agent_workflow_service.go` 标记 deprecated。
3. 迁移旧 artifact 到新 version 结构。
4. 删除不可达旧 mock 流程。
5. 更新 README 和部署文档。

## 16. 具体开发顺序清单

推荐按这个顺序开工：

### 第 1 批：后端骨架

1. 新建 `internal/service/agent_v2/domain`。
2. 新建 `RunState`、`StepResult`、`AgentNode`。
3. 新建 `runtime.Executor`。
4. 新建 `workflow.Sequential`。
5. 新建 mock agents。
6. 新建 `agent_v2_ctrl`。
7. 新增 `/api/v2/conversations/:id/runs`。
8. 新增 `/api/v2/runs/:id`。
9. 新增 `/api/v2/runs/:id/events`。

### 第 2 批：数据落库

1. 扩展 model。
2. 新增 artifact version model。
3. 新增 feedback model。
4. 新增 prompt version model。
5. 新增 reflection model。
6. 新增 DAO。
7. Executor 写入真实 run/step。

### 第 3 批：图片生成链路

1. 包装当前文本 provider。
2. 包装当前图片 provider。
3. 实现 Requirement Agent。
4. 实现 Prompt Agent。
5. 实现 Image Generation Agent。
6. 实现 Artifact Agent。
7. 通过 v2 接口生成一张图片。

### 第 4 批：前端 v2 页面

1. 新建 `AgentWorkspaceView.vue`。
2. 新建 v2 API client。
3. 接入创建 run。
4. 接入 run events。
5. 接入 artifact list。
6. 展示 timeline。
7. 展示候选图和下载。

### 第 5 批：记忆

1. 扩展 `context_memories`。
2. 实现 Memory Retriever。
3. 实现 Memory Writer。
4. Prompt Agent 使用记忆。
5. 前端增加偏好删除或查看入口。

### 第 6 批：审查和进化

1. 接入 Vision/OCR mock。
2. 实现 Review Agent。
3. 实现 Refiner Agent。
4. 增加 feedback API。
5. 实现 Evolution Agent 手动任务。

## 17. 最小可用版本定义

MVP 只做这些：

- 用户登录。
- 选择模型。
- 输入图片需求。
- v2 工作流执行：Requirement -> Memory -> Prompt -> Image -> Artifact。
- 生成图片并预览。
- 查看 step timeline。
- 下载图片。
- 选择最佳图。
- 写入 artifact feedback。

MVP 暂时不做：

- 完整向量数据库。
- 复杂多 Agent 辩论。
- ComfyUI 节点编排。
- GroundingDINO/SAM 局部编辑。
- 自动上线 prompt 版本。
- 多租户计费。

先把主链路做稳，再加高级能力。

## 18. 最佳效果关键点

如果只选 10 件最重要的事，优先做这些：

1. **RunState 结构化**：所有 Agent 共享同一份状态，避免靠自然语言传来传去。
2. **Step 可观测**：每一步都要有输入摘要、输出摘要、状态、错误、耗时。
3. **Artifact 版本化**：图片不是结果文件，而是可继续编辑的资产。
4. **Provider 能力声明**：不同模型限制不同，Prompt Agent 必须知道限制。
5. **中文文字分层处理**：海报底图和文字排版分离，效果会明显更稳。
6. **用户反馈写库**：选择、下载、重试都是进化数据。
7. **记忆谨慎写入**：只写稳定偏好和可复用经验，避免污染。
8. **Review 先轻后重**：先做 OCR/尺寸/主题匹配，再做复杂视觉检测。
9. **v2 渐进替换**：新链路可跑后再删旧链路。
10. **每阶段可验收**：不要连续重写两周才第一次运行。

## 19. 开发验收表

| 阶段 | 必须通过 |
| --- | --- |
| Phase 0 | 旧系统可用性记录完成 |
| Phase 1 | 新表可迁移，旧功能不破 |
| Phase 2 | mock workflow 可完整执行 |
| Phase 3 | provider registry 可选择文本/图片工具 |
| Phase 4 | v2 文生图生成真实 artifact |
| Phase 5 | 候选图和版本链可用 |
| Phase 6 | 用户偏好可被下一次任务使用 |
| Phase 7 | Review 能发现至少一种图片质量问题 |
| Phase 8 | 前端工作台可完成生成、查看、下载、反馈 |
| Phase 9 | 反馈可转成 reflection 或候选 memory |
| Phase 10 | 后端测试、前端构建、安全校验通过 |

## 20. 不建议做的事

- 不建议一开始引入复杂工作流引擎，先用 Go 代码固定 DAG。
- 不建议把所有 Agent 都写成自由对话，必须结构化输入输出。
- 不建议把记忆写入完全交给模型自动决定。
- 不建议让图片模型直接生成复杂中文海报文字。
- 不建议在没有 artifact version 的情况下做图片编辑。
- 不建议先做 ComfyUI/SAM/GroundingDINO，再做主链路。
- 不建议直接删除旧接口，先用 v2 平行替换。

## 21. 开工第一天应该做什么

第一天只做 6 件事：

1. 确认当前分支干净程度，保留已有文档。
2. 新建 `agent_v2` 目录。
3. 写 `domain.RunState`、`domain.StepResult`、`domain.AgentNode`。
4. 写 `runtime.Executor` 的 mock 版本。
5. 写一个固定 workflow，节点全是 mock。
6. 暴露一个 v2 run API，能创建 run 并写入 step。

第一天结束时，哪怕还不能生成图，也应该能看到：

```text
POST /api/v2/conversations/:id/runs
  -> created run
  -> executed mock steps
  -> saved run state
  -> returned step timeline
```

这就是后续所有真实 Agent 的骨架。

