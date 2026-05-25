# 图片 AI Agent V2 重写开发计划

生成日期：2026-05-25  
适用范围：允许重写当前图片 Agent 核心架构  
参考文档：[IMAGE_AGENT_DEVELOPMENT_GUIDE.md](./IMAGE_AGENT_DEVELOPMENT_GUIDE.md)

## 1. 开发目标

将当前项目从“固定图片生成流程”重写为“可编排、可追溯、可记忆、可评测、可进化”的图片 Agent 平台。

本计划不以兼容旧 `agent_svc` 为目标。旧链路只作为临时回滚路径，新能力全部进入 `agent_v2`。

## 2. 总体阶段

```text
P0 架构冻结与代码边界
P1 数据模型重建
P2 Agent Runtime / DAG / 状态机
P3 Memory Service
P4 Artifact Service
P5 Tool Registry / Provider 拆分
P6 多 Agent 工作流
P7 Observability / Budget / Idempotency
P8 Evolution / Evaluation
P9 Security
P10 Frontend Workspace
P11 API 完整闭环
P12 清理旧代码与验收
```

## 3. P0：架构冻结与代码边界

目标：停止继续扩展旧 `agent_svc`，建立新模块边界。

任务：

1. 保留旧接口 `/api/conversations/:id/messages` 作为兼容入口。
2. 所有新能力进入 `/api/v2`。
3. 新建或整理以下目录：

```text
internal/service/agent_v2/app
internal/service/agent_v2/domain
internal/service/agent_v2/runtime
internal/service/agent_v2/workflow
internal/service/agent_v2/agents
internal/service/agent_v2/memory
internal/service/agent_v2/artifact
internal/service/agent_v2/tools
internal/service/agent_v2/eval
internal/service/agent_v2/security
internal/service/agent_v2/event
internal/service/agent_v2/prompt
internal/dao/agent_v2_dao
internal/controller/agent_v2_ctrl
```

验收：

- `go test ./...` 通过。
- `/api/v2/conversations/:id/runs` 可以创建 mock run。
- `agent_v2/domain` 不依赖 Gin/GORM。

## 4. P1：数据模型重建

目标：先把平台底座建好，避免后续逻辑无处落库。

### 4.1 扩展 `agent_runs`

新增或确认字段：

| 字段 | 说明 |
| --- | --- |
| `workflow_name` | 工作流名称 |
| `workflow_version` | 工作流版本 |
| `state_json` | RunState 快照 |
| `budget_json` | 预算配置 |
| `idempotency_key` | run 创建幂等键 |
| `started_at` | 开始时间 |
| `completed_at` | 完成时间 |
| `cancelled_at` | 取消时间 |

### 4.2 扩展 `agent_steps`

新增或确认字段：

| 字段 | 说明 |
| --- | --- |
| `step_key` | 稳定节点 key |
| `attempt` | 第几次尝试 |
| `provider_name` | provider |
| `model_name` | 模型 |
| `duration_ms` | 耗时 |
| `cost_json` | 成本 |
| `input_json` | 结构化输入 |
| `output_json` | 结构化输出 |
| `input_hash` | 输入 hash |
| `output_hash` | 输出 hash |
| `error_code` | 错误类型 |

### 4.3 扩展 `context_memories`

新增字段：

```text
namespace
scope
source_type
source_id
artifact_id
tags_json
confidence
embedding_id
expires_at
last_used_at
use_count
deleted_at
```

### 4.4 扩展 `artifacts`

新增字段：

```text
parent_artifact_id
artifact_group_id
rank_score
selected_at
visibility
storage_policy
```

### 4.5 新增表

必须新增：

```text
artifact_versions
artifact_relations
artifact_feedback
agent_prompt_versions
agent_reflections
eval_cases
eval_runs
tool_definitions
tool_invocations
memory_events
task_ledger_items
```

验收：

- AutoMigrate 或迁移脚本能创建所有表。
- 所有新表有 GORM model。
- 关键查询按 `user_id`、`conversation_id`、`run_id`、`artifact_id` 建索引。

## 5. P2：Agent Runtime / DAG / 状态机

目标：实现可恢复、可重试、可取消的运行时。

模块：

```text
agent_v2/runtime
  executor.go
  state_store.go
  step_runner.go
  retry_policy.go
  budget_manager.go
  lock_manager.go
  idempotency.go
```

核心接口：

```go
type Node interface {
    Key() string
    Run(ctx context.Context, input NodeInput) (NodeOutput, error)
}

type Workflow interface {
    Name() string
    Version() string
    Nodes() []NodeDefinition
}
```

任务：

1. 定义 RunState。
2. 定义 NodeInput / NodeOutput。
3. 实现 DAG 拓扑排序。
4. 实现依赖检查。
5. 实现失败重试。
6. 实现 run 取消。
7. 实现 waiting_user 暂停。
8. 实现 run state 恢复。
9. 实现 step 事件推送。

验收：

- mock DAG 可以按依赖顺序执行。
- 任一节点失败时记录失败 step。
- 可重试节点按策略重试。
- 同一 idempotency key 不重复创建 run。

## 6. P3：Memory Service

目标：构建分层记忆基础设施。

模块：

```text
agent_v2/memory
  service.go
  retriever.go
  writer.go
  extractor.go
  ranker.go
  conflict_resolver.go
  expirer.go
  permission.go
```

任务：

1. 实现按 namespace 查询。
2. 实现关键词检索。
3. 预留向量检索接口。
4. 实现组合排序。
5. 实现记忆写入提案。
6. 实现冲突降权。
7. 实现过期过滤。
8. 实现用户隔离。
9. 实现记忆删除或停用。

API：

```http
GET    /api/v2/memories
POST   /api/v2/memories/search
PATCH  /api/v2/memories/:id
DELETE /api/v2/memories/:id
```

验收：

- 用户偏好可写入、检索、删除。
- Prompt Agent 能使用检索出的视觉风格记忆。
- 不同用户不能访问彼此记忆。

## 7. P4：Artifact Service

目标：支持版本链、候选图、血缘和回滚。

模块：

```text
agent_v2/artifact
  service.go
  version_service.go
  relation_service.go
  feedback_service.go
  storage_service.go
  permission.go
```

任务：

1. 新增 artifact version 写入。
2. 新增 parent version 关系。
3. 新增 artifact relation。
4. 支持 candidate group。
5. 支持 selected artifact。
6. 支持 feedback。
7. 支持版本列表。
8. 支持下载鉴权。
9. 支持 object key 随机化。

API：

```http
GET  /api/v2/conversations/:id/artifacts
GET  /api/v2/artifacts/:id/versions
POST /api/v2/artifacts/:id/feedback
POST /api/v2/artifacts/:id/select
POST /api/v2/artifacts/:id/edit
GET  /api/v2/artifacts/:id/download
```

验收：

- 同一 run 可生成 3 张候选图。
- 用户选择某张后记录 feedback。
- 基于旧版本编辑会产生新版本链。
- 非 owner 无法下载产物。

## 8. P5：Tool Registry / Provider 拆分

目标：按能力拆分工具调用。

模块：

```text
agent_v2/tools
  registry.go
  text_provider.go
  image_generation_provider.go
  image_edit_provider.go
  vision_provider.go
  ocr_provider.go
  segmentation_provider.go
  safety_provider.go
  invocation_logger.go
```

任务：

1. 抽象 TextProvider。
2. 抽象 ImageGenerationProvider。
3. 抽象 ImageEditProvider。
4. 抽象 VisionProvider。
5. 抽象 OCRProvider。
6. 抽象 SegmentationProvider。
7. 实现 Tool Registry。
8. 记录 tool invocation。
9. 保存模型限制和能力描述。

验收：

- Prompt Agent 可读取图片模型 prompt 长度限制。
- Image Agent 不依赖具体 provider 实现。
- Review Agent 可同时调用 Vision 和 OCR mock。

## 9. P6：多 Agent 工作流

目标：实现标准协作模式。

首批 Agent：

```text
intent_router_agent
requirement_agent
memory_agent
prompt_agent
image_generation_agent
artifact_agent
vision_review_agent
refiner_agent
evolution_agent
```

工作流：

```text
image_generation_v2:
  intent_router
  requirement_agent
  memory_agent
  prompt_agent
  image_generation_agent
  artifact_agent
  vision_review_agent
  evolution_agent
```

任务：

1. 每个 Agent 定义 input/output schema。
2. 每个 Agent 输出标准 `NodeOutput`。
3. Task Ledger 记录任务依赖和完成情况。
4. 支持候选图并行节点。
5. 支持 `waiting_user` 追问节点。

验收：

- 一次图片生成能产生完整 timeline。
- Agent 输出可以被前端稳定展示。
- 低质量图能进入 Review 分支。

## 10. P7：Observability / Budget / Idempotency

目标：生产环境可定位、可控成本、可防重复调用。

任务：

1. 每个 step 记录耗时。
2. 每次 tool 调用记录 provider、model、参数摘要、错误。
3. 记录 token、图片次数、费用估算。
4. 实现 run 级预算。
5. 实现 step 级 idempotency key。
6. 实现 Redis lock。
7. 实现错误码归一化。

验收：

- 同一请求重复提交不会重复调用模型。
- 超预算 run 会失败降级。
- 可以按 run 查到完整成本。

## 11. P8：Evolution / Evaluation

目标：让 Agent 能从反馈和失败中迭代。

模块：

```text
agent_v2/eval
  evaluator.go
  reflection_service.go
  prompt_version_service.go
  eval_case_service.go
  promotion_service.go
```

任务：

1. 写入 artifact feedback。
2. 低分产物生成 reflection draft。
3. 高频失败沉淀 memory draft。
4. 高分产物生成 prompt version draft。
5. 实现 eval case。
6. 实现 prompt active/review/draft/archived。
7. 支持 prompt 版本回滚。

验收：

- 能列出失败原因 Top N。
- 能生成 prompt 草稿版本。
- active prompt 可回滚。

## 12. P9：Security

目标：补齐图片 Agent 的安全边界。

模块：

```text
agent_v2/security
  artifact_guard.go
  upload_policy.go
  signed_url.go
  log_redactor.go
  content_safety.go
```

任务：

1. 下载/预览鉴权。
2. 上传 MIME/大小/像素限制。
3. 上传图片重新编码。
4. object key 随机化。
5. 签名 URL 或鉴权代理。
6. 日志脱敏。
7. 安全审查 provider 接口。

验收：

- 非 owner 访问 artifact 返回拒绝。
- API key 不出现在日志。
- 上传非法文件被拒绝。

## 13. P10：Frontend Workspace

目标：从基础聊天页升级为完整图片 Agent 工作台。

前端结构：

```text
frontend/src/api/agentV2.ts
frontend/src/api/artifacts.ts
frontend/src/api/memories.ts
frontend/src/components/workspace/
frontend/src/views/AgentWorkspaceView.vue
frontend/src/composables/useAgentRun.ts
frontend/src/composables/useRunEvents.ts
frontend/src/composables/useArtifacts.ts
frontend/src/composables/useMemories.ts
```

任务：

1. 新建 V2 工作台页面。
2. 展示 run timeline。
3. 展示候选图。
4. 展示版本历史。
5. 支持选择、下载、重新生成。
6. 支持反馈原因。
7. 支持记忆查看/编辑/删除。
8. 支持用户偏好设置。

验收：

- 用户可完成“输入需求 -> 生成候选图 -> 选择 -> 反馈 -> 下载”。
- 用户可看到并删除记忆。
- 前端构建通过。

## 14. P11：API 完整闭环

目标：统一 v2 API。

接口分组：

```text
/api/v2/runs
/api/v2/artifacts
/api/v2/memories
/api/v2/tools
/api/v2/evaluations
/api/v2/settings
```

验收：

- 前端不再依赖旧图片生成接口。
- 所有接口做 user_id 权限隔离。
- API 返回结构稳定。

## 15. P12：清理旧代码与最终验收

任务：

1. 标记旧 `agent_svc` 图片流程 deprecated。
2. 删除不可达 mock 代码。
3. 更新 README。
4. 更新部署文档。
5. 补齐 API 文档。
6. 补齐测试。

最终验收：

- `go test ./...` 通过。
- `npm run build` 通过。
- v2 工作台完成核心闭环。
- Artifact 权限隔离通过。
- 记忆可管理。
- Agent Run 可观测。
- Prompt 版本可回滚。

## 16. 推荐开发顺序

严格按以下顺序执行：

1. 数据模型和迁移。
2. Runtime DAG 和状态机。
3. Step 可观测字段。
4. Artifact 版本链。
5. Tool Registry。
6. Memory Service。
7. 文生图主链路。
8. Vision Review。
9. Feedback / Evolution。
10. Security Guard。
11. Frontend Workspace。
12. 清理旧代码。

不要先做复杂视觉模型接入。先让数据闭环、权限闭环和可观测闭环成立。

## 17. 详细模块落地清单

本节把 P0 到 P12 拆成文件级开发任务。执行时按模块提交，不要跨多个模块混合改动。

### 17.1 `domain` 核心类型

路径：

```text
gin_agent_gorm/internal/service/agent_v2/domain/
```

需要新增或整理：

| 文件 | 职责 |
| --- | --- |
| `run_state.go` | 定义 `RunState`、`RunBudget`、`RunConstraints` |
| `node.go` | 定义 `AgentNode`、`NodeInput`、`NodeOutput`、`ToolCall` |
| `artifact.go` | 定义 `ArtifactRef`、`ArtifactVersionRef`、`ArtifactRelationRef` |
| `memory.go` | 定义 `MemoryItem`、`MemoryQuery`、`MemoryWrite` |
| `workflow.go` | 定义 workflow、node dependency、状态枚举 |
| `errors.go` | 定义业务错误码，例如预算耗尽、权限拒绝、工具不可用 |

要求：

- `domain` 不允许 import `gin`、`gorm`、`database`、`redis`。
- 所有数组字段初始化为空数组，避免 JSON 返回 `null`。
- 所有跨模块传递的数据必须结构化，避免只传自然语言字符串。

### 17.2 `runtime` 执行器

路径：

```text
gin_agent_gorm/internal/service/agent_v2/runtime/
```

需要实现：

| 文件 | 职责 |
| --- | --- |
| `executor.go` | 按 DAG 推进 run，执行节点，更新状态 |
| `state_store.go` | 读写 `agent_runs.state_json` |
| `step_runner.go` | 单节点执行、step 创建和更新 |
| `retry_policy.go` | 节点重试策略 |
| `budget_manager.go` | token、图片次数、费用、耗时预算 |
| `idempotency.go` | run 和 step 幂等键 |
| `lock_manager.go` | Redis 锁，防止同一 run 或 artifact 并发写 |
| `event_publisher.go` | SSE / WebSocket 事件发布 |

最小接口：

```go
type Executor interface {
    Start(ctx context.Context, runID uint) error
    Resume(ctx context.Context, runID uint) error
    Cancel(ctx context.Context, runID uint, reason string) error
}

type StateStore interface {
    Load(ctx context.Context, runID uint) (domain.RunState, error)
    Save(ctx context.Context, state domain.RunState) error
}
```

运行时必须保证：

- step 开始前先写 `running`。
- step 成功后写 `completed`、`duration_ms`、`output_json`。
- step 失败后写 `failed`、`error_code`、`error_message`。
- 每次节点执行前检查预算。
- 每次外部工具调用前检查幂等键。
- run 进入 `waiting_user` 时必须停止自动推进。

### 17.3 `workflow` 编排定义

路径：

```text
gin_agent_gorm/internal/service/agent_v2/workflow/
```

需要实现：

| 文件 | 职责 |
| --- | --- |
| `definition.go` | workflow 定义结构 |
| `dag.go` | 拓扑排序和依赖校验 |
| `registry.go` | workflow 注册和查询 |
| `image_generation.go` | 文生图 workflow |
| `image_edit.go` | 图像编辑 workflow |
| `poster.go` | 海报生成 workflow |
| `review_refine.go` | Review / Refine 子流程 |

文生图 V2 DAG：

```text
intent_router
  -> requirement_agent
  -> memory_agent
  -> prompt_agent
  -> image_generation_agent
  -> artifact_agent
  -> vision_review_agent
  -> evolution_agent
```

候选图并行 DAG：

```text
prompt_agent
  -> image_generation_agent_candidate_1
  -> image_generation_agent_candidate_2
  -> image_generation_agent_candidate_3
  -> ranker_agent
  -> artifact_agent
```

验收：

- DAG 有循环时启动失败。
- 依赖节点失败时，后续节点按策略 skipped 或 failed。
- workflow 版本写入 `agent_runs.workflow_version`。

### 17.4 `agents` 节点实现

路径：

```text
gin_agent_gorm/internal/service/agent_v2/agents/
```

首批 Agent：

| Agent | 文件 | 输入 | 输出 |
| --- | --- | --- | --- |
| Intent Router | `intent_router.go` | 用户消息、附件 | `task_type`、`intent`、`confidence` |
| Requirement | `requirement_agent.go` | 用户消息、历史、附件 | 结构化需求、追问问题 |
| Memory | `memory_agent.go` | 结构化需求、用户、会话 | 相关记忆列表 |
| Prompt | `prompt_agent.go` | 需求、记忆、工具限制 | prompt bundle |
| Image Generation | `image_generation_agent.go` | prompt bundle、工具配置 | artifact refs |
| Artifact | `artifact_agent.go` | 生成文件、参数 | artifact/version 记录 |
| Vision Review | `vision_review_agent.go` | artifact refs、需求 | 评分和问题 |
| Refiner | `refiner_agent.go` | review 结果、原图 | 二次生成或编辑计划 |
| Evolution | `evolution_agent.go` | run trace、反馈、评分 | reflection draft、memory draft |

每个 Agent 的输出必须符合：

```json
{
  "status": "completed",
  "summary": "简短可展示摘要",
  "tool_calls": [],
  "artifacts": [],
  "memory_writes": [],
  "eval_scores": {},
  "next_step": ""
}
```

禁止：

- Agent 直接操作 GORM。
- Agent 直接拼接 provider HTTP 请求。
- Agent 直接写本地文件。
- Agent 输出只有自然语言，没有结构化字段。

### 17.5 `tools` 工具注册体系

路径：

```text
gin_agent_gorm/internal/service/agent_v2/tools/
```

文件：

| 文件 | 职责 |
| --- | --- |
| `registry.go` | 工具注册、查询、能力匹配 |
| `capability.go` | 工具能力描述 |
| `text_provider.go` | 文本模型接口 |
| `image_generation_provider.go` | 文生图接口 |
| `image_edit_provider.go` | 图片编辑接口 |
| `vision_provider.go` | 图片理解接口 |
| `ocr_provider.go` | OCR 接口 |
| `segmentation_provider.go` | 分割接口 |
| `safety_provider.go` | 安全审查接口 |
| `invocation_logger.go` | 工具调用日志 |

工具能力字段：

```json
{
  "name": "jimeng_text_to_image",
  "kind": "image_generation",
  "provider": "volcengine",
  "model": "jimeng",
  "max_prompt_chars": 750,
  "supported_ratios": ["1:1", "16:9", "9:16"],
  "supports_image_input": false,
  "supports_mask": false,
  "supports_stream": false,
  "cost_policy": {
    "unit": "image",
    "price": 0
  }
}
```

工具调用必须记录：

- run_id
- step_id
- tool_name
- provider_name
- model_name
- request_hash
- duration_ms
- status
- error_code
- cost_json

## 18. 数据库详细设计

字段命名以当前项目风格为准，时间字段继续使用 int 时间戳。JSON 字段若当前 MySQL 版本不稳定，可先使用 `type:text` 保存 JSON 字符串。

### 18.1 `agent_runs`

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `workflow_name` | varchar(128) | workflow 名称 |
| `workflow_version` | varchar(64) | workflow 版本 |
| `state_json` | text | RunState 快照 |
| `budget_json` | text | 预算配置 |
| `idempotency_key` | varchar(128) | run 创建幂等键 |
| `lock_key` | varchar(128) | runtime 锁 key |
| `started_at` | int | 开始时间 |
| `completed_at` | int | 完成时间 |
| `cancelled_at` | int | 取消时间 |

索引：

```text
idx_agent_runs_user_status(user_id, status)
idx_agent_runs_conversation(conversation_id, id)
idx_agent_runs_idempotency(user_id, idempotency_key)
```

### 18.2 `agent_steps`

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `step_key` | varchar(128) | 节点 key |
| `attempt` | int | 重试次数 |
| `provider_name` | varchar(128) | provider |
| `model_name` | varchar(128) | 模型 |
| `duration_ms` | bigint | 耗时 |
| `cost_json` | text | 成本 |
| `input_json` | text | 输入快照 |
| `output_json` | text | 输出快照 |
| `input_hash` | varchar(128) | 输入 hash |
| `output_hash` | varchar(128) | 输出 hash |
| `error_code` | varchar(128) | 错误码 |

索引：

```text
idx_agent_steps_run(agent_run_id, id)
idx_agent_steps_key(agent_run_id, step_key, attempt)
idx_agent_steps_status(status)
```

### 18.3 `context_memories`

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `namespace` | varchar(64) | 会话、用户、视觉、反思等命名空间 |
| `scope` | varchar(128) | 作用域，例如 project、conversation、global |
| `source_type` | varchar(64) | 来源类型 |
| `source_id` | bigint | 来源 ID |
| `artifact_id` | bigint | 关联产物 |
| `tags_json` | text | 标签 |
| `confidence` | decimal | 置信度 |
| `embedding_id` | varchar(128) | 向量索引 ID |
| `expires_at` | int | 过期时间 |
| `last_used_at` | int | 最近使用 |
| `use_count` | int | 使用次数 |
| `deleted_at` | int | 软删除 |

### 18.4 `artifacts`

新增字段：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `parent_artifact_id` | bigint | 父产物 |
| `artifact_group_id` | varchar(128) | 候选图分组 |
| `rank_score` | decimal | 排名分 |
| `selected_at` | int | 用户选择时间 |
| `visibility` | varchar(32) | private、shared、public |
| `storage_policy` | varchar(64) | local、s3、signed |

### 18.5 新增表优先级

第一批必须实现：

```text
artifact_versions
artifact_feedback
agent_prompt_versions
agent_reflections
task_ledger_items
tool_invocations
memory_events
```

第二批实现：

```text
artifact_relations
eval_cases
eval_runs
tool_definitions
```

原因：第一批直接支撑主链路闭环，第二批支撑高级扩展和离线评测。

## 19. API 详细契约

所有 V2 API 返回统一结构：

```json
{
  "code": 200,
  "msg": "success",
  "data": {}
}
```

### 19.1 创建 Run

```http
POST /api/v2/conversations/:id/runs
```

请求：

```json
{
  "content": "生成一张科技感新品发布会海报",
  "task_type": "image_generation",
  "workflow_mode": "auto",
  "idempotency_key": "client-generated-key",
  "attachments": [
    {
      "artifact_id": 12,
      "role": "reference_image"
    }
  ],
  "model_config": {
    "text_model_config_id": 1,
    "image_model_config_id": 2,
    "vision_model_config_id": 3
  },
  "generation_options": {
    "aspect_ratio": "16:9",
    "candidate_count": 3,
    "auto_review": true,
    "auto_refine": true
  }
}
```

响应：

```json
{
  "agent_run": {},
  "state": {},
  "steps": [],
  "artifacts": []
}
```

### 19.2 查询 Run

```http
GET /api/v2/runs/:id
```

响应必须包含：

- run 基本信息。
- RunState。
- step timeline。
- artifact refs。
- budget 使用情况。
- 当前是否等待用户输入。

### 19.3 Run 事件流

```http
GET /api/v2/runs/:id/events
```

事件：

```text
run_started
step_started
tool_call_started
tool_call_completed
step_completed
step_failed
run_waiting_user
run_completed
run_failed
```

### 19.4 Artifact API

```http
GET  /api/v2/conversations/:id/artifacts
GET  /api/v2/artifacts/:id/versions
POST /api/v2/artifacts/:id/feedback
POST /api/v2/artifacts/:id/select
POST /api/v2/artifacts/:id/edit
GET  /api/v2/artifacts/:id/download
```

`feedback` 请求：

```json
{
  "artifact_version_id": 33,
  "feedback_type": "negative",
  "rating": 2,
  "comment": "文字不可读，主体太小"
}
```

### 19.5 Memory API

```http
GET    /api/v2/memories?namespace=visual_style
POST   /api/v2/memories/search
PATCH  /api/v2/memories/:id
DELETE /api/v2/memories/:id
```

`search` 请求：

```json
{
  "query": "科技海报 蓝色 16:9",
  "namespaces": ["user_profile", "visual_style", "tool_experience"],
  "conversation_id": 10,
  "limit": 8
}
```

## 20. 前端详细开发计划

目标页面：

```text
/agent-workspace
```

### 20.1 组件结构

```text
frontend/src/views/AgentWorkspaceView.vue
frontend/src/components/workspace/WorkspaceLayout.vue
frontend/src/components/workspace/ConversationSidebar.vue
frontend/src/components/workspace/ChatComposer.vue
frontend/src/components/workspace/MessageList.vue
frontend/src/components/workspace/RunTimeline.vue
frontend/src/components/workspace/ArtifactBoard.vue
frontend/src/components/workspace/ArtifactViewer.vue
frontend/src/components/workspace/VersionStrip.vue
frontend/src/components/workspace/FeedbackBar.vue
frontend/src/components/workspace/MemoryDrawer.vue
frontend/src/components/workspace/ModelSelector.vue
```

### 20.2 API Client

```text
frontend/src/api/agentV2.ts
frontend/src/api/artifacts.ts
frontend/src/api/memories.ts
frontend/src/api/tools.ts
```

### 20.3 Composables

```text
frontend/src/composables/useAgentRun.ts
frontend/src/composables/useRunEvents.ts
frontend/src/composables/useArtifacts.ts
frontend/src/composables/useMemories.ts
frontend/src/composables/useFeedback.ts
```

### 20.4 必须支持的 UI 状态

| 状态 | 前端表现 |
| --- | --- |
| `created` | 显示任务已创建 |
| `queued` | 显示排队中 |
| `running` | 展示 timeline 和当前 step |
| `waiting_user` | 展示追问输入 |
| `completed` | 展示产物和反馈入口 |
| `failed` | 展示失败 step、错误摘要、重试按钮 |
| `cancelled` | 展示取消原因 |

### 20.5 验收场景

1. 用户创建图片任务。
2. 前端实时显示 step timeline。
3. 生成 3 张候选图。
4. 用户切换候选图。
5. 用户查看版本历史。
6. 用户选择最佳图。
7. 用户提交不满意原因。
8. 用户查看并删除一条记忆。
9. 用户下载产物。

## 21. 测试计划

### 21.1 后端单元测试

| 模块 | 测试 |
| --- | --- |
| runtime | DAG 顺序、失败、重试、取消、waiting_user |
| memory | namespace 查询、排序、冲突、软删除 |
| artifact | 版本创建、父子版本、候选分组、权限 |
| tools | provider 选择、能力限制、调用失败 |
| eval | feedback 生成 reflection、prompt draft |
| security | artifact 鉴权、object key 随机化 |

### 21.2 后端集成测试

必须覆盖：

- 创建 run。
- 执行 mock workflow。
- 执行真实文生图 mock provider。
- 生成 artifact version。
- 写入 feedback。
- 查询 memory。
- 非 owner 访问 artifact 被拒绝。

### 21.3 前端构建和交互测试

必须通过：

```bash
npm run build
```

人工验收：

- 长 prompt 不撑破输入区。
- 长错误信息不撑破 timeline。
- 多候选图布局稳定。
- 空数据状态明确。
- 失败状态可重试。

## 22. 里程碑和交付物

### M1：运行时骨架

交付：

- `RunState`
- `Executor`
- `Workflow Registry`
- mock workflow
- v2 run API

验收：

- 创建 run 后写入 steps。
- SSE 能返回 step 事件。

### M2：数据闭环

交付：

- artifact versions
- feedback
- prompt versions
- reflections
- tool invocations
- memory events

验收：

- 一次 run 可追踪到 step、tool、artifact、feedback。

### M3：文生图主链路

交付：

- Requirement Agent
- Memory Agent
- Prompt Agent
- Image Generation Agent
- Artifact Agent

验收：

- 用户输入需求后生成图片产物。

### M4：Review 和反馈闭环

交付：

- Vision Review Agent
- Feedback API
- Evolution draft

验收：

- 低分图生成 reflection。
- 用户反馈写入并可查询。

### M5：前端工作台

交付：

- v2 workspace
- timeline
- artifact board
- version strip
- memory drawer

验收：

- 前端完成生成、选择、反馈、下载、记忆管理。

## 23. 开发风险和约束

| 风险 | 控制措施 |
| --- | --- |
| 一次性重写过大 | 按 M1 到 M5 小步交付 |
| 记忆污染 | 所有长期记忆先 draft 或低 confidence |
| 成本失控 | run budget 和 step idempotency 先实现 |
| 权限漏洞 | artifact service 统一鉴权，不允许绕过 |
| provider 耦合 | 所有模型调用必须经过 Tool Registry |
| 前端状态复杂 | 先用 composables，状态失控后再引入 Pinia |
| 数据表频繁变更 | 先补最小可用字段，JSON 保存扩展字段 |

## 24. 代码提交建议

建议按以下粒度提交：

1. `agent_v2 domain and runtime skeleton`
2. `agent_v2 database models and migrations`
3. `artifact versioning service`
4. `tool registry and provider interfaces`
5. `memory service mvp`
6. `image generation workflow`
7. `vision review and feedback`
8. `frontend v2 workspace`
9. `security guards and tests`

每个提交必须满足：

- `go test ./...` 通过。
- 涉及前端时 `npm run build` 通过。
- 新 API 有请求响应示例。
- 新数据表有权限隔离说明。

## 25. 第一轮开发任务拆解

第一轮只做平台骨架和数据闭环，不接复杂真实视觉能力。

### 25.1 后端任务

1. 补齐所有 model。
2. AutoMigrate 注册新表。
3. 新增 `task_ledger_items`。
4. 新增 `tool_invocations`。
5. 新增 `memory_events`。
6. Runtime 支持 DAG 节点依赖。
7. Runtime 支持 idempotency key。
8. Runtime 支持 budget 检查。
9. Artifact Service 支持 version 写入。
10. Memory Service 支持 namespace 查询。

### 25.2 前端任务

1. 新建 `AgentWorkspaceView.vue`。
2. 新建 `agentV2.ts`。
3. 新建 `RunTimeline.vue`。
4. 新建 `ArtifactBoard.vue`。
5. 新建 `VersionStrip.vue`。
6. 新建 `MemoryDrawer.vue`。

### 25.3 验收命令

```bash
cd gin_agent_gorm
go test ./...

cd ../frontend
npm run build
```

### 25.4 第一轮完成标准

- 可以创建 v2 run。
- run 有 DAG step timeline。
- step 有耗时、输入输出 hash、状态。
- artifact version 可写入。
- feedback 可写入。
- memory 可查询。
- 前端能展示 run timeline 和 artifact 占位面板。
