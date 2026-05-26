# 图片 AI Agent V2 开发进度记录

生成日期：2026-05-25  
维护规则：每次完成开发任务后都必须更新本文档，明确记录“做了哪一步、做了哪些、哪些没做、验收是否通过”。

参考文档：
- [IMAGE_AGENT_CURRENT_TO_V2_DEVELOPMENT_PLAN.md](./IMAGE_AGENT_CURRENT_TO_V2_DEVELOPMENT_PLAN.md)
- [IMAGE_AGENT_REWRITE_DEVELOPMENT_PLAN.md](./IMAGE_AGENT_REWRITE_DEVELOPMENT_PLAN.md)
- [IMAGE_AGENT_DEVELOPMENT_GUIDE.md](./IMAGE_AGENT_DEVELOPMENT_GUIDE.md)

## 1. 当前总状态

当前方向没有偏离重写计划：新能力继续放在 `internal/service/agent_v2`、`internal/dao/agent_v2_dao`、`internal/controller/agent_v2_ctrl` 和 `/api/v2` 下，旧 `agent_svc` 未继续扩展为 V2 能力。

当前系统状态：

| 范围 | 状态 | 说明 |
| --- | --- | --- |
| V2 后端骨架 | 已完成 | Run、Step、Workflow、Runtime、DAO、基础 Service 已具备 |
| 第15节第一轮验收 | 代码链路已闭环 | `/api/v2/conversations/:id/runs` 已接真实 provider adapter、真实 Requirement/Prompt/Image/Artifact Agent、artifact version 写库、V2 前端入口；真实外部模型端到端仍依赖用户配置可用图片模型 |
| 第16节第二轮后端能力 | 已完成后端切片 | Memory 查询/删除、candidate group、selected artifact、vision review mock、reflection draft、basic budget、idempotency key 已实现 |
| 前端 V2 Workspace | 已完成第一批 | 新增 `/workspace`，支持输入、模型选择、运行、timeline、artifact board、版本、下载、反馈 |
| 真实图片生成链路 | 已完成代码接入 | V2 Image Agent 通过 Tool Registry 调用旧图片 provider adapter，并由 Artifact Agent 写入 artifact 与 artifact_version；未在本机用真实三方模型凭据做外部端到端验收 |

## 2. `IMAGE_AGENT_DEVELOPMENT_GUIDE.md` 全量对齐清单

本项目进度不只对齐 V2 文档末尾任务，也必须对齐 [IMAGE_AGENT_DEVELOPMENT_GUIDE.md](./IMAGE_AGENT_DEVELOPMENT_GUIDE.md) 中全部开发要求。下面是当前全量对齐状态，后续每次任务完成后都要同步更新。

### 2.1 总体架构和边界

| 指南章节 | 要求 | 当前状态 | 已完成 | 未完成 / 风险 |
| --- | --- | --- | --- | --- |
| 4 推荐目标架构 | Runtime、Workflow、Memory、Tool、Artifact、Evolution 分层 | 部分完成 | 已新增 `agent_v2/domain/runtime/workflow/memory/tools/artifact/eval` 基础包 | Event、Security、Prompt 子包还未完整落地 |
| 14 后端代码组织建议 | 后端按 runtime、workflow、agents、memory、tools、artifacts、eval 拆分 | 部分完成 | V2 已按新结构拆包；新增真实 Requirement/Prompt/Image/Artifact Agent；未继续扩展旧 `agent_svc` | `prompt/security/event` 子包未建；Refiner/Evolution Agent 未完成 |
| 21.12 重写目录结构 | 新能力进入 `agent_v2` | 部分完成 | 当前新增后端代码进入 `agent_v2`、`agent_v2_dao`、`agent_v2_ctrl`；前端新增独立 V2 工作台 | 旧前端和旧接口仍保留，后续需逐步迁移主入口和历史能力 |

### 2.2 多 Agent 协同

| 指南章节 | 要求 | 当前状态 | 已完成 | 未完成 / 风险 |
| --- | --- | --- | --- | --- |
| 5.1 共享任务状态 | 所有 Agent 读写同一份 `RunState` | 部分完成 | 已定义 `domain.RunState`，Runtime 会保存 `state_json`；新增 `GeneratedImages` 作为 Image Agent 到 Artifact Agent 的结构化交接 | `constraints`、完整 tool calls、完整 review/eval 结构仍不完整 |
| 5.2 Agent 输入输出契约 | Agent 输出必须结构化，不只返回自然语言 | 部分完成 | `domain.StepResult` 已承载真实 Requirement/Prompt/Image/Artifact Agent 输出；Runtime 合并结构化需求、prompt、图片、artifact 引用 | schema 校验未做；`questions/plan/tool_calls/eval_scores` 未完整建模 |
| 5.3 Task Ledger | 每个 run 维护任务账本 | 部分完成 | 已新增 `task_ledger_items` model 和 DAO | Runtime 尚未写 ledger；前端不展示 ledger |
| 5.4 协同模式 | 支持顺序、Planner+Tools、Review、DAG、人工介入 | 部分完成 | 顺序 DAG 已支持；mock review 已支持 | 并行 DAG、Planner 动态工具、Human-in-the-loop、Refiner 未完成 |
| 5.5 生产级协作约束 | 幂等、资源锁、预算、失败降级、可观测、安全边界 | 部分完成 | step hash、duration、basic max_steps budget、run idempotency key、基础权限查询已实现 | Redis lock、retry、费用预算、图片次数预算、失败降级、完整权限代理未完成 |

### 2.3 Agent 分工和工作流

| 指南章节 | 要求 | 当前状态 | 已完成 | 未完成 / 风险 |
| --- | --- | --- | --- | --- |
| 6 Agent 分工 | Intent、Requirement、Memory、Prompt、Image、Vision、Review、Refiner、Artifact、Evolution | 部分完成 | 已实现真实 Intent/Requirement/Memory/Prompt/Image/Artifact Agent 第一版；mock vision review；artifact/eval service 基础 | Requirement/Prompt 仍是规则型第一版；Refiner/Evolution Agent 未完成；真实 Vision/OCR 未接 |
| 8.1 文生图工作流 | 文生图完整链路含追问、记忆、prompt、图片生成、artifact、review、refine | 部分完成 | V2 workflow `0.2.0` 已从 mock 切到真实 provider adapter + artifact version 写库；前端可展示 timeline 和产物 | 追问、人设记忆带入、自动 review/refine 尚未闭环；真实外部模型端到端依赖有效配置 |
| 8.2 图生图 / 图片编辑 | 上传、视觉分析、mask、编辑模型、版本链 | 未完成 | Tool 接口预留 ImageEdit/Segmentation | 上传图 V2 流程、Segmentation、Image Edit Agent 未完成 |
| 8.3 品牌图 / 海报图 | 文字分层处理、HTML/Canvas 排版、Review | 未完成 | Prompt 结构里有 `render_text_separately` 字段 | HTML/Canvas Agent、中文文字 OCR 检查、可控排版未完成 |
| 8.4 候选图并行 | 3 个候选 prompt/图、逐张 review、排序、最佳图 | 部分完成 | artifact 支持 candidate group、rank_score、selected_at；selected feedback 已实现 | 并行生成、Ranker Agent、逐张 review、前端候选对比未完成 |

### 2.4 记忆系统

| 指南章节 | 要求 | 当前状态 | 已完成 | 未完成 / 风险 |
| --- | --- | --- | --- | --- |
| 7.1 记忆分层 | 短期、会话摘要、用户偏好、视觉风格、产物、工具经验、失败反思、评测记忆 | 部分完成 | 已定义 namespace：conversation、user_profile、visual_style、artifact_lineage、tool_experience、reflection | 分层写入策略还未接真实 Agent；评测记忆未完整实现 |
| 7.2 扩展 `context_memories` | namespace/source/artifact/tags/confidence/embedding/expires/use_count | 已完成基础字段 | model 和 AutoMigrate 已补字段；Memory Service 可查询、写入、删除 | `tags` 当前为 `tags_json`；未接向量库 |
| 7.3 写入策略 | 只写稳定偏好、用户选择、高分、失败模式 | 部分完成 | Memory Service 可写入；selected artifact 会写 feedback | 未从 feedback/review 自动生成 memory proposal |
| 7.4 检索策略 | 会话、用户偏好、视觉风格、失败经验组合检索 | 部分完成 | 支持 user/conversation/namespace/scope/limit 查询 | 语义检索、tag 检索、模型经验检索未完成 |
| 7.5 冲突处理 | 同 scope 冲突降权旧记忆 | 未完成 | 暂无 | 需实现 ranker/conflict resolver |

### 2.5 产物、版本和反馈

| 指南章节 | 要求 | 当前状态 | 已完成 | 未完成 / 风险 |
| --- | --- | --- | --- | --- |
| 9 产物与版本管理 | artifact version 记录 prompt、模型、参数、source、quality、feedback | 部分完成 | `artifact_versions`、`artifact_feedback` model 已有；Artifact Agent 已把 prompt、negative prompt、模型、参数、object key 写入 version；V2 feedback API 已接入 | 质量分未接 review；edit/version parent 未跑通 |
| 13 Phase 1 图片产物血缘 | artifact_versions、artifact_group_id、rank_score、selected_at、step observability | 部分完成 | 字段、基础 service、真实链路写入、前端 artifact board/versions/feedback 第一批已完成 | 图片 provider 当前不保证真实生成 3 张候选；逐张 review/rank 未完成 |
| 11.1/11.3 进化数据来源和新增表 | feedback、prompt versions、reflections | 部分完成 | `agent_prompt_versions`、`agent_reflections`、`artifact_feedback` 已有；低分 reflection draft 已实现 | eval_cases/eval_runs 未实现；prompt promote/rollback 未实现 |

### 2.6 Tool / Provider 抽象

| 指南章节 | 要求 | 当前状态 | 已完成 | 未完成 / 风险 |
| --- | --- | --- | --- | --- |
| 10 Provider 抽象升级 | Text/ImageGen/ImageEdit/Vision/Segmentation 分接口 | 部分完成 | Tool Registry 已定义 Text、ImageGeneration、ImageEdit、Vision、OCR、Segmentation、Safety 接口；新增旧 `HTTPProvider.Chat/Generate` 到 V2 `TextProvider`/`ImageGenerationProvider` 的 adapter | ImageEdit/Vision/OCR/Segmentation 仍未接真实 provider |
| 10.1 图片工具链 | 接图片生成、VLM、OCR，后续 GroundingDINO/SAM | 未完成 | 仅接口预留和 mock vision review | 真实 VLM/OCR/GroundingDINO/SAM 均未接入 |
| 21.5 Provider / Tool | Agent 只依赖能力，不依赖具体模型 | 部分完成 | Registry 支持 `FindTool(kind, user_id, model_config_id)`；Image Agent 已真实通过 Registry 查找图片生成工具 | Prompt Agent 第一版仍为规则型，未调用文本 provider；后续可切换为 provider 驱动的结构化 prompt |

### 2.7 Prompt 策略和图片文字处理

| 指南章节 | 要求 | 当前状态 | 已完成 | 未完成 / 风险 |
| --- | --- | --- | --- | --- |
| 15.1 结构化需求 | Prompt Agent 先生成结构化需求 | 部分完成 | Requirement Agent 第一版已从用户输入抽取 subject/style/aspect_ratio/must_include/must_avoid/questions | scene/composition/text_policy/layout_hints 仍需更细建模和 schema 校验 |
| 15.2 Prompt 输出 | positive/negative/layout_hints/render_text_separately | 部分完成 | Prompt Agent 第一版已输出 positive/negative/render_text_separately/params | layout_hints 未建模；尚未接文本模型生成更强 prompt |
| 15.3 图片文字处理 | 中文海报底图和文字排版分离，OCR 检查 | 未完成 | 仅有策略字段预留 | HTML/Canvas/SVG 排版和 OCR 校验未完成 |

### 2.8 API 和前端体验

| 指南章节 | 要求 | 当前状态 | 已完成 | 未完成 / 风险 |
| --- | --- | --- | --- | --- |
| 12 前后端 API 建议 | run events、artifact 操作、memory 操作 | 部分完成 | `/api/v2/conversations/:id/runs` 已执行真实 workflow；已补 artifacts list、versions、download、feedback、select；memory search/delete 已有 | edit、memory patch、run 异步事件流增强未完成 |
| 16 前端体验建议 | 对话 + 产物 + 过程 + 版本 + 反馈 | 部分完成 | 新增 `/workspace` V2 Workspace 第一批：输入、模型选择、运行、timeline、artifact board、版本、下载、反馈 | Memory 入口、Review/Eval 展示、候选对比精排未完成 |
| 21.10 前端工作台 | 记忆查看、候选图、版本、选择、下载、Review/Eval | 部分完成 | V2 工作台已支持候选图列表、版本、下载、反馈 | 记忆查看、选择按钮、Review/Eval、编辑/重生成入口未完成 |

### 2.9 安全、权限和合规

| 指南章节 | 要求 | 当前状态 | 已完成 | 未完成 / 风险 |
| --- | --- | --- | --- | --- |
| 17 安全权限合规 | artifact 预览/下载/编辑校验 user/conversation，上传限制，object key 不可预测，日志脱敏 | 部分完成 | DAO/service 查询按 user_id；memory delete 按 user_id；V2 download/feedback/list/version 均通过 user 范围校验；旧 access log 避免 artifacts 响应体 | V2 preview 仍走静态 `/artifacts`；签名 URL、上传限制、安全审查、object key 随机策略未系统化 |
| 21.9 安全与合规 | 不暴露静态路径作为权限边界 | 部分完成 | V2 download 已走鉴权 API | 当前仍有静态 `/artifacts` 预览路径，后续需改鉴权代理或签名 URL |

### 2.10 实施计划和 MVP

| 指南章节 | 要求 | 当前状态 | 已完成 | 未完成 / 风险 |
| --- | --- | --- | --- | --- |
| 18 第0周整理基线 | 梳理当前表使用，step timeline，artifact 可追溯 | 部分完成 | step observability 字段、真实 timeline、真实图片 artifact/version 追溯第一批已完成 | 仍需补真实 DB 集成记录和长期可用性记录 |
| 18 第1周产物版本化 | 3 候选图、选择第2张、feedback/selected | 部分完成 | candidate group、selected feedback 后端能力、V2 feedback API、前端版本/下载/反馈已完成 | 图片 provider 当前不保证 3 张候选；前端选择按钮和候选精排未完成 |
| 18 第2周记忆 MVP | 用户偏好自动带入、可删除覆盖 | 部分完成 | namespace 查询/删除已完成 | 偏好自动带入 Prompt Agent 未完成 |
| 18 第3周 Vision Review | VLM/OCR 质量检查，低分问题写入 version | 部分完成 | mock review 和 low-score reflection draft 已完成 | 真实 VLM/OCR 与 artifact version quality_scores 未完成 |
| 18 第4周自动 Refine | Review 低分自动二次 prompt，最多重试 | 未完成 | 暂无 | Refiner Agent 和 retry budget 未实现 |
| 18 第5周进化闭环 | Top 5 失败原因，prompt version 回滚 | 部分完成 | reflection/prompt version 表和低分 draft 基础已完成 | Top 5 聚合、prompt promote/rollback 未完成 |
| 19 MVP 技术决策 | 固定 DAG、Redis/Asynq 长任务、MySQL 记忆、复用 provider、先反馈反思 | 部分完成 | 固定 DAG、MySQL 记忆、旧 provider 复用、反馈/反思基础已完成 | Redis/Asynq V2 长任务、反馈进化未闭环、真实外部模型运行稳定性未验收 |

## 3. 第14节第一轮开发顺序进度

| 序号 | 任务 | 状态 | 已完成 | 未完成 / 备注 |
| --- | --- | --- | --- | --- |
| 1 | 补齐 model 和 AutoMigrate | 已完成 | 扩展 `agent_runs`、`agent_steps`、`context_memories`、`artifacts`；新增 `task_ledger_items`、`tool_invocations`、`memory_events`；更新 AutoMigrate 清单 | 未做独立 SQL migration，当前沿用项目 AutoMigrate |
| 2 | 补齐 v2 DAO | 已完成 | 拆分 `run/step/artifact/memory/tool/eval/ledger` DAO；补权限范围查询方法 | 还未做真实 DB 集成测试 |
| 3 | Runtime 支持顺序 DAG | 已完成 | `workflow.DAG`、依赖排序、循环/缺失依赖检测；Executor 使用 `OrderedNodes()` | 暂不做并行节点、恢复、重试 |
| 4 | Artifact Service MVP | 已完成第一批 | 支持 artifact + version 创建、candidate group、选择产物、下载鉴权入口、feedback 写入入口；V2 list/version/download/feedback API 已接上 | edit/version parent、质量分写入未完成 |
| 5 | Tool Registry MVP | 已完成 | 支持按 kind/model_config_id 注册和查找工具；定义 Text/Image/Vision/OCR/Segmentation/Safety provider 接口；旧文本/图片 provider 已包装进 V2 tools | ImageEdit/Vision/OCR/Segmentation 真实 provider 未接入 |
| 6 | Memory Service MVP | 已完成 | 支持 namespace 查询、MarkUsed、写入、软删除、memory event | 尚未接向量检索和冲突降权 |
| 7 | 文生图真实链路 | 已完成第一批 | 实现真实 Requirement、Prompt、Image Generation、Artifact Agent；`/api/v2/conversations/:id/runs` 已执行 workflow `0.2.0`，调用真实 provider adapter 并写 artifact/version | Requirement/Prompt 仍是规则型；真实外部模型凭据端到端未验收；review/refine 未闭环 |
| 8 | v2 API 第一批 | 已完成第一批 | 已有 run 创建/查询/events；新增 memories 查询/删除、artifact select；补齐 artifacts list、versions、download、feedback；run 创建已从 mock workflow 切到真实 workflow | edit、memory patch、异步长任务 API 未完成 |
| 9 | 前端 V2 Workspace 第一批 | 已完成 | 新建 `/workspace` 页面和 API client，支持输入、模型选择、运行、timeline、artifact board、versions、download、feedback | Memory 入口、Review/Eval、选择按钮、编辑/重生成未完成 |
| 10 | 权限校验 | 部分完成 | run 按 user 校验；artifact service/DAO 按 user 校验；memory 删除按 user 校验；V2 download/feedback/list/version 按 user 校验 | V2 preview 仍是静态 URL；签名 URL/代理预览未完成 |
| 11 | 测试和文档同步 | 进行中 | 已补 model、AutoMigrate、DAO、DAG、Artifact、Memory、Tool、Budget、Idempotency、Review、Reflection、Provider Adapter、Image/Artifact Agent、RunState 合并测试；新增本文档；新增全量指南对齐清单 | 后续每次开发后继续更新本文档 |

## 4. 第15节第一轮验收状态

第一轮验收要求是主链路闭环：

```text
用户登录
  -> 创建会话
  -> 调用 /api/v2/conversations/:id/runs
  -> Runtime 执行 DAG
  -> Prompt Agent 生成 prompt
  -> Image Agent 调用图片 provider
  -> Artifact Service 写入 artifact_version
  -> 前端展示 timeline 和图片
  -> 用户下载或反馈
```

当前验收结论：代码链路第一批已通过；真实外部模型调用仍需要在有可用图片模型配置和网络环境下做人工端到端验收。

| 验收项 | 状态 | 说明 |
| --- | --- | --- |
| `go test ./...` 通过 | 已通过 | 后端全量测试通过 |
| `npm run build` 通过 | 已通过 | 前端构建通过 |
| 每个 run 有 step timeline | 已通过 | 真实 workflow `0.2.0` 会写 intent、requirement、memory、prompt、image_generation、artifact step |
| 每个 step 有耗时和 input/output hash | 已具备基础能力 | Executor 已写 `duration_ms`、`input_hash`、`output_hash` |
| 每个 artifact 至少一个 version | 已通过代码验收 | Artifact Agent 会创建 artifact 和 artifact_version，写入 prompt、negative prompt、模型、参数和 object key |
| artifact 下载校验 user_id | 已通过代码验收 | 新增 `GET /api/v2/artifacts/:id/download`，通过 Artifact Service owner check 后返回文件 |
| feedback 可写入 | 已通过代码验收 | 新增 `POST /api/v2/artifacts/:id/feedback`，写入前校验 artifact owner |
| Prompt Agent 生成 prompt | 已完成第一版 | Prompt Agent 从结构化需求生成 positive/negative/render_text_separately/params；后续接文本模型增强 |
| Image Agent 调用图片 provider | 已完成第一版 | 旧 `HTTPProvider.Generate` 已包装为 V2 `ImageGenerationProvider`，Image Agent 通过 Tool Registry 调用 |
| 前端展示 timeline 和图片 | 已完成第一版 | 新增 `/workspace`，展示 timeline、artifact board、version、download、feedback |

## 5. 第16节第二轮开发和验收状态

当前验收结论：后端能力切片已完成并通过测试；本轮已把其中 artifact API 与前端第一批体验接入 V2 工作台。

| 能力 | 状态 | 已完成 | 未完成 / 备注 |
| --- | --- | --- | --- |
| Memory namespace 查询和删除 | 已完成 | `Memory Service` 支持 namespace/scope/conversation 查询、MarkUsed、软删除；新增 `/api/v2/memories`、`/api/v2/memories/search`、`DELETE /api/v2/memories/:id` | 前端入口未做 |
| candidate group | 已完成 | `Artifact Service` 支持 `CreateCandidateGroup`，同一轮候选图共享 `artifact_group_id`；真实 Artifact Agent 已调用 | provider 当前不保证一次真实返回 3 张候选 |
| selected artifact | 已完成 | `Artifact Service.SelectArtifact` 更新 `selected_at` 并写入 `artifact_feedback(selected)`；新增 `/api/v2/artifacts/:id/select` | 前端选择按钮未做，当前先支持通用 feedback |
| vision review mock | 已完成 | 新增 `MockVisionReviewAgent`，能对无 artifact 给低分并返回 `should_refine` | 尚未接真实 VLM/OCR |
| low score reflection draft | 已完成 | 新增 `eval.ReflectionService`，低分 Review 生成 draft `agent_reflections`，不自动提升为 memory | 尚未接定时 Evolution Agent |
| basic budget | 已完成 | Runtime 在执行前检查 `RunBudget.MaxSteps`，超过预算时 run 失败 | 还未做费用预算、图片次数预算、Redis lock |
| idempotency key | 已完成 | `CreateRunRequest` 支持 `idempotency_key`；重复 key 返回已有 run | 还未加数据库唯一约束 |

第16节验证命令：

```bash
cd gin_agent_gorm
go test ./...

cd ../frontend
npm run build
```

最近一次验证结果：
- `go test ./...`：通过
- `npm run build`：通过
- `git diff --check`：通过

## 6. 重写计划对齐情况

已确认 `IMAGE_AGENT_REWRITE_DEVELOPMENT_PLAN.md` 的方向：

| 计划要求 | 当前状态 |
| --- | --- |
| 保留外围能力，重写核心 Agent | 对齐 |
| 新能力走 `agent_v2`，不继续堆旧 `agent_svc` | 对齐 |
| Runtime 不写具体 prompt/图片业务 | 对齐 |
| Agent 不直接写 DB | 当前新增能力对齐 |
| Artifact / Memory / Tool / Eval 独立 service | 对齐 |
| 每阶段可运行、可验收 | 部分对齐，后端测试和前端构建已跑；第15主链路代码闭环已完成，但外部真实模型端到端需人工配置验收 |

当前主要偏差风险：
- 真实图片 provider 已接入代码链路，但未在当前环境使用真实凭据跑一次外部端到端。
- V2 预览仍依赖静态 `/artifacts`，安全边界还需要鉴权代理或签名 URL。
- Review/Refine/Evolution 仍是后续闭环，不能长期停在“能生成但不评价/不迭代”的状态。

## 7. 下一步建议

本轮已完成上一版“下一步建议”中的 1-6：

| 序号 | 建议 | 状态 | 本轮结果 | 未完成 / 后续 |
| --- | --- | --- | --- | --- |
| 1 | 包装旧文本 provider 为 `tools.TextProvider` | 已完成 | 新增 `LegacyProviderAdapter.GenerateText` | Prompt Agent 尚未默认调用文本 provider |
| 2 | 包装旧图片 provider 为 `tools.ImageGenerationProvider` | 已完成 | 新增 `LegacyProviderAdapter.GenerateImage`，保存对象并返回 V2 图片元数据 | 真实外部模型端到端需凭据验收 |
| 3 | 实现真实 Requirement/Prompt/Image/Artifact Agent | 已完成第一版 | 新增规则型 Requirement/Prompt、Registry 驱动 Image、Artifact version 写库 Agent | Refiner/Evolution/Vision/OCR 未完成 |
| 4 | `/api/v2/conversations/:id/runs` 生成真实 artifact 和 artifact_version | 已完成第一版 | run 创建已执行 workflow `0.2.0` 并返回 artifacts | 长任务异步化未完成 |
| 5 | 补齐 V2 artifact API：list、versions、download、feedback | 已完成 | 新增 list、versions、download、feedback 路由和 controller/service 方法 | edit、preview 鉴权代理未完成 |
| 6 | 新建前端 V2 Workspace 第一批 | 已完成 | 新增 `/workspace`，支持输入、模型选择、运行、timeline、artifact board、versions、download、feedback | Memory、Review/Eval、选择、编辑/重生成未完成 |

新的下一步建议：

1. 用真实图片模型配置跑一次 `/workspace` 端到端，记录 provider、artifact/version、下载、feedback 的真实验收结果。
2. 给 V2 preview 增加鉴权代理或签名 URL，降低静态 `/artifacts` 直接暴露风险。
3. 接入真实 Vision/OCR Review，把质量分写入 `artifact_versions.quality_scores`。
4. 补前端 artifact 选择按钮、Memory 入口、Review/Eval 面板。
5. 设计 V2 长任务异步化（Redis/Asynq 或现有任务队列），避免真实图片模型阻塞 HTTP 请求。

## 8. 更新日志

| 日期 | 任务 | 更新内容 | 验收 |
| --- | --- | --- | --- |
| 2026-05-25 | 第14节第一轮基础能力 | 补齐 V2 model/AutoMigrate、DAO、顺序 DAG、Artifact/Tool/Memory MVP service | `go test ./...` 通过 |
| 2026-05-25 | 第15节验收 | 明确第15节未完全通过，真实图片链路和 V2 前端仍缺失 | `go test ./...`、`npm run build` 通过 |
| 2026-05-25 | 第16节后端能力 | 完成 Memory 查询/删除、candidate group、selected artifact、vision review mock、reflection draft、basic budget、idempotency key，并接入部分 V2 API | `go test ./...`、`npm run build`、`git diff --check` 通过 |
| 2026-05-25 | 进度文档 | 新增本文档，后续每次任务完成后必须更新 | 文档已创建 |
| 2026-05-25 | 全量指南对齐 | 将进度记录范围扩展到 `IMAGE_AGENT_DEVELOPMENT_GUIDE.md` 全部开发要求，不再只对齐 V2 末尾任务 | 文档已更新 |
| 2026-05-25 | 下一步建议 1-6 | 完成旧文本/图片 provider adapter、真实 Requirement/Prompt/Image/Artifact Agent、V2 run 真实 artifact/version 写库、artifact list/version/download/feedback API、前端 `/workspace` 第一批 | `go test ./...`、`npm run build`、`git diff --check` 通过；真实外部图片模型端到端待凭据验收 |
