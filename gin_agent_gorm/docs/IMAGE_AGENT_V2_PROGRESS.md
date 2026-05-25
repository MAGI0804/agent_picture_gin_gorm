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
| 第15节第一轮验收 | 未完全通过 | Mock run 和 step timeline 可跑，但真实文生图闭环、V2 前端、下载/反馈闭环未完成 |
| 第16节第二轮后端能力 | 已完成后端切片 | Memory 查询/删除、candidate group、selected artifact、vision review mock、reflection draft、basic budget、idempotency key 已实现 |
| 前端 V2 Workspace | 未完成 | 当前仍不是独立 V2 工作台 |
| 真实图片生成链路 | 未完成 | 还没有通过 V2 Image Agent 调用真实图片 provider 并写入 artifact version |

## 2. 第14节第一轮开发顺序进度

| 序号 | 任务 | 状态 | 已完成 | 未完成 / 备注 |
| --- | --- | --- | --- | --- |
| 1 | 补齐 model 和 AutoMigrate | 已完成 | 扩展 `agent_runs`、`agent_steps`、`context_memories`、`artifacts`；新增 `task_ledger_items`、`tool_invocations`、`memory_events`；更新 AutoMigrate 清单 | 未做独立 SQL migration，当前沿用项目 AutoMigrate |
| 2 | 补齐 v2 DAO | 已完成 | 拆分 `run/step/artifact/memory/tool/eval/ledger` DAO；补权限范围查询方法 | 还未做真实 DB 集成测试 |
| 3 | Runtime 支持顺序 DAG | 已完成 | `workflow.DAG`、依赖排序、循环/缺失依赖检测；Executor 使用 `OrderedNodes()` | 暂不做并行节点、恢复、重试 |
| 4 | Artifact Service MVP | 部分完成 | 支持 artifact + version 创建、candidate group、选择产物、下载鉴权入口、feedback 写入入口 | V2 artifact list/version/download/feedback API 第一批仍未全部接上 |
| 5 | Tool Registry MVP | 已完成 | 支持按 kind/model_config_id 注册和查找工具；定义 Text/Image/Vision/OCR/Segmentation/Safety provider 接口 | 还未包装旧真实 provider |
| 6 | Memory Service MVP | 已完成 | 支持 namespace 查询、MarkUsed、写入、软删除、memory event | 尚未接向量检索和冲突降权 |
| 7 | 文生图真实链路 | 未完成 | 暂无 | 需实现 Requirement、Prompt、Image Generation、Artifact Agent 的真实链路 |
| 8 | v2 API 第一批 | 部分完成 | 已有 run 创建/查询/events；新增 memories 查询/删除、artifact select | 缺 artifacts list、versions、download、feedback；run 创建仍执行 mock workflow |
| 9 | 前端 V2 Workspace 第一批 | 未完成 | 暂无 | 需新建 V2 工作台页面和 API client |
| 10 | 权限校验 | 部分完成 | run 按 user 校验；artifact service/DAO 按 user 校验；memory 删除按 user 校验 | V2 download/feedback 完整权限链路还未完成 |
| 11 | 测试和文档同步 | 进行中 | 已补 model、AutoMigrate、DAO、DAG、Artifact、Memory、Tool、Budget、Idempotency、Review、Reflection 单元测试；新增本文档 | 后续每次开发后继续更新本文档 |

## 3. 第15节第一轮验收状态

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

当前验收结论：未完全通过。

| 验收项 | 状态 | 说明 |
| --- | --- | --- |
| `go test ./...` 通过 | 已通过 | 后端全量测试通过 |
| `npm run build` 通过 | 已通过 | 前端构建通过 |
| 每个 run 有 step timeline | 已具备基础能力 | Mock workflow 可以写 step timeline |
| 每个 step 有耗时和 input/output hash | 已具备基础能力 | Executor 已写 `duration_ms`、`input_hash`、`output_hash` |
| 每个 artifact 至少一个 version | 未闭环 | Service 支持创建，但 V2 真实图片链路还未写入真实 artifact version |
| artifact 下载校验 user_id | 部分完成 | Service 有 `AuthorizeDownload`，但 V2 download API 未完成 |
| feedback 可写入 | 部分完成 | Service 有 `RecordFeedback`，selected 会写 feedback；V2 通用 feedback API 未完成 |
| Prompt Agent 生成 prompt | 未完成 | 当前是 mock prompt |
| Image Agent 调用图片 provider | 未完成 | 真实 provider 尚未进入 V2 Tool Registry 链路 |
| 前端展示 timeline 和图片 | 未完成 | V2 Workspace 未建 |

## 4. 第16节第二轮开发和验收状态

当前验收结论：后端能力切片已完成并通过测试。

| 能力 | 状态 | 已完成 | 未完成 / 备注 |
| --- | --- | --- | --- |
| Memory namespace 查询和删除 | 已完成 | `Memory Service` 支持 namespace/scope/conversation 查询、MarkUsed、软删除；新增 `/api/v2/memories`、`/api/v2/memories/search`、`DELETE /api/v2/memories/:id` | 前端入口未做 |
| candidate group | 已完成 | `Artifact Service` 支持 `CreateCandidateGroup`，同一轮候选图共享 `artifact_group_id` | 真实生成链路还未调用 |
| selected artifact | 已完成 | `Artifact Service.SelectArtifact` 更新 `selected_at` 并写入 `artifact_feedback(selected)`；新增 `/api/v2/artifacts/:id/select` | 前端选择按钮未做 |
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

## 5. 重写计划对齐情况

已确认 `IMAGE_AGENT_REWRITE_DEVELOPMENT_PLAN.md` 的方向：

| 计划要求 | 当前状态 |
| --- | --- |
| 保留外围能力，重写核心 Agent | 对齐 |
| 新能力走 `agent_v2`，不继续堆旧 `agent_svc` | 对齐 |
| Runtime 不写具体 prompt/图片业务 | 对齐 |
| Agent 不直接写 DB | 当前新增能力对齐 |
| Artifact / Memory / Tool / Eval 独立 service | 对齐 |
| 每阶段可运行、可验收 | 部分对齐，后端测试和前端构建已跑；第15主链路仍未完成 |

当前主要偏差风险：
- 第15节主链路还未闭环，不能继续长期堆第二/三轮能力而不接真实文生图。
- V2 API 第一批尚未补齐 artifact list/version/download/feedback。
- 前端仍未迁到 V2 Workspace。

## 6. 下一步建议

按当前进度，下一步优先级应为：

1. 包装旧文本 provider 为 `tools.TextProvider`。
2. 包装旧图片 provider 为 `tools.ImageGenerationProvider`。
3. 实现真实 `requirement_agent`、`prompt_agent`、`image_generation_agent`、`artifact_agent`。
4. 让 `/api/v2/conversations/:id/runs` 生成真实 artifact 和 artifact_version。
5. 补齐 V2 artifact API：list、versions、download、feedback。
6. 新建前端 V2 Workspace 第一批：输入、模型选择、运行、timeline、artifact board、下载。

## 7. 更新日志

| 日期 | 任务 | 更新内容 | 验收 |
| --- | --- | --- | --- |
| 2026-05-25 | 第14节第一轮基础能力 | 补齐 V2 model/AutoMigrate、DAO、顺序 DAG、Artifact/Tool/Memory MVP service | `go test ./...` 通过 |
| 2026-05-25 | 第15节验收 | 明确第15节未完全通过，真实图片链路和 V2 前端仍缺失 | `go test ./...`、`npm run build` 通过 |
| 2026-05-25 | 第16节后端能力 | 完成 Memory 查询/删除、candidate group、selected artifact、vision review mock、reflection draft、basic budget、idempotency key，并接入部分 V2 API | `go test ./...`、`npm run build`、`git diff --check` 通过 |
| 2026-05-25 | 进度文档 | 新增本文档，后续每次任务完成后必须更新 | 文档已创建 |
