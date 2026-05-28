# 当前项目状态快照

生成日期：2026-05-28  
分支：`Again`  
最新提交：`d81eae5 完成 Agent V2 默认入口迁移`

## 1. 总体结论

图片 AI Agent V2 已完成从后端主链路、异步执行、观测、候选图、Review/OCR、Refine、Memory、编辑、文字分层、Evolution/Eval、安全边界，到前端工作台组件化和默认入口迁移的第一版闭环。

当前默认前端入口已经迁移到 `/workspace`。旧 `/chat` 与旧 `/api` Agent 接口继续保留，用于历史会话和旧 artifact 兼容读取；旧 `internal/service/agent_svc.AgentService` 已标记 deprecated，新图片 Agent 能力应继续进入 `internal/service/agent_v2`、`internal/dao/agent_v2_dao`、`internal/controller/agent_v2_ctrl` 和 `/api/v2`。

## 2. 最新完成任务

| 任务 | 状态 | 最新提交 |
| --- | --- | --- |
| Task 14：安全、存储和合规边界 | 已完成第一版 | `da96c29` |
| Task 15：前端工作台组件化和体验补齐 | 已完成第一版 | `b1695fc` |
| Task 16：默认入口迁移、文档同步和旧代码收敛 | 已完成第一版 | `d81eae5` |

## 3. 当前核心能力

### 后端 V2

- 固定 DAG 工作流：Intent、Requirement、Memory、Prompt、Image Generation、Artifact、Poster Render、Vision Review/OCR、Ranker、Refiner、Safety。
- 异步 Run：`POST /api/v2/conversations/:id/runs/async` 创建 queued run，并由 Asynq worker 执行。
- 恢复与取消：支持 `waiting_user` 追问恢复、run cancel、completed step 复用、provider 临时错误重试、预算约束。
- 可观测性：`task_ledger_items`、`tool_invocations`、run events cursor 轮询已接入。
- Artifact 版本链：生成、上传、编辑、refine、文字分层均写入 artifact/version，并保留 parent/source refs。
- Review 与排序：支持逐候选 Vision/OCR Review、`quality_scores`、`rank_score`、候选推荐和用户选择。
- Memory：支持 proposal、晋级、语义去重、冲突降权、PromptContext ranker、前端查看/编辑/停用/确认。
- Evolution/Eval：支持失败 Top 5 聚合、prompt version draft、`draft -> review -> active -> archived` 状态流转、eval case/run 基础记录。
- 安全边界：V2 preview/download/edit/feedback/list/version 均按 user 范围校验；上传校验大小、MIME、像素、扩展名；SafetyProvider 进行文本前置和图片后置检查；object key 已随机化；LocalObjectStore 拒绝路径穿越；access log 脱敏 token、API key、prompt/content/messages，跳过上传和二进制体。

### 前端 V2

- 默认入口：根路径、登录成功、注册成功和设置页导航默认进入 `/workspace`。
- 工作台能力：输入、模型选择、异步运行、Timeline、Artifact Board、版本链、鉴权预览、下载、反馈、选择、Memory、Review/Eval、追问恢复、上传参考图、继续编辑、文字分层。
- 组件化：已拆出 `frontend/src/api`、`frontend/src/composables` 和 `frontend/src/components/workspace`。
- 体验补齐：支持 empty/queued/running/waiting_user/failed/cancelled/completed 状态展示、失败 run 重试、候选对比、刷新后恢复最近 run、长文本/长错误换行约束。

## 4. 入口和接口

| 类型 | 地址 / 路径 | 说明 |
| --- | --- | --- |
| V2 工作台 | `http://localhost:5173/workspace` | 当前默认入口 |
| 旧版兼容页 | `http://localhost:5173/chat` | 历史会话和旧 artifact 兼容 |
| 后端服务 | `http://localhost:8501` | Gin API |
| V2 API 前缀 | `/api/v2` | 新图片 Agent 能力 |
| V2 artifact 预览 | `/api/v2/artifacts/:id/preview` | 鉴权代理 |
| V2 artifact 下载 | `/api/v2/artifacts/:id/download` | 鉴权代理 |

## 5. 主要代码位置

| 范围 | 路径 |
| --- | --- |
| V2 应用服务 | `gin_agent_gorm/internal/service/agent_v2/app` |
| V2 agents | `gin_agent_gorm/internal/service/agent_v2/agents` |
| V2 runtime/workflow | `gin_agent_gorm/internal/service/agent_v2/runtime`、`workflow` |
| V2 artifact/memory/eval/security | `gin_agent_gorm/internal/service/agent_v2/artifact`、`memory`、`eval`、`security` |
| V2 controller/routes | `gin_agent_gorm/internal/controller/agent_v2_ctrl`、`gin_agent_gorm/routers/agent_v2_routes.go` |
| 前端 V2 页面 | `frontend/src/views/AgentWorkspaceV2View.vue` |
| 前端 V2 API client | `frontend/src/api/agentV2.ts`、`artifacts.ts`、`memories.ts` |
| 前端 V2 组件 | `frontend/src/components/workspace` |
| 前端 V2 composables | `frontend/src/composables` |

## 6. 最近验证结果

最近完成 Task 16 时已通过：

```bash
go test ./... -count=1
npm run build
git diff --check
```

Task 14 针对性验证也已通过：

```bash
go test ./internal/service/agent_v2/security ./internal/service/agent_v2/tools ./internal/service/agent_v2/agents ./internal/service/agent_v2/workflow ./internal/service/agent_v2/app ./internal/service/agent_v2/artifact ./internal/service/agent_svc ./internal/middleware ./internal/controller/agent_v2_ctrl ./routers -count=1
```

说明：本机 Git 仍会输出 `C:\Users\易理志/.config/git/ignore` 权限 warning 和 LF/CRLF warning，但未阻塞测试、提交或推送。

## 7. 已知风险和未完成项

- 前端 `/workspace` 使用真实 Google 配置的手工冒烟仍建议再跑一次，确认真实 UI 流程、预览、下载、反馈、Review/Eval 展示一致。
- 真实 Google Vision/OCR 外部 E2E 需要在代理和网络稳定时复验。
- ImageEditProvider 当前仍复用现有图片生成 provider 适配，原生 image-to-image、mask、SegmentationProvider 仍待后续模型适配。
- Eval run 当前先记录结果，不自动执行完整评测集。
- 签名 URL 未实现；当前策略是继续保持鉴权 preview/download API 为唯一入口，静态 `/artifacts` 默认关闭。
- 旧 `/chat` 和旧 `/api` 仍保留兼容，后续只有在确认历史会话和旧 artifact 迁移/兼容策略稳定后才适合删除。

## 8. 建议下一步

1. 启动后端、前端和必要代理，用正式 Google 配置从 `/workspace` 完成一次真实端到端手工验收。
2. 补原生 ImageEdit/Segmentation provider，减少编辑链路对文生图 provider 的复用。
3. 将 eval run 从“记录结果”升级为可执行评测集。
4. 如确实需要外链分享，再实现短期签名 URL；否则继续保持当前鉴权 API 策略。
5. 等历史会话和旧 artifact 兼容策略稳定后，再计划删除旧 Chat/旧 Agent 流程。
