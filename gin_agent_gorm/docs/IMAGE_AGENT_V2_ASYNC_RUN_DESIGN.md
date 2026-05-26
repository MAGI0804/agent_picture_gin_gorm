# Image Agent V2 异步 Run 设计

生成日期：2026-05-26

## 目标

真实图片模型可能阻塞几十秒到数分钟，`POST /api/v2/conversations/:id/runs` 不应长期占用 HTTP 请求。V2 后续应把 Run 创建、排队、执行、查询拆开，保持当前同步接口可兼容，但新增异步路径作为默认生产路径。

## 状态模型

`agent_runs.status` 继续使用现有枚举：

| 状态 | 含义 |
| --- | --- |
| `created` | Run 已落库，尚未入队 |
| `queued` | 已写入任务队列，等待 worker |
| `running` | worker 正在执行 workflow |
| `waiting_user` | 需要用户补充输入 |
| `completed` | workflow 完成，artifact/version/review 已落库 |
| `failed` | 执行失败，`error_message` 有摘要 |
| `cancelled` | 用户或系统取消 |

## API 调整

保留当前同步接口用于本地调试：

```http
POST /api/v2/conversations/:id/runs
```

新增异步接口：

```http
POST /api/v2/conversations/:id/runs/async
GET  /api/v2/runs/:id
GET  /api/v2/runs/:id/events
POST /api/v2/runs/:id/cancel
```

异步创建接口只做权限校验、幂等检查、message/run 落库、预算初始化和入队，然后返回：

```json
{
  "agent_run": {},
  "queued": true
}
```

## 队列选择

优先复用项目现有 `pkg/job` / Redis 队列能力；如果现有队列无法满足延迟重试、唯一任务和 worker 可观测性，再引入 Asynq。

首版任务 payload：

```json
{
  "run_id": 1,
  "user_id": 1,
  "conversation_id": 10,
  "idempotency_key": "optional"
}
```

worker 只从 DB 读取 run/message/model config，避免把 prompt、token、provider 配置放入队列 payload。

## 执行流程

1. API 校验 conversation owner。
2. API 写入 user message 和 `agent_runs(status=created)`。
3. API 按 `idempotency_key` 防重。
4. API 将 run 标记为 `queued` 并投递任务。
5. worker 抢占 run，状态从 `queued` 改为 `running`。
6. worker 执行 `workflow.ImageGenerationWorkflow`。
7. Runtime 持续写 `agent_steps`，前端通过 `GET /runs/:id/events` 轮询或 SSE 读取。
8. Artifact Agent 写 artifact/version，Review 写 `quality_scores`。
9. worker 标记 `completed` 或 `failed`。

## 幂等和并发

- `idempotency_key` 后续需要 DB 唯一约束：`(user_id, idempotency_key)`。
- worker 抢占时使用条件更新：只允许 `queued -> running`，更新失败说明已有 worker 接管。
- 后续接 Redis lock 时，锁 key 使用 `agent_v2:run:{run_id}`，锁 TTL 不小于单次任务最大超时。

## 失败策略

- provider 临时网络错误：首版最多重试 2 次，指数退避。
- 业务校验错误：不重试，直接 `failed`。
- worker 崩溃：依赖队列重投递或定时扫描 `running` 超时 run。

## 前端体验

- 创建异步 run 后立即展示 queued 状态。
- Timeline 每 2 秒轮询 `GET /api/v2/runs/:id/events`，后续可替换成真正增量 SSE。
- Artifact Board 在 run `completed` 后刷新。
- 失败时保留已写 step，展示 `error_message`。

## 首版验收

- `POST /runs/async` 在 1 秒内返回 queued run。
- worker 能完成现有 `image_generation_v2` workflow。
- 页面刷新后仍能从 run id 恢复 timeline 和 artifact board。
- 同一个 `idempotency_key` 不创建重复 run。
- `go test ./...` 和 `npm run build` 通过。
