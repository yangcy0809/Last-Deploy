last-deploy — 开源项目 Docker 管理 WebUI 工具
  ┌──────┬──────────────────────────────────┬─────────────────────────────────────┐
  │ 模块 │              技术栈              │              关键文件               │
  ├──────┼──────────────────────────────────┼─────────────────────────────────────┤
  │ 后端 │ Go + Gin + SQLite                │ backend/cmd/last-deploy/main.go     │
  ├──────┼──────────────────────────────────┼─────────────────────────────────────┤
  │ 前端 │ React + Vite + TypeScript + antd │ frontend/src/pages/ProjectsPage.tsx │
  ├──────┼──────────────────────────────────┼─────────────────────────────────────┤
  │ 部署 │ Docker Compose                   │ deploy/docker-compose.yml           │
  └──────┴──────────────────────────────────┴─────────────────────────────────────┘
## 核心功能

1. 项目管理 — 从 GitHub URL 拉取并部署（支持 docker-compose 和 Dockerfile）
2. 状态监控 — 实时显示容器状态 (running/stopped/unknown)
3. 操作控制 — 启动/停止/删除项目
4. 端口映射 — 用户手动指定端口
5. 持久化 — SQLite 存储项目配置

关键决策

- Compose 执行：CLI 方式 (docker compose)，简单可靠
- 异步任务：内存 Job 队列 + 单 Worker
- 资源标记：com.last-deploy.project_id label
- 安全：默认监听 127.0.0.1，路径校验防穿越

运行方式

## 后端
```bash
cd backend && go run ./cmd/last-deploy
```

## 前端 (开发)
```bash
cd frontend && npm install && npm run dev
```
## Docker 部署
```bash
cd deploy && docker compose up -d
```
## 验证
```bash
curl http://127.0.0.1:8080/api/health
{"ok":true}
```