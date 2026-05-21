# 图片 AI Agent 平台流程与接口文档

## 1. 平台概述

图片 AI Agent 平台基于 `gin_agent_gorm` 后端脚手架实现，核心能力包括：

- 用户登录与 JWT 鉴权。
- 会话管理。
- 双输入框对话流程。
- 多 Agent 固定流程编排。
- 上下文记忆存储。
- 图片、HTML 等产物生成、预览和下载。
- 模型配置透传，为后续接入真实模型 Provider 做准备。

当前版本使用 `mock Provider` 跑通完整链路：用户回答补充问题后，系统会生成一个 SVG 图片文件和一个 HTML 页面文件。

## 2. 页面操作流程

### 2.0 页面与端口绑定

本项目默认使用以下端口：

| 服务 | 地址 | 说明 |
| --- | --- | --- |
| 前端 Vite | `http://localhost:5173` | 页面入口 |
| 后端 Gin API | `http://localhost:8501` | API 服务 |

前端页面路由：

| 页面 | 前端地址 | 后端接口 |
| --- | --- | --- |
| 登录页 | `http://localhost:5173/login` | `POST /api/auth/login` |
| 注册页 | `http://localhost:5173/register` | `POST /api/auth/register/email-verify-code`、`POST /api/auth/register/using-email` |
| 对话页 | `http://localhost:5173/chat` | `/api/conversations`、`/api/runs/:id/events`、`/api/artifacts/:id/download` |
| 设置页 | `http://localhost:5173/settings` | 当前保存到浏览器 localStorage，并随消息请求透传 `model_config` |

Vite 代理配置：

```text
/api       -> http://localhost:8501
/artifacts -> http://localhost:8501
```

### 2.1 登录页

1. 打开前端地址：`http://localhost:5173`。
2. 输入账号或邮箱。
3. 输入密码。
4. 点击“登录”。
5. 登录成功后，前端会保存后端返回的 JWT token，并进入对话页。

对应后端接口：

```http
POST /api/auth/login
```

### 2.1.1 注册页完整流程

注册页地址：

```text
http://localhost:5173/register
```

注册全流程：

1. 用户打开注册页。
2. 输入账号。
3. 输入邮箱。
4. 点击“发送验证码”。
5. 前端调用后端发送邮箱验证码接口。
6. 用户查看邮箱，复制 6 位验证码。
7. 用户输入验证码、密码、确认密码。
8. 点击“注册并进入”。
9. 前端调用邮箱注册接口。
10. 后端校验邮箱是否被占用、账号是否被占用、验证码是否正确、两次密码是否一致。
11. 后端创建用户。
12. 后端返回 JWT token。
13. 前端保存 token 到 `localStorage.agent_token`。
14. 前端跳转到 `http://localhost:5173/chat`。

注册流程绑定的后端接口：

```http
POST /api/auth/register/email-verify-code
POST /api/auth/register/using-email
```

### 2.2 设置页

设置页用于配置当前登录用户绑定的模型参数。前端会通过后端接口保存配置，后端按 `user_id` 写入 `user_model_configs` 表。

可配置字段：

- `provider`：模型供应商，例如 `deepseek`、`openai`、`dashscope`、`doubao`、`stable-diffusion`、`mock`。
- `chat_model`：对话模型名称。
- `image_model`：图片模型名称。
- `base_url`：模型 API 地址。
- `api_key`：模型密钥。
- `temperature`：模型温度参数。

设置页绑定接口：

```http
GET /api/settings/model-config
PUT /api/settings/model-config
```

示例：配置 DeepSeek Anthropic 兼容接口。

```json
{
  "provider": "deepseek-anthropic",
  "chat_model": "deepseek-v4-pro",
  "image_model": "",
  "base_url": "https://api.deepseek.com/anthropic",
  "api_key": "sk-xxx",
  "temperature": "0.7",
  "anthropic_auth_token": "sk-xxx",
  "anthropic_base_url": "https://api.deepseek.com/anthropic",
  "anthropic_model": "deepseek-v4-pro",
  "anthropic_default_opus_model": "deepseek-v4-pro",
  "anthropic_default_sonnet_model": "deepseek-v4-pro",
  "anthropic_default_haiku_model": "deepseek-v4-pro",
  "claude_code_subagent_model": "deepseek-v4-pro",
  "claude_code_max_output_tokens": "32000"
}
```

当前后端已经在消息请求结构中接收 `model_config`，同时也提供了用户绑定的模型配置接口。后续接真实 Provider 时，可以优先读取数据库中的 `user_model_configs`，再结合消息请求内的 `model_config` 做覆盖。

### 2.3 对话页

对话页分为左侧对话区和右侧产物区。

左侧对话区：

- 顶部显示当前会话标题和 Agent Run 状态。
- 左侧显示会话列表，可切换历史会话。
- 消息区展示用户输入和 assistant 回复。
- assistant 回复补充问题后，会在该回复下方显示“补充问题回答框”。
- 底部“正常对话框”用于发起新需求。

右侧产物区：

- 展示 Agent Step 执行记录。
- 展示生成的图片、HTML 等产物。
- 图片使用 `img` 预览。
- HTML 使用 `sandbox iframe` 预览。
- 每个产物都提供下载按钮。

## 3. Agent 平台运行流程

### 3.1 普通输入流程

用户在底部“正常对话框”输入需求后，前端发送：

```json
{
  "input_type": "normal",
  "content": "帮我生成一张科技感宣传图",
  "model_config": {
    "provider": "mock",
    "chat_model": "mock-chat",
    "image_model": "mock-image",
    "base_url": "",
    "api_key": "",
    "temperature": "0.7"
  }
}
```

后端流程：

1. 校验会话归属当前用户。
2. 写入用户消息 `messages`。
3. 创建 `agent_runs`。
4. 创建 `planner_agent` 步骤。
5. assistant 返回补充问题。
6. 写入 `follow_up_questions`。
7. Agent Run 状态更新为 `waiting_questions`。

前端表现：

- 消息区显示用户消息。
- 消息区显示 assistant 回复。
- assistant 回复下方显示补充问题和“补充问题回答框”。

### 3.2 补充问题回答流程

用户在 assistant 回复下方的“补充问题回答框”填写答案后，前端发送：

```json
{
  "input_type": "answer_to_questions",
  "content": "两者都生成，尺寸 16:9，科技感，蓝绿色主色。",
  "answered_question_ids": [1, 2],
  "model_config": {
    "provider": "mock",
    "chat_model": "mock-chat",
    "image_model": "mock-image",
    "base_url": "",
    "api_key": "",
    "temperature": "0.7"
  }
}
```

后端流程：

1. 校验会话归属当前用户。
2. 写入用户回答消息。
3. 创建新的 `agent_runs`。
4. 将对应 `follow_up_questions` 标记为 `answered`。
5. 执行固定多 Agent DAG。

固定多 Agent DAG：

```text
context_agent
  -> prompt_agent
  -> image_agent
  -> html_agent
  -> review_agent
  -> artifact_agent
```

各步骤职责：

- `context_agent`：读取最近上下文和长期记忆。
- `prompt_agent`：组合用户输入、上下文记忆和会话信息。
- `image_agent`：生成图片产物。
- `html_agent`：生成 HTML 产物。
- `review_agent`：检查生成结果，当前为 mock 检查。
- `artifact_agent`：保存文件并写入产物元数据。

生成完成后：

1. 产物文件保存到本地对象存储目录。
2. 产物元数据写入 `artifacts`。
3. assistant 返回生成完成消息。
4. 会话摘要写入 `context_memories`。
5. Agent Run 状态更新为 `completed`。

## 4. 后端模块说明

### 4.1 路由文件

路由已按业务拆分：

- `routers/api.go`：API 总入口和静态资源注册。
- `routers/auth_routes.go`：认证与用户相关路由。
- `routers/agent_routes.go`：AI Agent 平台路由。
- `routers/example_routes.go`：脚手架示例路由。
- `routers/test_routes.go`：脚手架测试路由。

### 4.2 Controller

AI Agent 控制器：

```text
internal/controller/agent_ctrl/agent_controller.go
```

主要职责：

- 解析请求参数。
- 获取当前登录用户。
- 调用 `agent_svc.AgentService`。
- 统一返回 JSON 或 SSE。

### 4.3 Service

AI Agent 服务：

```text
internal/service/agent_svc/agent_service.go
```

主要职责：

- 会话和消息业务编排。
- 普通输入与补充问题回答分流。
- 固定多 Agent DAG 执行。
- Provider 调用。
- 产物保存。
- 上下文记忆写入。

### 4.4 DAO

AI Agent 数据访问：

```text
internal/dao/agent_dao/agent_dao.go
```

主要职责：

- 查询和创建会话。
- 查询和创建消息。
- 创建和更新补充问题。
- 创建 Agent Run 和 Agent Step。
- 查询上下文记忆。
- 创建和查询产物元数据。

### 4.5 Provider

模型 Provider 接口：

```text
internal/service/agent_svc/provider.go
```

当前实现：

- `Provider`：模型供应商接口。
- `MockProvider`：mock 模型实现。
- `Generate`：返回 SVG 图片和 HTML 页面两个产物。

后续真实模型接入建议：

- 保留 `Provider` 接口。
- 新增 `OpenAIProvider`、`DashScopeProvider`、`DoubaoProvider` 等实现。
- 根据 `model_config.provider` 选择具体 Provider。

### 4.6 ObjectStore

产物存储接口：

```text
internal/service/agent_svc/storage.go
```

当前实现：

- `ObjectStore`：对象存储接口。
- `LocalObjectStore`：本地文件存储。

默认存储配置：

```yaml
AIAgent:
  Storage:
    Driver: local
    LocalPath: public/artifacts
    PublicPath: /artifacts
```

后续接 S3 兼容对象存储时，建议新增 `S3ObjectStore`，保持接口不变。

## 5. 数据表说明

### 5.1 conversations

会话表。

| 字段 | 说明 |
| --- | --- |
| id | 会话 ID |
| user_id | 用户 ID |
| title | 会话标题 |
| status | 会话状态 |
| created_at | 创建时间 |
| updated_at | 更新时间 |

### 5.2 messages

消息表。

| 字段 | 说明 |
| --- | --- |
| id | 消息 ID |
| conversation_id | 会话 ID |
| user_id | 用户 ID |
| role | 消息角色：user、assistant、system |
| input_type | 输入类型：normal、answer_to_questions、agent_result 等 |
| content | 消息内容 |
| agent_run_id | 关联 Agent Run |
| created_at | 创建时间 |
| updated_at | 更新时间 |

### 5.3 follow_up_questions

补充问题表。

| 字段 | 说明 |
| --- | --- |
| id | 问题 ID |
| conversation_id | 会话 ID |
| message_id | 产生问题的 assistant 消息 ID |
| user_id | 用户 ID |
| question | 问题内容 |
| answer | 用户回答 |
| status | pending、answered |
| created_at | 创建时间 |
| updated_at | 更新时间 |

### 5.4 agent_runs

Agent 总任务表。

| 字段 | 说明 |
| --- | --- |
| id | Agent Run ID |
| conversation_id | 会话 ID |
| user_id | 用户 ID |
| trigger_message_id | 触发任务的消息 ID |
| status | running、waiting_questions、completed、failed |
| intent | 任务意图：image、html、mixed |
| error_message | 失败原因 |
| created_at | 创建时间 |
| updated_at | 更新时间 |

### 5.5 agent_steps

Agent 子步骤表。

| 字段 | 说明 |
| --- | --- |
| id | 步骤 ID |
| agent_run_id | Agent Run ID |
| name | 步骤名称 |
| status | 步骤状态 |
| input | 步骤输入 |
| output | 步骤输出 |
| error_message | 失败原因 |
| created_at | 创建时间 |
| updated_at | 更新时间 |

### 5.6 context_memories

上下文记忆表。

| 字段 | 说明 |
| --- | --- |
| id | 记忆 ID |
| conversation_id | 会话 ID |
| user_id | 用户 ID |
| kind | 记忆类型 |
| content | 记忆内容 |
| score | 检索排序分数 |
| created_at | 创建时间 |
| updated_at | 更新时间 |

### 5.7 artifacts

产物元数据表。

| 字段 | 说明 |
| --- | --- |
| id | 产物 ID |
| conversation_id | 会话 ID |
| user_id | 用户 ID |
| agent_run_id | Agent Run ID |
| name | 文件名 |
| kind | image、html 等 |
| mime_type | MIME 类型 |
| object_key | 对象存储 key |
| preview_url | 预览地址 |
| size_bytes | 文件大小 |
| hash | 文件 hash |
| created_at | 创建时间 |
| updated_at | 更新时间 |

## 6. 接口文档

### 6.1 登录

```http
POST /api/auth/login
Content-Type: application/json
```

请求体：

```json
{
  "account": "demo",
  "password": "123456"
}
```

或：

```json
{
  "email": "demo@example.com",
  "password": "123456"
}
```

成功响应：

```json
{
  "code": 0,
  "msg": "请求成功",
  "data": {
    "token": "JWT_TOKEN",
    "user": {
      "id": 1,
      "account": "demo"
    }
  }
}
```

### 6.1.1 发送注册邮箱验证码

```http
POST /api/auth/register/email-verify-code
Content-Type: application/json
```

请求体：

```json
{
  "email": "demo@example.com"
}
```

成功响应：

```json
{
  "code": 0,
  "msg": "请求成功",
  "data": {
    "email": "demo@example.com"
  }
}
```

失败场景：

- 邮箱为空。
- 邮箱格式错误。
- 邮箱已被占用。
- SMTP 配置错误或邮件发送失败。

### 6.1.2 邮箱注册

```http
POST /api/auth/register/using-email
Content-Type: application/json
```

请求体：

```json
{
  "account": "demo",
  "email": "demo@example.com",
  "password": "123456",
  "password_confirm": "123456",
  "verify_code": "123456"
}
```

成功响应：

```json
{
  "code": 0,
  "msg": "请求成功",
  "data": {
    "token": "JWT_TOKEN"
  }
}
```

失败场景：

- 账号为空或格式错误。
- 账号已被占用。
- 邮箱为空或格式错误。
- 邮箱已被占用。
- 密码长度不足。
- 两次密码不一致。
- 验证码为空、不是 6 位数字或验证码错误。

### 6.2 获取会话列表

### 6.1.3 获取当前用户模型配置

```http
GET /api/settings/model-config
token: JWT_TOKEN
```

成功响应：

```json
{
  "code": 0,
  "msg": "请求成功",
  "data": {
    "model_config": {
      "user_id": 1,
      "provider": "deepseek-anthropic",
      "chat_model": "deepseek-v4-pro",
      "image_model": "",
      "base_url": "https://api.deepseek.com/anthropic",
      "api_key": "sk-xxx",
      "temperature": "0.7",
      "anthropic_auth_token": "sk-xxx",
      "anthropic_base_url": "https://api.deepseek.com/anthropic",
      "anthropic_model": "deepseek-v4-pro",
      "anthropic_default_opus_model": "deepseek-v4-pro",
      "anthropic_default_sonnet_model": "deepseek-v4-pro",
      "anthropic_default_haiku_model": "deepseek-v4-pro",
      "claude_code_subagent_model": "deepseek-v4-pro",
      "claude_code_max_output_tokens": "32000"
    }
  }
}
```

说明：

- 如果当前用户还没有保存过配置，后端会返回 DeepSeek 默认配置。
- 配置与当前登录用户绑定，其他用户无法读取。

### 6.1.4 保存当前用户模型配置

```http
PUT /api/settings/model-config
token: JWT_TOKEN
Content-Type: application/json
```

请求体：

```json
{
  "provider": "deepseek-anthropic",
  "chat_model": "deepseek-v4-pro",
  "image_model": "",
  "base_url": "https://api.deepseek.com/anthropic",
  "api_key": "sk-xxx",
  "temperature": "0.7",
  "anthropic_auth_token": "sk-xxx",
  "anthropic_base_url": "https://api.deepseek.com/anthropic",
  "anthropic_model": "deepseek-v4-pro",
  "anthropic_default_opus_model": "deepseek-v4-pro",
  "anthropic_default_sonnet_model": "deepseek-v4-pro",
  "anthropic_default_haiku_model": "deepseek-v4-pro",
  "claude_code_subagent_model": "deepseek-v4-pro",
  "claude_code_max_output_tokens": "32000"
}
```

成功响应：

```json
{
  "code": 0,
  "msg": "请求成功",
  "data": {
    "model_config": {
      "user_id": 1,
      "provider": "deepseek-anthropic",
      "chat_model": "deepseek-v4-pro",
      "image_model": "",
      "base_url": "https://api.deepseek.com/anthropic",
      "api_key": "sk-xxx",
      "temperature": "0.7",
      "anthropic_auth_token": "sk-xxx",
      "anthropic_base_url": "https://api.deepseek.com/anthropic",
      "anthropic_model": "deepseek-v4-pro",
      "anthropic_default_opus_model": "deepseek-v4-pro",
      "anthropic_default_sonnet_model": "deepseek-v4-pro",
      "anthropic_default_haiku_model": "deepseek-v4-pro",
      "claude_code_subagent_model": "deepseek-v4-pro",
      "claude_code_max_output_tokens": "32000"
    }
  }
}
```

说明：

- 首次保存会创建配置记录。
- 再次保存会更新当前用户已有配置。
- API Key 当前以明文字段保存，生产环境建议加密或接入密钥管理服务。

```http
GET /api/conversations
token: JWT_TOKEN
```

成功响应：

```json
{
  "code": 0,
  "msg": "请求成功",
  "data": {
    "conversations": []
  }
}
```

### 6.3 创建会话

```http
POST /api/conversations
token: JWT_TOKEN
Content-Type: application/json
```

请求体：

```json
{
  "title": "图片生成工作台"
}
```

成功响应：

```json
{
  "code": 0,
  "msg": "请求成功",
  "data": {
    "conversation": {
      "id": 1,
      "title": "图片生成工作台",
      "status": "active"
    }
  }
}
```

### 6.4 获取会话消息

```http
GET /api/conversations/{conversation_id}/messages
token: JWT_TOKEN
```

成功响应：

```json
{
  "code": 0,
  "msg": "请求成功",
  "data": {
    "messages": []
  }
}
```

### 6.5 发送普通对话

```http
POST /api/conversations/{conversation_id}/messages
token: JWT_TOKEN
Content-Type: application/json
```

请求体：

```json
{
  "input_type": "normal",
  "content": "帮我生成一张科技感宣传图",
  "attachments": [],
  "model_config": {
    "provider": "mock",
    "chat_model": "mock-chat",
    "image_model": "mock-image",
    "base_url": "",
    "api_key": "",
    "temperature": "0.7"
  }
}
```

成功响应：

```json
{
  "code": 0,
  "msg": "请求成功",
  "data": {
    "user_message": {},
    "assistant_message": {},
    "follow_up_questions": [
      {
        "id": 1,
        "question": "希望生成图片、HTML 页面，还是两者都生成？",
        "status": "pending"
      }
    ],
    "agent_run": {
      "id": 1,
      "status": "waiting_questions"
    }
  }
}
```

### 6.6 提交补充问题回答

```http
POST /api/conversations/{conversation_id}/messages
token: JWT_TOKEN
Content-Type: application/json
```

请求体：

```json
{
  "input_type": "answer_to_questions",
  "content": "两者都生成，尺寸 16:9，科技感。",
  "answered_question_ids": [1, 2],
  "model_config": {
    "provider": "mock",
    "chat_model": "mock-chat",
    "image_model": "mock-image",
    "base_url": "",
    "api_key": "",
    "temperature": "0.7"
  }
}
```

成功响应：

```json
{
  "code": 0,
  "msg": "请求成功",
  "data": {
    "user_message": {},
    "assistant_message": {},
    "artifacts": [
      {
        "id": 1,
        "name": "generated-image.svg",
        "kind": "image",
        "mime_type": "image/svg+xml",
        "preview_url": "/artifacts/user-1/conversation-1/run-2/generated-image.svg"
      },
      {
        "id": 2,
        "name": "generated-page.html",
        "kind": "html",
        "mime_type": "text/html; charset=utf-8",
        "preview_url": "/artifacts/user-1/conversation-1/run-2/generated-page.html"
      }
    ],
    "agent_run": {
      "id": 2,
      "status": "completed"
    }
  }
}
```

### 6.7 获取会话产物

```http
GET /api/conversations/{conversation_id}/artifacts
token: JWT_TOKEN
```

成功响应：

```json
{
  "code": 0,
  "msg": "请求成功",
  "data": {
    "artifacts": []
  }
}
```

### 6.8 下载产物

```http
GET /api/artifacts/{artifact_id}/download
token: JWT_TOKEN
```

响应：

- 成功时直接返回文件下载。
- 如果产物不存在或不属于当前用户，返回错误 JSON。

### 6.9 获取 Agent Run 事件

```http
GET /api/runs/{run_id}/events?token=JWT_TOKEN
Accept: text/event-stream
```

返回格式：

```text
event: agent_step
data: {"id":1,"name":"context_agent","status":"completed"}

event: agent_step
data: {"id":2,"name":"prompt_agent","status":"completed"}

event: done
data: {}
```

说明：

- 当前版本是一次性返回已落库的 Agent Step。
- 后续如果改成异步队列执行，可保持该 SSE 接口不变，逐步推送运行中事件。

## 7. 鉴权规则

除登录接口外，AI Agent 平台接口均需要 JWT。

JWT 传递方式：

```http
token: JWT_TOKEN
```

SSE 接口也支持 query token：

```http
/api/runs/{run_id}/events?token=JWT_TOKEN
```

后端会使用当前登录用户 ID 过滤：

- 会话。
- 消息。
- 补充问题。
- Agent Run。
- Agent Step。
- 产物。

## 8. 本地启动

### 8.1 后端

```bash
cd C:\Users\易理志\Desktop\Yl_Agent_picture\gin_agent_gorm
copy etc\config.yaml.example etc\config.yaml
go run . server
```

后端默认地址：

```text
http://localhost:8501
```

### 8.2 前端

```bash
cd C:\Users\易理志\Desktop\Yl_Agent_picture\frontend
npm install
npm run dev
```

前端默认地址：

```text
http://localhost:5173
```

Vite 已代理：

- `/api` -> `http://localhost:8501`
- `/artifacts` -> `http://localhost:8501`

## 9. 当前限制与后续扩展

当前限制：

- Provider 使用 mock 实现，暂未调用真实模型。
- ObjectStore 使用本地文件，暂未接 S3 兼容对象存储。
- Agent DAG 当前同步执行，尚未接 Asynq 异步队列。
- `model_config` 已透传到后端请求结构，但当前 mock Provider 暂未消费。

建议后续扩展：

1. 根据 `model_config.provider` 实现 Provider 工厂。
2. 新增真实模型 Provider，例如 OpenAI、DashScope、豆包、Stable Diffusion。
3. 将 `executeGeneration` 改造成 Asynq 异步任务。
4. SSE 接口改为实时推送 Agent Step 状态。
5. 新增 S3 兼容对象存储实现。
6. 增加接口测试和多用户权限隔离测试。
