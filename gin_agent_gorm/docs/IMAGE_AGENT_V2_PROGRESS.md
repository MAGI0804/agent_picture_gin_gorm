# 图片 AI Agent V2 开发进度记录

生成日期：2026-05-25  
最近更新：2026-05-26
维护规则：每次完成开发任务后都必须更新本文档，明确记录“做了哪一步、做了哪些、哪些没做、验收是否通过”。

参考文档：
- [IMAGE_AGENT_CURRENT_TO_V2_DEVELOPMENT_PLAN.md](./IMAGE_AGENT_CURRENT_TO_V2_DEVELOPMENT_PLAN.md)
- [IMAGE_AGENT_REWRITE_DEVELOPMENT_PLAN.md](./IMAGE_AGENT_REWRITE_DEVELOPMENT_PLAN.md)
- [IMAGE_AGENT_DEVELOPMENT_GUIDE.md](./IMAGE_AGENT_DEVELOPMENT_GUIDE.md)
- [IMAGE_AGENT_V2_ASYNC_RUN_DESIGN.md](./IMAGE_AGENT_V2_ASYNC_RUN_DESIGN.md)

## 1. 当前总状态

当前方向没有偏离重写计划：新能力继续放在 `internal/service/agent_v2`、`internal/dao/agent_v2_dao`、`internal/controller/agent_v2_ctrl` 和 `/api/v2` 下，旧 `agent_svc` 未继续扩展为 V2 能力。

当前系统状态：

| 范围 | 状态 | 说明 |
| --- | --- | --- |
| V2 后端骨架 | 已完成 | Run、Step、Workflow、Runtime、DAO、基础 Service 已具备 |
| 第15节第一轮验收 | 代码链路已闭环 | `/api/v2/conversations/:id/runs` 已接真实 provider adapter、真实 Requirement/Prompt/Image/Artifact/Review Agent 链路、artifact version 写库、V2 前端入口；真实外部模型端到端仍依赖用户配置可用图片模型 |
| 第16节第二轮后端能力 | 已完成后端切片 | Memory 查询/删除、candidate group、selected artifact、vision review mock、reflection draft、basic budget、idempotency key、V2 鉴权预览代理、review quality_scores 写入、feedback/review memory proposal、proposal 去重/晋级、Prompt 高置信记忆带入、异步 Run 后端第一版已实现 |
| 前端 V2 Workspace | 已完成第二批 | 新增 `/workspace`，支持输入、模型选择、运行、timeline、artifact board、版本、下载、反馈、选择按钮、Memory 入口、Review/Eval 面板、鉴权预览 blob |
| 真实图片生成链路 | 已完成后端 E2E | V2 Image Agent 通过 Tool Registry 调用 Google Imagen 真实 provider，并由 Artifact Agent 写入 artifact 与 artifact_version；workflow `0.3.0` 已可接真实 Google Vision Review 或回退 mock；Google Imagen 后端 E2E 已通过，前端 `/workspace` 待本机服务和代理在线后冒烟 |

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
| 5.2 Agent 输入输出契约 | Agent 输出必须结构化，不只返回自然语言 | 部分完成 | `domain.StepResult` 已承载真实 Requirement/Prompt/Image/Artifact/Review Agent 输出；Runtime 合并结构化需求、prompt、图片、artifact 引用和 review 结果 | schema 校验未做；`questions/plan/tool_calls/eval_scores` 未完整建模 |
| 5.3 Task Ledger | 每个 run 维护任务账本 | 部分完成 | 已新增 `task_ledger_items` model 和 DAO | Runtime 尚未写 ledger；前端不展示 ledger |
| 5.4 协同模式 | 支持顺序、Planner+Tools、Review、DAG、人工介入 | 部分完成 | 顺序 DAG 已支持；mock review 已接入主 workflow | 并行 DAG、Planner 动态工具、Human-in-the-loop、Refiner 未完成 |
| 5.5 生产级协作约束 | 幂等、资源锁、预算、失败降级、可观测、安全边界 | 部分完成 | step hash、duration、basic max_steps budget、run idempotency key、基础权限查询、V2 preview 鉴权代理、access log token 脱敏已实现 | Redis lock、retry、费用预算、图片次数预算、失败降级、签名 URL 未完成 |

### 2.3 Agent 分工和工作流

| 指南章节 | 要求 | 当前状态 | 已完成 | 未完成 / 风险 |
| --- | --- | --- | --- | --- |
| 6 Agent 分工 | Intent、Requirement、Memory、Prompt、Image、Vision、Review、Refiner、Artifact、Evolution | 部分完成 | 已实现真实 Intent/Requirement/Memory/Prompt/Image/Artifact Agent 第一版；Google Vision Review 后端第一版已接入；artifact/eval service 基础 | Requirement/Prompt 仍是规则型第一版；Refiner/Evolution Agent 未完成；OCR/版面检测未接 |
| 8.1 文生图工作流 | 文生图完整链路含追问、记忆、prompt、图片生成、artifact、review、refine | 部分完成 | V2 workflow `0.3.0` 已从 mock 切到真实 provider adapter + artifact version 写库，并支持真实 Google Vision Review 写 `quality_scores`；前端可展示 timeline、产物和 Review/Eval | 追问、人设记忆带入、自动 refine 尚未闭环；真实 Vision E2E 待代理/网络在线复验；OCR 未接 |
| 8.2 图生图 / 图片编辑 | 上传、视觉分析、mask、编辑模型、版本链 | 未完成 | Tool 接口预留 ImageEdit/Segmentation | 上传图 V2 流程、Segmentation、Image Edit Agent 未完成 |
| 8.3 品牌图 / 海报图 | 文字分层处理、HTML/Canvas 排版、Review | 未完成 | Prompt 结构里有 `render_text_separately` 字段 | HTML/Canvas Agent、中文文字 OCR 检查、可控排版未完成 |
| 8.4 候选图并行 | 3 个候选 prompt/图、逐张 review、排序、最佳图 | 部分完成 | artifact 支持 candidate group、rank_score、selected_at；selected feedback 和前端选择按钮已实现 | 并行生成、Ranker Agent、逐张真实 review、候选精排未完成 |

### 2.4 记忆系统

| 指南章节 | 要求 | 当前状态 | 已完成 | 未完成 / 风险 |
| --- | --- | --- | --- | --- |
| 7.1 记忆分层 | 短期、会话摘要、用户偏好、视觉风格、产物、工具经验、失败反思、评测记忆 | 部分完成 | 已定义 namespace：conversation、user_profile、visual_style、artifact_lineage、tool_experience、reflection | 分层写入策略还未接真实 Agent；评测记忆未完整实现 |
| 7.2 扩展 `context_memories` | namespace/source/artifact/tags/confidence/embedding/expires/use_count | 已完成基础字段 | model 和 AutoMigrate 已补字段；Memory Service 可查询、写入、删除 | `tags` 当前为 `tags_json`；未接向量库 |
| 7.3 写入策略 | 只写稳定偏好、用户选择、高分、失败模式 | 已完成第二版 | Memory Service 可写入；selected artifact 会写 feedback；`kind=memory_proposal` 候选记忆已接入；同 user/namespace/scope 的重复 proposal 会合并而不是重复写入；新增 proposal 晋级稳定记忆能力 | 仍未做复杂冲突降权和人工确认前端 |
| 7.4 检索策略 | 会话、用户偏好、视觉风格、失败经验组合检索 | 部分完成 | 支持 user/conversation/namespace/scope/limit 查询；新增 Prompt 上下文检索，只加载非 proposal 且高置信的 `visual_style/user_profile` 记忆，并在 Prompt Agent 中拼入 positive prompt | 语义检索、tag 检索、模型经验检索未完成 |
| 7.5 冲突处理 | 同 scope 冲突降权旧记忆 | 部分完成 | 同 scope 的 `memory_proposal` 会合并，避免单个 artifact 反复产生重复候选 | 稳定记忆之间的冲突降权、ranker/conflict resolver 未完成 |

### 2.5 产物、版本和反馈

| 指南章节 | 要求 | 当前状态 | 已完成 | 未完成 / 风险 |
| --- | --- | --- | --- | --- |
| 9 产物与版本管理 | artifact version 记录 prompt、模型、参数、source、quality、feedback | 部分完成 | `artifact_versions`、`artifact_feedback` model 已有；Artifact Agent 已把 prompt、negative prompt、模型、参数、object key 写入 version；V2 feedback API 已接入；真实 Google Vision 或 mock review 会写 `quality_scores`；feedback/review 会写 `memory_proposal` | edit/version parent 未跑通；OCR/版面质量分未完成 |
| 13 Phase 1 图片产物血缘 | artifact_versions、artifact_group_id、rank_score、selected_at、step observability | 部分完成 | 字段、基础 service、真实链路写入、前端 artifact board/versions/feedback/选择/Review 展示已完成 | 图片 provider 当前不保证真实生成 3 张候选；逐张真实 review/rank 未完成 |
| 11.1/11.3 进化数据来源和新增表 | feedback、prompt versions、reflections | 部分完成 | `agent_prompt_versions`、`agent_reflections`、`artifact_feedback` 已有；低分 reflection draft 已实现 | eval_cases/eval_runs 未实现；prompt promote/rollback 未实现 |

### 2.6 Tool / Provider 抽象

| 指南章节 | 要求 | 当前状态 | 已完成 | 未完成 / 风险 |
| --- | --- | --- | --- | --- |
| 10 Provider 抽象升级 | Text/ImageGen/ImageEdit/Vision/Segmentation 分接口 | 部分完成 | Tool Registry 已定义 Text、ImageGeneration、ImageEdit、Vision、OCR、Segmentation、Safety 接口；新增旧 `HTTPProvider.Chat/Generate` 到 V2 `TextProvider`/`ImageGenerationProvider` 的 adapter；Google Gemini Vision provider 已接 `KindVision` | ImageEdit/OCR/Segmentation 仍未接真实 provider |
| 10.1 图片工具链 | 接图片生成、VLM、OCR，后续 GroundingDINO/SAM | 部分完成 | 已接 Google Imagen 图片生成和 Google Gemini Vision Review 第一版 | OCR/GroundingDINO/SAM 均未接入 |
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
| 12 前后端 API 建议 | run events、artifact 操作、memory 操作 | 部分完成 | `/api/v2/conversations/:id/runs` 已执行真实 workflow；新增 `/api/v2/conversations/:id/runs/async` 进程内异步第一版；已补 artifacts list、versions、preview、download、feedback、select；memory search/delete/promote 已有 | edit、memory patch、持久化队列/真正增量事件流未完成 |
| 16 前端体验建议 | 对话 + 产物 + 过程 + 版本 + 反馈 | 部分完成 | 新增 `/workspace` V2 Workspace 第二批：输入、模型选择、运行、timeline、artifact board、版本、鉴权预览、下载、反馈、选择、Memory、Review/Eval | 编辑/重生成入口、候选对比精排未完成 |
| 21.10 前端工作台 | 记忆查看、候选图、版本、选择、下载、Review/Eval | 部分完成 | V2 工作台已支持候选图列表、版本、下载、反馈、选择按钮、Memory 查看/删除、Review/Eval 面板 | 编辑/重生成入口、候选精排未完成 |

### 2.9 安全、权限和合规

| 指南章节 | 要求 | 当前状态 | 已完成 | 未完成 / 风险 |
| --- | --- | --- | --- | --- |
| 17 安全权限合规 | artifact 预览/下载/编辑校验 user/conversation，上传限制，object key 不可预测，日志脱敏 | 部分完成 | DAO/service 查询按 user_id；memory delete 按 user_id；V2 preview/download/feedback/list/version 均通过 user 范围校验；旧版和 V2 前端预览均改用带 token header 的 blob；V2 列表/版本响应隐藏 object key 和静态 preview；access log 跳过二进制响应并脱敏 token/query；静态 `/artifacts` 默认关闭，可通过 `AIAgent.Storage.StaticEnabled=true` 临时恢复 | 签名 URL、上传限制、安全审查、object key 随机策略未系统化 |
| 21.9 安全与合规 | 不暴露静态路径作为权限边界 | 已完成第一版 | 旧版 Chat 与 V2 Workspace 预览均走鉴权 API；`/artifacts` 静态路由改为配置开关，当前默认关闭 | 短期签名 URL 未做 |

### 2.10 实施计划和 MVP

| 指南章节 | 要求 | 当前状态 | 已完成 | 未完成 / 风险 |
| --- | --- | --- | --- | --- |
| 18 第0周整理基线 | 梳理当前表使用，step timeline，artifact 可追溯 | 部分完成 | step observability 字段、真实 timeline、真实图片 artifact/version 追溯第一批已完成 | 仍需补真实 DB 集成记录和长期可用性记录 |
| 18 第1周产物版本化 | 3 候选图、选择第2张、feedback/selected | 部分完成 | candidate group、selected feedback 后端能力、V2 feedback API、前端版本/下载/反馈/选择按钮已完成 | 图片 provider 当前不保证 3 张候选；候选精排未完成 |
| 18 第2周记忆 MVP | 用户偏好自动带入、可删除覆盖 | 已完成后端第一版 | namespace 查询/删除已完成；前端 Memory 查看/删除入口已完成；feedback/review 会写入 `memory_proposal` 候选记忆；候选记忆可去重合并并通过 API 晋级稳定记忆；Prompt Agent 会自动带入高置信稳定偏好 | 人工确认前端、语义去重和复杂冲突降权未完成 |
| 18 第3周 Vision Review | VLM/OCR 质量检查，低分问题写入 version | 部分完成 | mock review 已接入 workflow `0.3.0`；Google Gemini Vision Review 后端已接入并写入 `artifact_versions.quality_scores`；low-score reflection draft 和 memory proposal 已完成 | OCR/版面检测和真实 Vision E2E 复验未完成 |
| 18 第4周自动 Refine | Review 低分自动二次 prompt，最多重试 | 未完成 | 暂无 | Refiner Agent 和 retry budget 未实现 |
| 18 第5周进化闭环 | Top 5 失败原因，prompt version 回滚 | 部分完成 | reflection/prompt version 表、低分 draft 基础、review/feedback 到 memory proposal、proposal 晋级稳定记忆已完成 | Top 5 聚合、prompt promote/rollback、自动晋级策略未完成 |
| 19 MVP 技术决策 | 固定 DAG、Redis/Asynq 长任务、MySQL 记忆、复用 provider、先反馈反思 | 部分完成 | 固定 DAG、MySQL 记忆、旧 provider 复用、反馈/反思基础已完成；已新增异步 Run 设计文档；`/runs/async` 进程内后台执行第一版已接入 | Redis/Asynq 持久队列、worker 抢占/重试、真实外部模型运行稳定性未完整验收 |

## 3. 第14节第一轮开发顺序进度

| 序号 | 任务 | 状态 | 已完成 | 未完成 / 备注 |
| --- | --- | --- | --- | --- |
| 1 | 补齐 model 和 AutoMigrate | 已完成 | 扩展 `agent_runs`、`agent_steps`、`context_memories`、`artifacts`；新增 `task_ledger_items`、`tool_invocations`、`memory_events`；更新 AutoMigrate 清单 | 未做独立 SQL migration，当前沿用项目 AutoMigrate |
| 2 | 补齐 v2 DAO | 已完成 | 拆分 `run/step/artifact/memory/tool/eval/ledger` DAO；补权限范围查询方法 | 还未做真实 DB 集成测试 |
| 3 | Runtime 支持顺序 DAG | 已完成 | `workflow.DAG`、依赖排序、循环/缺失依赖检测；Executor 使用 `OrderedNodes()` | 暂不做并行节点、恢复、重试 |
| 4 | Artifact Service MVP | 已完成第一批 | 支持 artifact + version 创建、candidate group、选择产物、预览/下载鉴权入口、feedback 写入入口、review quality_scores 写入；V2 list/version/preview/download/feedback/select API 已接上 | edit/version parent 未完成 |
| 5 | Tool Registry MVP | 已完成 | 支持按 kind/model_config_id 注册和查找工具；定义 Text/Image/Vision/OCR/Segmentation/Safety provider 接口；旧文本/图片 provider 已包装进 V2 tools | ImageEdit/Vision/OCR/Segmentation 真实 provider 未接入 |
| 6 | Memory Service MVP | 已完成后端第二版 | 支持 namespace 查询、MarkUsed、写入、软删除、memory event；支持 proposal 去重合并、Promote 晋级和 Prompt 高置信记忆检索 | 尚未接向量检索和复杂冲突降权 |
| 7 | 文生图真实链路 | 已完成第二批 | 实现真实 Requirement、Prompt、Image Generation、Artifact Agent；`/api/v2/conversations/:id/runs` 已执行 workflow `0.3.0`，调用真实 provider adapter、写 artifact/version，并接真实 Google Vision 或 mock review 写 quality_scores | Requirement/Prompt 仍是规则型；前端真实 `/workspace` 待冒烟；OCR/refine 未闭环 |
| 8 | v2 API 第一批 | 已完成第一批 | 已有 run 创建/查询/events；新增 `/runs/async`；新增 memories 查询/删除/promote、artifact select；补齐 artifacts list、versions、preview、download、feedback；run 创建已从 mock workflow 切到真实 workflow；新增 `/runs/:id/cancel` 取消入口 | edit、memory patch、持久队列未完成 |
| 9 | 前端 V2 Workspace 第二批 | 已完成 | `/workspace` 支持输入、模型选择、运行、timeline、artifact board、versions、鉴权预览、download、feedback、选择按钮、Memory 入口、Review/Eval 面板 | 编辑/重生成、候选精排未完成 |
| 10 | 权限校验 | 部分完成 | run 按 user 校验；artifact service/DAO 按 user 校验；memory 删除按 user 校验；V2 preview/download/feedback/list/version 按 user 校验；旧版 Chat preview 也已迁移到鉴权 API；access log 脱敏 token/query 并跳过二进制响应体；静态 `/artifacts` 默认关闭 | 签名 URL 未完成 |
| 11 | 测试和文档同步 | 进行中 | 已补 model、AutoMigrate、DAO、DAG、Artifact、Memory、Tool、Budget、Idempotency、Review、Reflection、Provider Adapter、Image/Artifact Agent、RunState 合并、review quality_scores、workflow review 节点测试；新增本文档、全量指南对齐清单、异步 Run 设计文档 | 后续每次开发后继续更新本文档 |

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

当前验收结论：代码链路第二批已通过；真实外部模型调用仍需要在有可用图片模型配置和网络环境下做人工端到端验收。

| 验收项 | 状态 | 说明 |
| --- | --- | --- |
| `go test ./...` 通过 | 已通过 | 后端全量测试通过 |
| `npm run build` 通过 | 已通过 | 前端构建通过 |
| 每个 run 有 step timeline | 已通过 | 真实 workflow `0.3.0` 会写 intent、requirement、memory、prompt、image_generation、artifact、vision_review step |
| 每个 step 有耗时和 input/output hash | 已具备基础能力 | Executor 已写 `duration_ms`、`input_hash`、`output_hash` |
| 每个 artifact 至少一个 version | 已通过代码验收 | Artifact Agent 会创建 artifact 和 artifact_version，写入 prompt、negative prompt、模型、参数和 object key |
| artifact 下载校验 user_id | 已通过代码验收 | 新增 `GET /api/v2/artifacts/:id/download`，通过 Artifact Service owner check 后返回文件 |
| feedback 可写入 | 已通过代码验收 | 新增 `POST /api/v2/artifacts/:id/feedback`，写入前校验 artifact owner |
| artifact 预览校验 user_id | 已通过代码验收 | 新增 `GET /api/v2/artifacts/:id/preview`，通过 Artifact Service owner check 后内联返回文件；前端用 token header fetch blob |
| Review quality_scores 写入 | 已通过代码验收 | workflow `0.3.0` 接入 `vision_review_agent`，run 完成后把 review 结果写入 `artifact_versions.quality_scores` |
| Prompt Agent 生成 prompt | 已完成第一版 | Prompt Agent 从结构化需求生成 positive/negative/render_text_separately/params；后续接文本模型增强 |
| Image Agent 调用图片 provider | 已完成第一版 | 旧 `HTTPProvider.Generate` 已包装为 V2 `ImageGenerationProvider`，Image Agent 通过 Tool Registry 调用 |
| 前端展示 timeline 和图片 | 已完成第二版 | `/workspace` 展示 timeline、artifact board、version、鉴权预览、download、feedback、选择、Memory、Review/Eval |

## 5. 第16节第二轮开发和验收状态

当前验收结论：后端能力切片已完成并通过测试；本轮已把 artifact 鉴权预览、review quality_scores 和前端第二批体验接入 V2 工作台。

| 能力 | 状态 | 已完成 | 未完成 / 备注 |
| --- | --- | --- | --- |
| Memory namespace 查询、删除和晋级 | 已完成后端第二版 | `Memory Service` 支持 namespace/scope/conversation 查询、MarkUsed、软删除、proposal 去重合并、Prompt 高置信记忆检索；新增 `/api/v2/memories`、`/api/v2/memories/search`、`DELETE /api/v2/memories/:id`、`POST /api/v2/memories/:id/promote`；前端 Memory 入口已接入查询/删除 | promote 前端入口、语义去重、复杂冲突降权未做 |
| candidate group | 已完成 | `Artifact Service` 支持 `CreateCandidateGroup`，同一轮候选图共享 `artifact_group_id`；真实 Artifact Agent 已调用 | provider 当前不保证一次真实返回 3 张候选 |
| selected artifact | 已完成 | `Artifact Service.SelectArtifact` 更新 `selected_at` 并写入 `artifact_feedback(selected)`；新增 `/api/v2/artifacts/:id/select`；前端选择按钮已接入 | 候选精排未做 |
| vision review | 已完成后端第一版 | 新增 `MockVisionReviewAgent` 作为降级；新增真实 `VisionReviewAgent` 和 Google Gemini Vision provider，能对 artifact 图片给分、输出 issues/should_refine，并写入 `artifact_versions.quality_scores` | 真实 Vision E2E 待代理/网络在线后复验；OCR/版面检测未做 |
| V2 preview 鉴权代理 | 已完成第二版 | 新增 `GET /api/v2/artifacts/:id/preview`；前端通过带 token header 的 blob URL 预览；V2 列表/版本响应隐藏 object key 和静态 preview；access log 跳过预览/下载二进制响应体并脱敏 token/query；旧版 Chat preview 也已迁移到鉴权 API；静态 `/artifacts` 默认关闭 | 签名 URL 未做 |
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
| 每阶段可运行、可验收 | 部分对齐，后端测试和前端构建已跑；第15主链路代码闭环已完成，V2 preview/Review/Memory 前端第二批已接入，但外部真实模型端到端需人工配置验收 |

当前主要偏差风险：
- 真实图片 provider 已接入代码链路，但未在当前环境使用真实凭据跑一次外部端到端。
- V2 工作台和旧版 Chat 预览均已走鉴权代理，静态 `/artifacts` 默认关闭；后续仅在需要外链分享时补短期签名 URL。
- Review 已有真实 Google Vision 后端接入和 memory proposal 写入，但复杂 OCR、自动 Refine、Evolution 聚合仍是后续闭环，不能长期停在“能评价但不自动迭代”的状态。

## 7. 下一步建议

本轮已完成上一版“下一步建议”中的 1-6：

| 序号 | 建议 | 状态 | 本轮结果 | 未完成 / 后续 |
| --- | --- | --- | --- | --- |
| 1 | 包装旧文本 provider 为 `tools.TextProvider` | 已完成 | 新增 `LegacyProviderAdapter.GenerateText` | Prompt Agent 尚未默认调用文本 provider |
| 2 | 包装旧图片 provider 为 `tools.ImageGenerationProvider` | 已完成 | 新增 `LegacyProviderAdapter.GenerateImage`，保存对象并返回 V2 图片元数据 | 真实外部模型端到端需凭据验收 |
| 3 | 实现真实 Requirement/Prompt/Image/Artifact Agent | 已完成第一版 | 新增规则型 Requirement/Prompt、Registry 驱动 Image、Artifact version 写库 Agent | Refiner/Evolution/Vision/OCR 未完成 |
| 4 | `/api/v2/conversations/:id/runs` 生成真实 artifact 和 artifact_version | 已完成第二版 | run 创建已执行 workflow `0.3.0`，返回 artifacts，并接 mock review 写 quality_scores | 长任务异步化实现未完成 |
| 5 | 补齐 V2 artifact API：list、versions、download、feedback | 已完成 | 新增 list、versions、preview、download、feedback、select 路由和 controller/service 方法 | edit 未完成；签名 URL 未完成 |
| 6 | 新建前端 V2 Workspace 第一批 | 已完成第二版 | `/workspace` 支持输入、模型选择、运行、timeline、artifact board、versions、鉴权预览、download、feedback、选择、Memory、Review/Eval | 编辑/重生成未完成 |

新的下一步建议完成状态：

| 序号 | 建议 | 状态 | 本轮结果 | 未完成 / 后续 |
| --- | --- | --- | --- | --- |
| 1 | 用真实图片模型配置跑一次 `/workspace` 端到端，记录 provider、artifact/version、下载、feedback 的真实验收结果 | 已完成后端 E2E，前端待冒烟 | 真实模型参数已配置；`go test -tags googlee2e ./internal/service/agent_v2/app -run TestGoogleModelEndToEnd -v` 已通过，覆盖 DB 配置读取、会话创建、真实 run、artifact/version、preview/download 授权、select、feedback、review quality_scores 校验；本次使用 `user_id=1`、`text_model_config_id=5`、`image_model_config_id=6`，产出 `conversation_id=35`、`run_id=48`、`artifact_id=27`、`version_id=1`、provider=`google`、image_model=`imagen-4.0-ultra-generate-001`、bytes=`971755`、preview_url=`/api/v2/artifacts/27/preview` | 本轮检查时本地 8501/5173/5174 未监听，且 `127.0.0.1:22307` 代理未监听，未发起新的 UI 生成；后续需启动后端、前端和代理后从 `/workspace` 手工冒烟 |
| 2 | 给 V2 preview 增加鉴权代理或签名 URL，降低静态 `/artifacts` 直接暴露风险 | 已完成第二版 | 新增 `GET /api/v2/artifacts/:id/preview`；前端改为带 token header fetch blob；V2 列表/版本响应隐藏 object key 和静态 preview；预览/下载二进制响应不进 access log body，token/query 已脱敏；旧版 Chat 新增 `GET /api/artifacts/:id/preview` 并迁移到鉴权 blob 预览；`/artifacts` 静态路由默认关闭 | 签名 URL 未做 |
| 3 | 接入真实 Vision/OCR Review，把质量分写入 `artifact_versions.quality_scores` | 已完成后端接入 | workflow `0.3.0` 已接 mock `vision_review_agent` 并写入 `artifact_versions.quality_scores`；本轮新增真实 `VisionReviewAgent`，可从 Tool Registry 调用 `VisionProvider` 并输出 `overall_score/issues/should_refine`；新增 Google Gemini Vision provider，支持 OpenAI-compatible multimodal chat，把本地 artifact 图片转 data URL 并解析 JSON 评分；`CreateRun` 会自动查找 `capability=vision` 的 Google 文本模型配置并注册 `KindVision`，有配置时 workflow 切到真实 review，无配置时回退 mock；前端 Review/Eval 可展示 | 真实外部 Vision E2E 待本地代理/网络在线后重跑 `googlee2e` 验证；复杂 OCR/版面检测仍未做 |
| 4 | 补前端 artifact 选择按钮、Memory 入口、Review/Eval 面板 | 已完成 | `/workspace` 已接选择按钮、Memory 查询/删除入口、Review/Eval 面板和质量分展示 | 编辑/重生成、候选精排未做 |
| 5 | 设计 V2 长任务异步化（Redis/Asynq 或现有任务队列），避免真实图片模型阻塞 HTTP 请求 | 已完成后端第二版 | 新增 [IMAGE_AGENT_V2_ASYNC_RUN_DESIGN.md](./IMAGE_AGENT_V2_ASYNC_RUN_DESIGN.md)，明确状态模型、API、队列、幂等、worker、前端轮询和验收；本轮新增 `POST /api/v2/conversations/:id/runs/async`，创建 run 后标记 `queued` 并用后台 goroutine 执行现有 workflow，复用 `GET /api/v2/runs/:id` 和 `/events` 查询进度；新增 `POST /api/v2/runs/:id/cancel`，可取消 `created/queued/running/waiting_user` 状态 run，executor 每步前后检查 `cancelled`，避免取消后继续推进到 `completed` | 当前是进程内后台执行，不是 Redis/Asynq 持久队列；worker 抢占、重试和前端轮询接入未做 |
| 6 | 做 feedback/review 到 memory proposal 的闭环，避免用户选择和低分 review 只停留在单次记录 | 已完成第三版 | 新增 Memory Service proposal 能力：artifact selected/positive/negative/rating/comment 会写入 `context_memories`，以 `kind=memory_proposal` 标记；正向/选择反馈进入 `visual_style`，负向反馈和低分 review 进入 `reflection`；`memory_events` 会记录 created/merged/promoted 事件并保留 `agent_run_id/source/artifact`；同 scope proposal 会合并；新增 `POST /api/v2/memories/:id/promote` 晋级稳定记忆；CreateRun 会把高置信稳定偏好带入 Prompt Agent；前端 Memory 面板已对 `memory_proposal` 显示“候选”标记并提供人工确认按钮 | 语义去重、复杂冲突降权和自动晋级策略未做 |

下一步建议：

1. 用同一真实 Google 配置从前端 `/workspace` 手工发起一次生成，确认 artifact board、preview、download、feedback、Review/Eval 面板展示与后端 E2E 结果一致。
2. 在代理/网络在线时重跑 `go test -tags googlee2e ./internal/service/agent_v2/app -run TestGoogleModelEndToEnd -v`，确认真实 Google Vision review 也写入 `artifact_versions.quality_scores`。
3. 将 `/runs/async` 从进程内 goroutine 升级为 Redis/Asynq 或项目现有持久队列，实现 worker 抢占、重试和前端轮询接入。
4. 继续完善 `memory_proposal`：语义去重、稳定记忆冲突降权和自动晋级策略。
5. 后续如需要外链分享，再实现 artifact 短期签名 URL；当前前端预览已统一走鉴权 API，静态 `/artifacts` 默认关闭。

## 8. 更新日志

| 日期 | 任务 | 更新内容 | 验收 |
| --- | --- | --- | --- |
| 2026-05-25 | 第14节第一轮基础能力 | 补齐 V2 model/AutoMigrate、DAO、顺序 DAG、Artifact/Tool/Memory MVP service | `go test ./...` 通过 |
| 2026-05-25 | 第15节验收 | 明确第15节未完全通过，真实图片链路和 V2 前端仍缺失 | `go test ./...`、`npm run build` 通过 |
| 2026-05-25 | 第16节后端能力 | 完成 Memory 查询/删除、candidate group、selected artifact、vision review mock、reflection draft、basic budget、idempotency key，并接入部分 V2 API | `go test ./...`、`npm run build`、`git diff --check` 通过 |
| 2026-05-25 | 进度文档 | 新增本文档，后续每次任务完成后必须更新 | 文档已创建 |
| 2026-05-25 | 全量指南对齐 | 将进度记录范围扩展到 `IMAGE_AGENT_DEVELOPMENT_GUIDE.md` 全部开发要求，不再只对齐 V2 末尾任务 | 文档已更新 |
| 2026-05-25 | 下一步建议 1-6 | 完成旧文本/图片 provider adapter、真实 Requirement/Prompt/Image/Artifact Agent、V2 run 真实 artifact/version 写库、artifact list/version/download/feedback API、前端 `/workspace` 第一批 | `go test ./...`、`npm run build`、`git diff --check` 通过；真实外部图片模型端到端待凭据验收 |
| 2026-05-26 | 新的下一步建议 | 完成 V2 preview 鉴权代理、access log 脱敏和二进制跳过、workflow `0.3.0` mock review 入主链路并写 `quality_scores`、前端选择按钮/Memory/Review 面板、异步 Run 设计文档 | `go test ./...`、`npm run build`、`git diff --check` 通过 |
| 2026-05-26 | Google 模型默认配置 | 明确默认使用 Gemini 3.5 Flash 作为文本/多模态模型，使用 Imagen 4 Ultra 作为最高质量图片生成模型；新增 Google Imagen 原生 `:predict` 图片 provider 分支，模型配置落库到 `model_configs.config_info` | `go test ./internal/service/agent_svc` 通过 |
| 2026-05-26 | Google 三 Key 隔离配置文档 | 新增 `GOOGLE_MODEL_CONFIG.md`，将 Gemini 文本、Imagen 出图、Gemini Vision 拆成 3 个独立 API Key 和 3 条 `model_configs` 配置，降低单能力限流对其他能力的影响 | 文档已创建 |
| 2026-05-26 | 真实 Google E2E 验收入口 | 新增 build tag 集成测试 `internal/service/agent_v2/app/google_e2e_test.go`，显式执行时会读取当前数据库 Google 配置并验证真实 run、artifact/version、preview/download、select、feedback、quality_scores；测试会输出选中的 user/model_config ID 便于替换正式 Key | `go test -tags googlee2e ./internal/service/agent_v2/app -run ^$` 编译通过；真实外部调用已能到达 Google API，但当前 Key 返回 `API_KEY_INVALID` |
| 2026-05-26 | Google E2E Key 诊断增强 | E2E 日志新增 Google model config 的非敏感摘要：模型名、request_url、base_url、api_type、capability、api_key_length、api_key_sha256 前 12 位，便于确认数据库实际读取的 Key 是否已更新 | `go test -tags googlee2e ./internal/service/agent_v2/app -run ^$` 编译通过 |
| 2026-05-26 | Google Imagen 真实后端 E2E 通过 | 使用正式 Google 配置跑通 `TestGoogleModelEndToEnd`，真实调用 Imagen 4 Ultra 生成图片并完成 artifact/version、preview/download、select、feedback、review quality_scores 验收 | `go test -tags googlee2e ./internal/service/agent_v2/app -run TestGoogleModelEndToEnd -v` 通过；结果：`conversation_id=35`、`run_id=48`、`artifact_id=27`、`version_id=1`、bytes=`971755` |
| 2026-05-26 | 真实 Google Vision Review 后端接入 | 新增真实 `VisionReviewAgent` 和 Google Gemini Vision provider；provider 通过 OpenAI-compatible multimodal chat 分析本地 artifact 图片，解析 summary/overall_score/issues/should_refine；workflow 在存在 `capability=vision` 配置时自动切到真实 review，否则回退 mock | `go test ./internal/service/agent_v2/agents ./internal/service/agent_v2/tools ./internal/service/agent_v2/workflow ./internal/service/agent_v2/app ./internal/service/agent_v2/runtime -count=1` 通过 |
| 2026-05-26 | feedback/review memory proposal 闭环 | 新增 `MemoryService.ProposeFromArtifactFeedback` 和 `ProposeFromReview`；artifact 选择、显式反馈、低分/需 refine 的 review 会写入 `context_memories`，用 `kind=memory_proposal` 保持候选状态，并在 `memory_events` 记录来源 | `go test ./internal/service/agent_v2/memory ./internal/service/agent_v2/app -count=1` 通过 |
| 2026-05-26 | 异步 Run 后端第一版 | 新增 `CreateRunAsync`、`POST /api/v2/conversations/:id/runs/async`；接口只创建 message/run、标记 `queued` 并立即返回，后台 goroutine 继续执行现有 workflow，完成后仍写 step、artifact、review、assistant message | `go test ./internal/service/agent_v2/app ./internal/controller/agent_v2_ctrl ./routers -count=1` 通过 |
| 2026-05-26 | memory proposal 去重、晋级和 Prompt 带入 | 新增同 scope proposal 合并、`PromoteProposal`、`POST /api/v2/memories/:id/promote`；CreateRun 会检索高置信稳定 `visual_style/user_profile` 记忆并带入 Prompt Agent，proposal 草稿不会直接影响 prompt | `go test ./internal/service/agent_v2/memory ./internal/service/agent_v2/app ./internal/controller/agent_v2_ctrl ./routers ./internal/dao/agent_v2_dao -count=1` 通过 |
| 2026-05-26 | Git 缓存清理 | 将误跟踪的 `.gocache/` 从 Git 索引移除，并新增 `.gitignore` 忽略规则，后续 Go 测试缓存只保留在本地工作区 | `git rm -r --cached .gocache` 已执行；本地缓存文件未删除 |
| 2026-05-26 | 后端全局响应封装 | 将成功响应码统一为 `200`，保留全局 `{code,msg,data}` 响应壳；分页响应改为 `data.list/page/page_size/total/total_pages`，并新增响应层单元测试覆盖普通成功响应和分页查询响应 | `go test ./pkg/responses ./pkg/errcode ./... -count=1` 通过 |
| 2026-05-26 | 前端简约黑白主题 | 保持 logo 标记位置不变，将前端主视觉收敛为白色背景、黑白控件和浅蓝标题；同步前端 API 成功码兼容 `code=200`，避免后端全局响应封装后误判成功请求 | `npm run build` 通过；`http://127.0.0.1:5174/workspace` 返回 200 |
| 2026-05-27 | 异步 Run 取消闭环 | 新增 `POST /api/v2/runs/:id/cancel`；queued/running/created/waiting_user 状态可被标记为 `cancelled`，并写入 `cancelled_at`；runtime executor 在启动、每步执行前后和完成前检查 run 状态，避免取消后继续生成 completed 结果 | `go test ./internal/service/agent_v2/runtime ./internal/service/agent_v2/app ./internal/controller/agent_v2_ctrl ./routers ./internal/dao/agent_v2_dao -count=1` 通过 |
| 2026-05-27 | memory proposal 前端确认入口 | `/workspace` Memory 面板对 `kind=memory_proposal` 的候选记忆显示“候选”标记，并新增“确认”按钮调用 `POST /api/v2/memories/:id/promote`，确认后刷新记忆列表 | `npm run build` 通过 |
| 2026-05-27 | 静态 artifact 入口收敛 | 新增旧版 `GET /api/artifacts/:id/preview` 鉴权预览；旧版 Chat 与 V2 Workspace 均使用 token header 获取 blob URL，不再依赖静态 `/artifacts`；`AIAgent.Storage.StaticEnabled` 默认 `false`，静态路由只在显式开启时注册 | `go test ./internal/controller/agent_ctrl ./routers ./config -count=1`、`npm run build` 通过 |

## 9. Google 模型数据库配置

本轮默认模型：

- 文本 / 多模态：`gemini-3.5-flash`，用于规划、记忆、Prompt、评审。
- 图片生成：`imagen-4.0-ultra-generate-001`，用于最高质量出图。

配置存放位置：

- 数据库连接配置：`etc/config.yaml` 的 `DB.Host`、`DB.Port`、`DB.Database`、`DB.Username`、`DB.Password`。
- 全局模型配置表：`model_configs`。
- API Key 存放字段：`model_configs.config_info.api_key`。
- 用户默认选择表：`user_model_configs` 的 `selected_text_model_config_id`、`selected_image_model_config_id`。
- 用户权限表：`user_model_permissions` 的 `user_id`、`model_config_id`、`can_use`。

推荐落库约定：

- Gemini 3.5 Flash 使用 Google OpenAI-compatible base URL：`https://generativelanguage.googleapis.com/v1beta/openai`。
- Imagen 4 Ultra 使用 Google Imagen 原生 base URL：`https://generativelanguage.googleapis.com/v1beta`，运行时拼接为 `/models/imagen-4.0-ultra-generate-001:predict`，鉴权 header 为 `x-goog-api-key`。
