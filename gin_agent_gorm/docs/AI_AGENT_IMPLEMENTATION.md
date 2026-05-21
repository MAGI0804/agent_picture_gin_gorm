# AI Agent 后端实现说明

## 已实现能力

- 新增会话、消息、补充问题、Agent Run、Agent Step、上下文记忆、产物文件模型。
- 启动时根据 `AIAgent.AutoMigrate` 自动迁移 AI Agent 相关表。
- 新增登录接口 `POST /api/auth/login`，返回现有 JWT token。
- 新增受 JWT 保护的 AI Agent API：
  - `GET /api/conversations`
  - `POST /api/conversations`
  - `GET /api/conversations/:id/messages`
  - `POST /api/conversations/:id/messages`
  - `GET /api/conversations/:id/artifacts`
  - `GET /api/artifacts/:id/download`
  - `GET /api/runs/:id/events`
- 首版使用 mock Provider，回答补充问题后会生成 SVG 图片和 HTML 文件。
- 首版使用本地对象存储适配器，默认写入 `public/artifacts`，并通过 `/artifacts/*` 预览。

## 对话流程

1. 前端向 `POST /api/conversations/:id/messages` 发送 `input_type=normal`。
2. 后端创建用户消息、Agent Run 和 Planner Step。
3. 后端返回 assistant 消息和 `follow_up_questions`。
4. 用户在补充问题回答框提交 `input_type=answer_to_questions`。
5. 后端依次执行 Context、Prompt、Image、HTML、Review、Artifact steps。
6. 产物元数据写入 `artifacts`，文件写入对象存储，前端右侧预览。

## 配置

在 `etc/config.yaml` 中加入：

```yaml
AIAgent:
  AutoMigrate: true
  Provider:
    Name: mock
  Storage:
    Driver: local
    LocalPath: public/artifacts
    PublicPath: /artifacts
```

后续接入 S3 兼容对象存储时，保留 `artifacts.object_key`、`preview_url`、`hash` 等元数据字段，只替换 `internal/service/agent_svc/storage.go` 中的 ObjectStore 实现即可。

## 本地运行

```bash
cd C:\Users\易理志\Desktop\Yl_Agent_picture\gin_agent_gorm
copy etc\config.yaml.example etc\config.yaml
go run . server
```

```bash
cd C:\Users\易理志\Desktop\Yl_Agent_picture\frontend
npm install
npm run dev
```

前端开发服务默认运行在 `http://localhost:5173`，后端默认代理到 `http://localhost:8501`。
