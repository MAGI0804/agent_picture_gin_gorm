# 图片 AI Agent 项目具体规划

## Summary

基于 `C:\Users\易理志\Desktop\Yl_Agent_picture\gin_agent_gorm` 作为 Go 后端脚手架，建设一个图片 AI Agent 项目。前端使用 Vue 3 + Vite，后端使用 Gin + GORM，云端 MySQL 存储业务数据与上下文，Redis 用于缓存、任务状态和队列，对象存储保存图片、HTML 等生成文件。系统支持双输入框对话、多 agent 后端编排、上下文记忆、右侧产物预览和下载。

## Backend Plan

- 复用 `gin_agent_gorm` 现有结构：`routers` 注册接口，`internal/controller` 处理 HTTP，`internal/service` 编排业务，`internal/dao` 访问 GORM，`model` 定义数据表模型。
- 新增业务模块：会话、消息、补充问题、agent run、agent step、上下文记忆、产物文件。
- 复用现有 MySQL、Redis、JWT、Asynq、日志、中间件能力。
- 新增 Provider 抽象层，统一封装文本模型、图片生成模型、图片编辑模型、HTML 生成能力。
- 多 agent 由后端编排，首版固定流程为：Planner Agent、Context Agent、Prompt Agent、Image Agent、HTML Agent、Review Agent、Artifact Agent。

## Frontend Plan

- 在项目根目录规划 `frontend`，使用 Vue 3 + Vite + TypeScript。
- 页面采用三栏布局：左侧会话列表，中间对话区，右侧产物预览区。
- 中间对话区提供两个输入框：
  - 补充问题回答框：回答上一轮 assistant 产生的问题。
  - 正常对话框：提交新的图片、HTML 或对话需求。
- 右侧产物区支持图片预览、HTML iframe 预览、文件信息展示和下载按钮。
- 对话过程通过 SSE 或 WebSocket 展示 agent 执行进度和流式输出。

## Data Model

- `users`：用户账号与权限。
- `conversations`：会话。
- `messages`：用户消息、assistant 消息、系统消息。
- `follow_up_questions`：上一轮输出后生成的补充问题。
- `agent_runs`：一次用户请求触发的 agent 总任务。
- `agent_steps`：各 agent 子步骤输入、输出、状态、错误信息。
- `context_memories`：长期上下文摘要、偏好、历史任务索引。
- `artifacts`：生成图片、HTML、JSON、Markdown 等文件元数据。
- 文件本体不进入 MySQL，保存到 S3 兼容对象存储，本地只保存对象 key、MIME、大小、hash、归属用户和会话。

## API Plan

- `POST /api/auth/login`：登录。
- `GET /api/conversations`：会话列表。
- `POST /api/conversations`：创建会话。
- `GET /api/conversations/:id/messages`：获取消息。
- `POST /api/conversations/:id/messages`：提交普通输入或补充问题回答。
- `GET /api/conversations/:id/artifacts`：获取会话产物。
- `GET /api/artifacts/:id/download`：下载产物。
- `GET /api/runs/:id/events`：获取 agent run 流式事件。
- 消息提交字段包含：`input_type`、`content`、`answered_question_ids`、`attachments`。
- `input_type` 固定为 `normal` 或 `answer_to_questions`。

## Context And Agent Flow

- 每次请求先读取最近消息、会话摘要、用户偏好、相关产物元数据。
- Planner Agent 判断任务类型，并生成待确认问题或执行计划。
- 如果信息不足，assistant 返回问题并写入 `follow_up_questions`。
- 用户在第一个输入框回答后，后端将回答与问题绑定，再继续执行原任务。
- 如果用户在第二个输入框输入，则创建新的 agent run。
- 产物生成完成后写入对象存储，元数据入库，并推送给前端右侧预览区。

## Implementation Steps

- 新建 `AI_AGENT_PROJECT_PLAN.md` 到 `C:\Users\易理志\Desktop\Yl_Agent_picture`。
- 在 `gin_agent_gorm` 中新增 AI Agent 业务模型、DAO、Service、Controller 和路由。
- 扩展配置文件，加入模型 Provider、对象存储、agent 执行参数。
- 实现会话、消息、补充问题和产物 API。
- 实现 agent runner、固定 DAG 编排、agent step 状态持久化。
- 实现前端 Vue 工作台、双输入框、流式消息、右侧产物预览和下载。
- 增加基础测试和本地启动文档。

## Test Plan

- 后端测试：用户隔离、会话权限、消息类型分流、补充问题绑定、agent run 状态流转、产物下载鉴权。
- 集成测试：MySQL、Redis、对象存储、Provider mock 的完整生成链路。
- 前端测试：双输入框行为、流式输出、图片预览、HTML iframe 预览、下载按钮。
- 验收场景：用户提出图片需求，系统追问细节，用户回答后生成图片并在右侧展示；用户提出 HTML 需求后生成页面并可预览下载。

## Assumptions

- 后端脚手架固定使用 `C:\Users\易理志\Desktop\Yl_Agent_picture\gin_agent_gorm`。
- 计划 MD 文件放在 `C:\Users\易理志\Desktop\Yl_Agent_picture\AI_AGENT_PROJECT_PLAN.md`。
- 云端 MySQL 和 Redis 已可用或后续通过配置接入。
- 文件存储采用 S3 兼容对象存储。
- 首版使用一个实际模型 Provider，其他 Provider 只保留接口扩展点。
