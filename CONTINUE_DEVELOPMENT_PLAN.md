# 继续开发计划：图片 AI Agent 工作台

## Summary

- 模型配置收敛为全局模型目录，用户只选择文本模型和图片模型。
- 对话链路统一输出结构，前端不再随消息提交 API Key、Base URL 等敏感模型配置。
- 前端补齐 token 失效跳登录、三栏工作台、右侧 tabs、普通输入框和追问回答输入框，并在普通输入区直接选择文本/图片模式及对应模型。
- 后端继续保留会话、消息、追问问题、上下文记忆、Agent Run、Artifact 等现有模型，在兼容基础上升级。

## 已实现方向

- `model_configs` 作为全局模型目录，通过 `is_text_model` 和 `is_image_model` 区分文本类和图片类模型。
- `user_model_configs` 增加 `selected_text_model_config_id` 与 `selected_image_model_config_id`，用于保存用户选择。
- 新增 `GET /api/settings/model-selection` 与 `PUT /api/settings/model-selection`，前端设置页只保存模型 ID。
- `apiFetch` 统一处理 `100401` 与 token 失效文案，自动清理 token 并跳转登录页。
- 对话页保留两个输入框：底部普通输入框，以及 assistant 追问下方的回答输入框。
- 文本模型输出封装并展示思考过程：assistant 消息下方可展开查看思考内容，右侧消息索引也会显示摘要。
- 普通输入区只保留文本模式和图片模式；模型下拉框跟随模式切换，并直接保存用户选择。
- 图片模式先调用文本模型生成针对性追问和最终图片提示词，再调用图片模型生成真实图片产物；仅当图片模型配置为 mock 或缺少必要参数时才回退 mock。
- 上下文流程按短期消息、任务追问、长期记忆组装，并写入 summary、artifact_requirement、preference 记忆。

## 后续优化

- 为全局模型管理补管理员权限控制，避免普通用户直接增删改全局模型。
- 扩展真实图片模型 Provider：DashScope、豆包、Stable Diffusion 等供应商需要独立适配。
- 将 Agent Run 事件从轮询升级为真实 SSE 流式推送。
- 为模型选择、上下文组装、追问生成、Artifact 保存补充单元测试和集成测试。
- 为右侧产物预览补更多文件类型支持，例如 Markdown、JSON、ZIP。

## Test Plan

- 后端运行 `go test ./...`。
- 前端运行 `npm run build`。
- 手动验证登录、token 过期跳登录、模型选择保存、普通输入、追问回答、Artifact 预览与下载。
