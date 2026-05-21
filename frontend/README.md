# 图片 AI Agent 前端

Vue 3 + Vite 工作台，包含登录页、对话页、设置页、Agent Step 事件展示、图片/HTML 产物预览和下载。

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
- 对话页：`http://localhost:5173/chat`
- 设置页：`http://localhost:5173/settings`

## 使用流程

1. 在注册页输入账号、邮箱，发送邮箱验证码。
2. 填写验证码、密码和确认密码，注册成功后进入对话页。
3. 在设置页配置 Provider、对话模型、图片模型、Base URL、API Key 等信息，配置会保存到后端并绑定当前登录用户。
4. 进入对话页，新建会话。
5. 在对话页左侧选择或新建会话，在正常对话框输入图片或 HTML 需求。
6. 系统返回补充问题后，在 assistant 回复下方的补充问题回答框提交答案。
7. 右侧查看生成的 SVG 图片、HTML 页面，并可下载文件。
