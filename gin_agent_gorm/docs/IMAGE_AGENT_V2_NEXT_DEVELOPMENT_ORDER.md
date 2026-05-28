# 图片 AI Agent V2 后续开发顺序

生成日期：2026-05-27

## 1. 结论

当前项目已经完成 V2 文生图主链路的后端闭环：Run、Step、Workflow、Tool Registry、Artifact Version、Memory Proposal、鉴权预览、异步 Run 第一版、前端 `/workspace` 第二批能力都已经落地。后续不应再扩大旧 `agent_svc`，也不应继续堆 mock；下一阶段应优先把 V2 做成可恢复、可观测、可重试、可验收的生产链路。

开发顺序应按下面的依赖推进：

```text
真实验收基线
  -> 持久异步队列和恢复重试
  -> 可观测性和任务账本
  -> 结构化需求 / Prompt 升级
  -> 候选图并行、逐张 Review、Rank
  -> OCR / 版面 Review
  -> 自动 Refine 和版本链
  -> Memory 语义去重与冲突处理
  -> 图生图 / 图片编辑
  -> 海报文字分层渲染
  -> Evolution / Eval / Prompt 版本治理
  -> 安全与存储增强
  -> 前端组件化、默认入口迁移、旧代码收敛
```

这份文档基于以下来源整理：

- `docs/IMAGE_AGENT_V2_PROGRESS.md`
- `docs/IMAGE_AGENT_DEVELOPMENT_GUIDE.md`
- `docs/IMAGE_AGENT_CURRENT_TO_V2_DEVELOPMENT_PLAN.md`
- `docs/IMAGE_AGENT_REWRITE_DEVELOPMENT_PLAN.md`
- `docs/IMAGE_AGENT_V2_ASYNC_RUN_DESIGN.md`
- 当前后端 `internal/service/agent_v2`、`internal/dao/agent_v2_dao`、`internal/controller/agent_v2_ctrl`
- 当前前端 `frontend/src/views/AgentWorkspaceV2View.vue`、`frontend/src/api.ts`

## 2. 当前项目事实

已完成：

- V2 主链路：`intent_router -> requirement_agent -> memory_agent -> prompt_agent -> image_generation_agent -> artifact_agent -> vision_review_agent`。
- Google Imagen 后端 E2E 已通过，真实 artifact/version、preview/download、select、feedback、quality_scores 已验证。
- Google Vision Review 后端已接入，存在 vision 配置时走真实 review，否则回退 mock。
- `/api/v2/conversations/:id/runs/async` 已从进程内 goroutine 升级为 Asynq 持久队列第一版。
- `/api/v2/runs/:id/cancel` 已接入，runtime 每步前后检查 cancelled。
- `/workspace` 已支持异步创建、轮询、取消、artifact board、version、preview、download、feedback、select、Memory 查询/删除/确认、Review/Eval 展示。
- `task_ledger_items`、`tool_invocations` 表、DAO 和主链路写入已接入，`GET /runs/:id` 会返回 step/ledger/tool invocation 三类追踪数据。
- 项目已有 Asynq/Redis 基础设施：`pkg/job`、`bootstrap/queue_job.go`、`job/foo_job.go`。

主要未完成：

- 前端 `/workspace` 真实模型手工冒烟仍待执行。
- 异步 Run 已接入 Asynq 持久队列第一版，worker 可抢占 `queued/failed -> running`；step 幂等恢复、provider 临时错误重试、预算检查和 running 超时失败方法已完成。
- `idempotency_key` 已有 `(user_id, idempotency_key_unique)` 复合唯一索引策略，空 key 继续允许多次普通提交。
- `task_ledger_items`、`tool_invocations` 已接入 runtime/tool 调用路径。
- `GET /runs/:id/events?cursor=N` 已支持稳定 cursor 轮询；真正长连接增量推送仍未做。
- Requirement / Prompt 已接入文本 provider 生成结构化 JSON，并带 schema 校验、规则 fallback 和 issue 记录；追问恢复第一版已接入。
- 候选图 group、补齐生成、逐张 review 和 Ranker 精排第一版已完成；真实外部 E2E 稳定性仍需持续观察。
- OCR、版面检测、中文文字可读性检查未完成。
- Refiner Agent、自动二次生成、retry budget、版本 parent 链路未闭环。
- Memory 只有同 scope proposal 合并和自动晋级，语义去重、稳定记忆冲突降权和 conflict resolver 未完成。
- 图生图、上传图、mask、Segmentation、Image Edit Agent 未完成。
- 海报 / 品牌图的 HTML/Canvas/SVG 文字分层渲染未完成。
- Evolution 聚合、eval_cases/eval_runs、prompt promote/rollback 未完成。
- 上传限制、签名 URL、object key 随机策略、安全审查 provider 未系统化。
- 旧前端和旧接口仍保留，主入口迁移和旧代码清理应放在 V2 稳定后。

## 3. 风险热点

Git 历史显示项目还很新，31 个提交集中在 2026-05。当前高变动文件包括：

- `gin_agent_gorm/docs/IMAGE_AGENT_V2_PROGRESS.md`
- `frontend/src/styles.css`
- `frontend/src/views/ChatView.vue`
- `frontend/src/api.ts`
- `frontend/src/types.ts`
- `gin_agent_gorm/internal/service/agent_v2/app/service.go`
- `gin_agent_gorm/internal/service/agent_v2/runtime/executor.go`
- `gin_agent_gorm/internal/service/agent_svc/provider.go`

开发时需要特别控制两类风险：

- 不要继续扩大旧 `agent_svc` 和旧 `ChatView.vue` 的职责。
- `app/service.go` 和 `runtime/executor.go` 已经成为 V2 编排热点，后续应逐步把队列、事件、重试、tool invocation、ledger 拆到独立模块。

## 4. 后续开发顺序

### Task 1：完成真实前端冒烟和 Vision E2E 复验

说明：先确认现有能力真实可用，再继续开发。当前后端 Google Imagen E2E 已通过，但进度文档明确 `/workspace` 前端冒烟未跑，真实 Vision E2E 也需要在代理/网络在线时复验。

要做：

- 启动后端、前端和代理。
- 使用同一组 Google 文本、图片、Vision 配置，从 `/workspace` 发起一次真实生成。
- 验证 artifact board、preview、download、feedback、select、Review/Eval 面板。
- 在代理/网络在线时重跑 `googlee2e`，确认真实 Vision review 写入 `artifact_versions.quality_scores`。
- 把验收结果补回 `IMAGE_AGENT_V2_PROGRESS.md`。

验收：

- `go test -tags googlee2e ./internal/service/agent_v2/app -run TestGoogleModelEndToEnd -v` 通过。
- `npm run build` 通过。
- `/workspace` 手工生成完成，页面显示图片、版本、质量分和反馈入口。
- 进度文档记录本次真实验收的 run、artifact、version、provider、model。

依赖：无。

预计文件：

- `gin_agent_gorm/docs/IMAGE_AGENT_V2_PROGRESS.md`
- 必要时补充测试或前端小修。

### Task 2：把异步 Run 从 goroutine 升级为 Asynq 持久队列

说明：真实图片模型耗时长，进程内 goroutine 无法承受进程重启、并发抢占和失败重试。项目已有 Asynq，下一步应优先复用现有 `pkg/job`。

当前状态：已完成第一版（2026-05-27）。`CreateRunAsync` 已改为投递 `agent_v2:run` Asynq 任务，worker 从 DB 读取 run/state/model config 后抢占执行；完整恢复、错误分类、预算增强和增量事件流继续进入 Task 3/4。

要做：

- 新增 `agent_v2_run` Asynq task type 和 payload：`run_id`、`user_id`、`conversation_id`。
- `/runs/async` 只负责创建 message/run、标记 `queued`、投递队列。
- worker 从 DB 读取 run、message、model config，重新装配 workflow 后执行。
- worker 抢占使用条件更新：只允许 `queued -> running`。
- provider 临时错误最多重试 2 次，业务错误直接 failed。
- worker 崩溃后依赖 Asynq 重投递；另补 running 超时扫描设计或实现。

验收：

- `POST /api/v2/conversations/:id/runs/async` 在 1 秒内返回 queued。
- 队列 worker 可完成现有 `image_generation_v2` workflow。
- 停止 HTTP 请求后 worker 仍能继续完成 run。
- 同一个 run 不会被两个 worker 同时推进。
- `go test ./internal/service/agent_v2/app ./internal/service/agent_v2/runtime ./routers -count=1` 通过。

依赖：Task 1。

预计文件：

- `gin_agent_gorm/job/agent_v2_run_job.go`
- `gin_agent_gorm/bootstrap/queue_job.go`
- `gin_agent_gorm/internal/service/agent_v2/app/service.go`
- `gin_agent_gorm/internal/dao/agent_v2_dao/run_dao.go`
- `gin_agent_gorm/docs/IMAGE_AGENT_V2_ASYNC_RUN_DESIGN.md`
- `gin_agent_gorm/docs/IMAGE_AGENT_V2_PROGRESS.md`

### Task 3：补齐 Run 幂等、恢复、重试和预算

说明：异步队列落地后，需要保证重复提交、worker 失败、provider 临时错误和预算耗尽都能被明确处理。

当前状态：已完成第一版（2026-05-27）。`agent_runs` 新增 `(user_id, idempotency_key_unique)` 复合唯一索引策略，空 idempotency key 继续允许多次普通提交；message/run 创建改为事务；Runtime 会复用同 input hash 的 completed step，provider timeout/rate limit/网络错误最多 2 次重试并记录 attempt/retrying/error_code；已增加 `max_tool_calls`、图片生成次数和总耗时预算检查；新增 running 超时批量失败方法 `FailTimedOutRuns`/`MarkTimedOutRunningRuns`。

要做：

- 给 `agent_runs` 增加 `(user_id, idempotency_key)` 唯一约束或等价迁移策略。
- runtime 支持 step attempt 递增和 retrying 状态。
- 对 provider timeout、rate limit、网络错误做可重试分类。
- 增加图片生成次数预算、tool call 预算、总耗时预算。
- 支持 `Resume(runID)` 或等价 worker 恢复入口。
- 补 running 超时 run 的恢复或失败标记策略。

验收：

- 同一 idempotency key 不会创建重复 run。
- provider 临时失败会重试并记录 attempt。
- 超预算时 run 进入 failed，错误摘要可读。
- 已完成 step 不会在恢复时无意义重复调用外部 provider。
- runtime 单测覆盖成功、失败、重试、取消、预算耗尽。

依赖：Task 2。

预计文件：

- `gin_agent_gorm/model/agent_run_model.go`
- `gin_agent_gorm/bootstrap/ai_agent.go`
- `gin_agent_gorm/internal/service/agent_v2/runtime`
- `gin_agent_gorm/internal/service/agent_v2/app`
- `gin_agent_gorm/internal/dao/agent_v2_dao`

### Task 4：接入 Task Ledger、Tool Invocation 和增量事件

说明：当前 step timeline 可看，但工具调用、成本、任务账本没有贯通。生产排障必须能回答“哪一步、调用了哪个工具、耗时多少、花费多少、为什么失败”。

当前状态：已完成第一版（2026-05-27）。Runtime 执行每个 node 会写入/更新 `task_ledger_items`；Tool provider 调用通过 `InstrumentTool` 写 `tool_invocations`，记录 provider/model、输入摘要、输出摘要、duration、cost policy、error_code/error_message；`GET /api/v2/runs/:id` 返回 step、ledger、tool invocation 三类追踪数据；`GET /api/v2/runs/:id/events?cursor=N` 支持稳定轮询 cursor，SSE 仍兼容输出规范化事件；前端 `/workspace` timeline 展示 attempt、duration、provider/model 和可读错误摘要。

要做：

- Runtime 执行每个 node 时写 `task_ledger_items`。
- Tool Registry 或 Provider Adapter 调用前后写 `tool_invocations`。
- 将 provider/model、duration、input/output 摘要、error_code、cost_json 写入 step 或 invocation。
- `GET /runs/:id/events` 从一次性 SSE 改为可持续增量事件，或先实现稳定轮询 cursor。
- 前端 timeline 展示 provider、耗时、重试次数、错误摘要。

验收：

- 每次真实 run 至少有 step、ledger、tool_invocation 三类追踪记录。
- 外部 provider 错误不会只显示 500，而有可读错误摘要。
- 页面刷新后可以恢复 timeline。
- `go test ./internal/service/agent_v2/runtime ./internal/service/agent_v2/tools ./internal/dao/agent_v2_dao -count=1` 通过。
- `npm run build` 通过。

依赖：Task 2，可与 Task 3 部分并行，但事件和 retry 状态需要对齐。

预计文件：

- `gin_agent_gorm/internal/service/agent_v2/runtime`
- `gin_agent_gorm/internal/service/agent_v2/tools`
- `gin_agent_gorm/internal/dao/agent_v2_dao/tool_dao.go`
- `gin_agent_gorm/internal/dao/agent_v2_dao/ledger_dao.go`
- `frontend/src/views/AgentWorkspaceV2View.vue`
- `frontend/src/types.ts`

### Task 5：升级 Requirement / Prompt Agent 为文本模型驱动并加 schema 校验

说明：当前 Requirement 和 Prompt 主要靠规则和字符串拼接，已经能跑，但不够稳定。下一步应让 Prompt Agent 通过 `TextProvider` 生成结构化输出，并且必须经过 schema 校验和 fallback。

当前状态：已完成第一版（2026-05-27）。`RequirementAgent`/`PromptAgent` 已可通过 `TextProvider` 生成结构化 JSON；provider 输出非法或不可用时会回退规则版本并在 step output 写入 `schema_issues`；`ImageRequirements` 已扩展 `scene/composition/text_policy/layout_hints/target_use`；图像生成前会按工具 capability 限制 prompt 长度、比例和 candidate_count；中文海报默认 `render_text_separately=true`。

要做：

- 扩展 `ImageRequirements`：`scene`、`composition`、`text_policy`、`layout_hints`、`target_use`。
- Prompt Agent 调用 `TextProvider` 生成结构化 JSON。
- 对 Requirement/Prompt 输出做 schema 校验，失败时回退规则型第一版。
- 根据工具 capability 限制 prompt 长度、比例、candidate_count。
- 支持中文海报默认 `render_text_separately=true`，避免图片模型直接生成复杂中文。

验收：

- 文本 provider 可生成结构化 requirement 和 prompt。
- provider 输出不合法时不会导致 run 崩溃，会 fallback 并记录 issue。
- Prompt 中能带入高置信稳定 memory，但不会带入 proposal。
- 单测覆盖正常 JSON、坏 JSON、超长 prompt、中文文字策略。

依赖：Task 4 更佳；最少依赖当前 Tool Registry。

预计文件：

- `gin_agent_gorm/internal/service/agent_v2/domain/types.go`
- `gin_agent_gorm/internal/service/agent_v2/agents/image_generation.go`
- 可新增 `gin_agent_gorm/internal/service/agent_v2/prompt`
- `gin_agent_gorm/internal/service/agent_v2/tools`

### Task 6：实现追问和 Human-in-the-loop 恢复

说明：文档要求需求不足时进入 `waiting_user`。本任务补齐 runtime 暂停、回答接口和前端恢复链路。

当前状态：已完成第一版（2026-05-27）。Requirement Agent 对模糊需求会输出 `need_clarification/questions`；Runtime 在 requirement step 后将 run 标记为 `waiting_user` 并停止后续节点；新增 `POST /api/v2/runs/:id/resume`，用户回答会记录为 `answer_to_questions` 消息、合并进同一份 `RunState.clarifications/user_request`，再把同一个 run 重新投递队列；`/workspace` 已展示追问表单和继续按钮，轮询遇到 `waiting_user` 会暂停。

要做：

- Requirement Agent 判断信息不足时输出 questions。
- Runtime 将 run 标记为 `waiting_user`，不继续执行后续节点。
- 新增回答接口，例如 `POST /api/v2/runs/:id/resume`。
- 用户补充回答后合并到 RunState，worker 从暂停点继续。
- 前端在 `/workspace` 展示追问表单和继续按钮。

验收：

- 需求不足的 run 不会直接生成低质量图片。
- 用户回答后同一个 run 可继续执行，而不是创建无关联新 run。
- step timeline 能显示等待点和恢复点。
- `go test ./internal/service/agent_v2/runtime ./internal/service/agent_v2/app ./internal/controller/agent_v2_ctrl -count=1` 通过。
- `npm run build` 通过。

依赖：Task 3、Task 5。

预计文件：

- `gin_agent_gorm/internal/service/agent_v2/runtime`
- `gin_agent_gorm/internal/service/agent_v2/app`
- `gin_agent_gorm/internal/controller/agent_v2_ctrl`
- `gin_agent_gorm/routers/agent_v2_routes.go`
- `frontend/src/views/AgentWorkspaceV2View.vue`

### Task 7：候选图并行生成、逐张 Review 和 Ranker 精排

说明：artifact group、rank_score、selected_at 已有，但真实 provider 当前不保证 3 张候选，也没有逐张 review 和 ranker。

当前状态：已完成第一版（2026-05-27）。Image Generation Agent 已支持 provider 一次返回多图或返回不足时按剩余候选数继续补调用，Artifact Agent 会为每张候选图写独立 artifact/version；Vision Review 会逐张候选评分并输出 `candidate_reviews`；新增 `ranker_agent`，按 review score、需求匹配、用户历史偏好和失败信号生成 `rank_score` 并由应用层写回 artifact；`/workspace` 候选区按 rank 展示，并标识推荐图和用户选中图。

要做：

- 支持候选图生成策略：一次 provider 多图或多次 provider 调用。
- 每张候选图写独立 artifact/version。
- Vision Review 对每张候选图分别评分。
- 新增 Ranker Agent，综合 review score、需求匹配、用户历史偏好、生成失败信息更新 `rank_score`。
- 前端候选图按 rank 展示，并清晰标识推荐图和用户选中图。

验收：

- 同一 run 可产生 3 张候选图。
- 每张候选图都有版本、quality_scores、rank_score。
- 用户选择第 2 张时后端记录 selected 和 feedback，Memory Proposal 正常生成。
- 前端可对比候选图和评分。

依赖：Task 4、Task 5。

预计文件：

- `gin_agent_gorm/internal/service/agent_v2/workflow/image_generation.go`
- `gin_agent_gorm/internal/service/agent_v2/agents`
- `gin_agent_gorm/internal/service/agent_v2/artifact`
- `gin_agent_gorm/internal/dao/agent_v2_dao/artifact_dao.go`
- `frontend/src/views/AgentWorkspaceV2View.vue`

### Task 8：接入 OCR 和版面质量 Review

说明：真实 Vision Review 已能给总体分，但中文文字、海报排版、可读性和版面问题仍未覆盖。图片 Agent 的质量上限取决于这一层。

当前状态：已完成第一版（2026-05-27）。`GoogleVisionProvider` 复用 Gemini 多模态能力实现 `OCRProvider`；真实 Review Agent 会对每张候选图合并 Vision 与 OCR 结果，输出 `overall_score/requirement_match/composition_score/text_readability/layout_score`，不可读文字会产生 issue 并设置 `should_refine=true`；结果写入 `artifact_versions.quality_scores`，前端 Review/Eval 面板可展示拆分分数和 OCR 文本。真实外部 OCR 目前仍复用 Gemini，多模型/专用 OCR 替换留到后续增强。

要做：

- 实现 `OCRProvider`，可先用 Google Vision/Gemini 多模态能力包装，后续替换云 OCR 或 PaddleOCR。
- Review Agent 同时调用 VisionProvider 和 OCRProvider。
- 质量分拆成 `overall_score`、`requirement_match`、`composition_score`、`text_readability`、`layout_score`。
- 对中文海报、品牌图、含文字图片输出明确 issue。
- 写入 `artifact_versions.quality_scores`。

验收：

- 含文字图片能得到 OCR/text_readability 结果。
- 不可读文字会产生 issue，并将 `should_refine=true`。
- 前端 Review/Eval 面板能展示拆分分数。
- `go test ./internal/service/agent_v2/agents ./internal/service/agent_v2/tools ./internal/service/agent_v2/artifact -count=1` 通过。

依赖：Task 7 可先不完成，但逐张 review 完成后收益最大。

预计文件：

- `gin_agent_gorm/internal/service/agent_v2/tools`
- `gin_agent_gorm/internal/service/agent_v2/agents/vision_review.go`
- `gin_agent_gorm/internal/service/agent_v2/artifact/service.go`
- `frontend/src/views/AgentWorkspaceV2View.vue`

### Task 9：实现 Refiner Agent 和自动二次生成

说明：目前系统“能评估但不会自动改”。下一步应让低分 Review 触发一次受预算控制的改进。

当前状态：已完成第一版（2026-05-28）。新增 `refiner_agent`，在 `ranker_agent` 后根据低分 review/`should_refine` 触发最多一次自动二次生成；新图追加为同一 artifact 的新 version，`operation=refine`，`parent_version_id` 指向原始版本，并保留原始低分版本。`RunBudget.max_auto_refines` 默认 1，预算为 0 时跳过；前端版本列表会显示 refine 版本的 parent 关系。真实外部 refine 仍复用文生图 provider 生成替代版本，后续图像编辑 provider 接入后可切到 edit/refine 专用链路。

要做：

- 新增 `refiner_agent`，根据 review issues 生成改进 prompt 或编辑计划。
- Runtime 支持受控循环：最多自动 refine 1 次，后续靠用户手动触发。
- refine 结果创建新 artifact version 或新 candidate，并设置 `parent_version_id`。
- 低分原图和 refine 结果都保留，不能覆盖。
- 前端展示原始版本和 refine 版本的关系。

验收：

- Review 低于阈值且预算允许时自动生成一次 refine。
- 新版本有 `parent_version_id`，operation 为 `refine` 或 `edit`。
- timeline 能解释为什么 refine。
- 超预算不会继续重试。

依赖：Task 8。

预计文件：

- `gin_agent_gorm/internal/service/agent_v2/agents/refiner_agent.go`
- `gin_agent_gorm/internal/service/agent_v2/workflow`
- `gin_agent_gorm/internal/service/agent_v2/runtime`
- `gin_agent_gorm/internal/service/agent_v2/artifact`
- `frontend/src/views/AgentWorkspaceV2View.vue`

### Task 10：完善 Memory 语义去重、Ranker 和冲突降权

说明：当前 memory proposal 已经能候选、合并、人工确认和自动晋级，但只是同 scope 合并；稳定记忆之间的冲突没有处理。

当前状态：已完成第一版（2026-05-28）。Memory Service 已增加稳定记忆语义去重、PromptContext ranker、同 namespace/scope 冲突降权和 `memory_events=conflict_demoted` 记录；新增 `PATCH /api/v2/memories/:id` 支持编辑、停用和调置信度；前端 Memory 面板已支持 proposal/stable 过滤、编辑和来源展示。语义检索当前先用轻量 token 相似度实现，embedding/vector store 保留为后续增强。

要做：

- 增加 tag 检索和语义检索 adapter，先保留 MySQL 结构，embedding 可延迟接向量库。
- 实现 memory ranker：namespace、confidence、use_count、last_used_at、task_type、feedback 强度共同排序。
- 实现 conflict resolver：同 scope 稳定记忆冲突时降权旧记忆，记录 memory_events。
- 增加 `PATCH /api/v2/memories/:id`，允许用户编辑、停用或调置信度。
- 前端 Memory 面板支持编辑、过滤 proposal/stable、查看来源。

验收：

- 重复表达的偏好不会生成多条稳定记忆。
- 冲突偏好不会同时高置信进入 Prompt。
- 用户能修改或停用记忆。
- PromptContext 只带入经过 ranker 和 conflict resolver 处理后的稳定记忆。

依赖：Task 5。

预计文件：

- `gin_agent_gorm/internal/service/agent_v2/memory`
- `gin_agent_gorm/internal/dao/agent_v2_dao/memory_dao.go`
- `gin_agent_gorm/internal/controller/agent_v2_ctrl`
- `frontend/src/views/AgentWorkspaceV2View.vue`

### Task 11：实现图生图 / 上传图 / 图片编辑版本链

说明：这是 V2 从“文生图工具”升级为“图片 Agent 工作台”的关键功能。必须建立在 artifact version、权限和编辑 parent 链路之上。

当前状态：已完成第一版（2026-05-28）。新增 `POST /api/v2/conversations/:id/artifacts/upload`，上传图片会校验 MIME、大小和像素，按会话归属校验权限后写入 artifact/version，`operation=upload`；新增 `POST /api/v2/artifacts/:id/edit`，基于指定或最新 version 走 `ImageEditProvider` 追加 `operation=edit` 的子版本并记录 `parent_version_id`；前端 `/workspace` 已提供参考图上传、继续编辑 prompt 和版本链查看。当前 `ImageEditProvider` 先复用现有图片 provider 适配，真正原生图生图/局部 mask 编辑和 SegmentationProvider 留给后续 provider 增强。

要做：

- 新增上传接口，校验 MIME、大小、像素和用户权限。
- 上传图进入 artifact/version，operation 为 `upload`。
- 实现 `ImageEditProvider` 和可选 `SegmentationProvider`。
- 新增 Image Edit Agent，支持基于 artifact/version 继续编辑。
- 新增 `POST /api/v2/artifacts/:id/edit`。
- 前端提供继续编辑入口、参考图上传、编辑 prompt 输入。

验收：

- 用户可上传参考图或待编辑图。
- 基于某个 version 编辑时会生成新 version，并记录 parent。
- 旧版本仍可查看和下载。
- 非 owner 不能编辑或下载 artifact。

依赖：Task 9 前后均可；若先做本任务，自动 refine 可复用编辑链路。

预计文件：

- `gin_agent_gorm/internal/service/agent_v2/tools`
- `gin_agent_gorm/internal/service/agent_v2/agents/image_edit_agent.go`
- `gin_agent_gorm/internal/service/agent_v2/artifact`
- `gin_agent_gorm/internal/controller/agent_v2_ctrl`
- `frontend/src/views/AgentWorkspaceV2View.vue`

### Task 12：实现海报 / 品牌图文字分层渲染

说明：文档明确不建议让图片模型直接生成复杂中文。品牌图、海报图应采用“生成底图 + 前端或后端排版文字 + OCR/版面检查”的方式。

当前状态：已完成第一版（2026-05-28）。`Prompt Agent` 已沿用 `text_policy/layout_hints/render_text_separately`，新增 `poster_render_agent` 在生成底图写入 artifact 后自动创建 SVG 文字分层产物，并通过 `parent_artifact_id/parent_version_id/source_refs` 保留底图和排版关系；新增 `POST /api/v2/artifacts/:id/render-text`，前端 `/workspace` 支持用户编辑标题、副标题、品牌文案并生成可预览/下载的 SVG 分层 artifact。当前后端渲染采用 SVG text layer 第一版，浏览器 Canvas 导出和更精细中文排版留到后续 UI 组件化阶段。

要做：

- Prompt Agent 输出 `layout_hints` 和 `text_policy`。
- 新增 HTML/Canvas/SVG Render Agent，用无文字底图生成最终海报。
- Artifact 支持 `html`、`svg`、`image` 多产物关联。
- Review 对最终图做 OCR 和版面检查。
- 前端允许用户编辑标题、副标题、品牌文案和导出图片。

验收：

- 中文标题不由图片模型直接生成，而由渲染层排版。
- 最终 artifact 可预览、下载，并保留底图和排版版本。
- OCR 能检查文字是否存在、是否可读。

依赖：Task 8、Task 11 可选。

预计文件：

- `gin_agent_gorm/internal/service/agent_v2/agents/poster_render_agent.go`
- `gin_agent_gorm/internal/service/agent_v2/workflow`
- `gin_agent_gorm/internal/service/agent_v2/artifact`
- `frontend/src/views/AgentWorkspaceV2View.vue`
- 后续可拆 `frontend/src/components/workspace`

### Task 13：实现 Evolution / Eval / Prompt 版本治理

说明：当前已有 `agent_reflections` 和 `agent_prompt_versions` 表，低分 reflection draft 已有基础，但没有聚合、评测、上线和回滚流程。

要做：

- 新增 `eval_cases`、`eval_runs` model 和 DAO。
- Evolution Agent 定时或手动聚合失败原因 Top 5。
- 从高分/低分样本生成 prompt version draft。
- Prompt 版本状态流转：`draft -> review -> active -> archived`。
- 新增 promote/rollback API。
- 前端或后台页面展示失败聚合和 prompt 版本。

验收：

- 可以查看最近一段时间失败原因 Top 5。
- 可以生成 prompt version draft。
- 只有人工确认或评测通过后才能切 active。
- active prompt 可回滚。

依赖：Task 8、Task 10。

预计文件：

- `gin_agent_gorm/model`
- `gin_agent_gorm/internal/service/agent_v2/eval`
- `gin_agent_gorm/internal/dao/agent_v2_dao/eval_dao.go`
- `gin_agent_gorm/internal/controller/agent_v2_ctrl`
- `frontend/src/views/AgentWorkspaceV2View.vue` 或新增后台视图。

### Task 14：补齐安全、存储和合规边界

说明：鉴权预览已经完成第一版，但上传、分享、object key、内容安全和日志边界还需要系统化。

要做：

- 上传限制：文件大小、MIME、像素、扩展名、扫描失败策略。
- object key 随机化，避免可预测路径长期暴露。
- 若需要外链分享，实现短期签名 URL；如果不需要分享，继续保持鉴权 API 为唯一入口。
- Provider API key、token、prompt 敏感信息继续日志脱敏。
- 接入 SafetyProvider：生成前文本安全审查、生成后图片安全审查。
- 增加 artifact edit/download/preview 权限集成测试。

验收：

- 非 owner 不能预览、下载、编辑、反馈。
- 上传非法文件会被拒绝。
- 日志不包含 token、API key、二进制图片 body。
- 静态 `/artifacts` 默认关闭仍有效。

依赖：Task 11 前必须完成上传限制；签名 URL 可按外链需求延后。

预计文件：

- `gin_agent_gorm/internal/service/agent_v2/security`
- `gin_agent_gorm/config/ai_agent.go`
- `gin_agent_gorm/internal/middleware/access_log_middleware.go`
- `gin_agent_gorm/internal/service/agent_svc/storage.go`
- `gin_agent_gorm/internal/controller/agent_v2_ctrl`

### Task 15：前端工作台组件化和体验补齐

说明：`AgentWorkspaceV2View.vue` 已经承载很多职责，后续继续加编辑、候选精排、追问、演进面板会变得难维护。应在主链路稳定后拆组件。

要做：

- 拆出 API client：`agentV2.ts`、`artifacts.ts`、`memories.ts`。
- 拆出 composables：`useAgentRun`、`useRunEvents`、`useArtifacts`、`useMemories`。
- 拆出组件：Composer、Timeline、ArtifactBoard、VersionStrip、ReviewPanel、MemoryPanel、EditPanel。
- 补齐 UI 状态：empty、queued、running、waiting_user、failed、cancelled、completed。
- 增加编辑/重生成入口、候选对比、错误重试入口。

验收：

- `npm run build` 通过。
- 长 prompt、长错误、多个候选图、无 artifact、失败 run 都不撑破页面。
- 页面刷新后能从 run id 恢复进度和产物。

依赖：Task 6、Task 7、Task 11 的前端能力稳定后做最划算。

预计文件：

- `frontend/src/api.ts`
- `frontend/src/types.ts`
- `frontend/src/views/AgentWorkspaceV2View.vue`
- 新增 `frontend/src/api`
- 新增 `frontend/src/components/workspace`
- 新增 `frontend/src/composables`

### Task 16：默认入口迁移、文档同步和旧代码收敛

说明：旧代码应最后清理。只有 V2 工作台的生成、编辑、反馈、记忆、评审、恢复都稳定后，才迁移主入口。

要做：

- 前端默认入口从旧 Chat 流程切到 V2 Workspace。
- 旧 `agent_svc` 标记 deprecated，不再新增能力。
- 迁移旧 artifact 到新 version 结构，或提供兼容读取。
- 删除不可达 mock 流程和过期文档段落。
- 更新 README、部署文档、Google 配置文档和 V2 进度文档。

验收：

- 新用户默认进入 V2 工作台。
- 旧会话和旧 artifact 不丢失。
- `go test ./...`、`npm run build`、`git diff --check` 通过。
- 文档中“已完成 / 未完成”与实际代码一致。

依赖：Task 1-15。

预计文件：

- `frontend/src/router/index.ts`
- `frontend/src/views/ChatView.vue`
- `gin_agent_gorm/internal/service/agent_svc`
- `gin_agent_gorm/docs`
- `gin_agent_gorm/README.md`

## 5. 推荐里程碑

### Milestone A：生产可运行主链路

范围：Task 1-4。

目标：真实生成链路可通过前端验收，并且异步执行、恢复、重试、事件和工具调用追踪具备生产基础。

完成后再继续做 Prompt、候选、OCR、Refine。

### Milestone B：质量闭环

范围：Task 5-10。

目标：Prompt 更可靠，候选图可排序，Review 能识别文字和版面问题，低分结果能自动 refine，Memory 不污染 prompt。

完成后系统才从“能生成图”进入“能持续改进图”。

### Milestone C：编辑和海报能力

范围：Task 11-12。

目标：支持上传图、图生图、图片编辑、版本链和中文海报文字分层。

完成后 V2 才能覆盖图片工作台的核心编辑场景。

### Milestone D：演进、安全和收敛

范围：Task 13-16。

目标：补齐 Evolution、Eval、Prompt 版本治理、安全审查、前端组件化和旧代码收敛。

完成后再考虑新增更多模型、复杂工作流或第三方视觉工具。

## 6. 暂缓事项

这些能力可以先不做，避免抢占主链路稳定性：

- ComfyUI 节点编排。
- GroundingDINO/SAM 深度局部编辑。
- 完整向量数据库和复杂 RAG。
- 多 Agent 辩论式生成。
- 自动上线 Prompt 版本。
- 多租户计费。
- 外链分享签名 URL，除非产品明确需要分享链接。

## 7. 每轮开发后的固定动作

每完成一个 Task，必须同步做：

- 更新 `docs/IMAGE_AGENT_V2_PROGRESS.md` 的状态、验收命令和未完成项。
- 补或更新对应单元测试 / 集成测试。
- 后端改动至少跑相关 `go test`；跨模块改动跑 `go test ./...`。
- 前端改动跑 `npm run build`。
- 涉及文档或空白敏感改动时跑 `git diff --check`。
- 记录是否有真实外部模型验收；没有真实凭据时明确写“仅 mock / 编译通过”。
