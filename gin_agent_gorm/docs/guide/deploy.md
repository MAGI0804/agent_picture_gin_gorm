# 使用 docker 部署

## 图片 Agent V2 入口

当前默认前端入口为 `/workspace`，旧 `/chat` 只作为历史会话和旧 artifact 兼容入口保留。生产部署时建议：

- 前端访问 `http://<host>:5173/workspace` 或由网关将根路径转到 V2 工作台。
- 后端继续暴露 `8501`，V2 API 位于 `/api/v2`。
- 产物预览/下载默认走鉴权 API；`AIAgent.Storage.StaticEnabled` 保持 `false`，除非明确需要临时静态调试。
- 队列 worker 需要随 API 服务一起部署，用于执行 `agent_v2:run` 异步任务。

```shell

# 在项目根目录下执行以下命令生成 docker 镜像
docker build -t go-service:v1.0.0 .

# 创建一个容器
# 我把根目录下的 Dockerfile 文件中的 ENTRYPOINT 注释掉了，
# 如果希望启动容器的同时并且启动服务，那么则需要加上 `--entrypoint` 参数
# 如果取消掉根目录下的 Dockerfile 文件中的 ENTRYPOINT 注释，那么则可以不用加 `--entrypoint` 参数，容器启动时，服务也会启动
# 这里将容器中的 8501 端口映射到宿机 9501 端口上了
docker run -itd --name gin-biz-web-api \
--restart=always \
--privileged -u root \
-v <your-path>/gin-biz-web-api:/go-project \
--entrypoint /go-project/entryPoint.sh \
-p 9501:8501 go-service:v1.0.0

# 如果初始化容器时保留了 --entrypoint 参数，那么重启容器时，服务也会跟着重启
# 启动日志在 `storage/logs/str-*` 文件中
docker restart gin-biz-web-api

```
