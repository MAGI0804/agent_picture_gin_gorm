# 图片 AI Agent 前端

Vue 3 + Vite 工作台，默认入口为 Agent V2 Workspace，包含登录页、V2 工作台、旧版兼容对话页、设置页、Agent Step 事件展示、图片/HTML/SVG 产物预览和下载。

## 启动

```bash
npm install
npm run dev
```

默认访问 `http://localhost:5173`。Vite 会把 `/api` 和 `/artifacts` 代理到 `http://localhost:8501`。

## 页面与端口

- 前端：`http://localhost:5173`
- 后端：`http://localhost:8501`
- 登录页：`http://localhost:5173/login`
- 注册页：`http://localhost:5173/register`
- V2 工作台：`http://localhost:5173/workspace`
- 旧版兼容对话页：`http://localhost:5173/chat`
- 设置页：`http://localhost:5173/settings`

## 使用流程

1. 在注册页输入账号、邮箱，发送邮箱验证码。
2. 填写验证码、密码和确认密码，注册成功后进入 V2 工作台。
3. 在设置页配置 Provider、对话模型、图片模型、Base URL、API Key 等信息，配置会保存到后端并绑定当前登录用户。
4. 进入 V2 工作台，新建或选择会话。
5. 在 V2 工作台输入图片需求，查看 Timeline、候选产物、版本链、Review/Eval、Memory 和编辑面板。
6. 系统返回补充问题后，在 V2 工作台的补充信息区域提交答案，同一个 run 会继续推进。
7. 产物预览和下载默认走鉴权 API；旧版 `/chat` 仅作为历史会话兼容入口保留。
