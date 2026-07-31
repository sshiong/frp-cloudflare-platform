# FRP 多用户云隧道管理平台

一个轻量、安全、可恢复的多用户 FRP 管理平台，支持 TCP/UDP 端口映射和 HTTP/HTTPS 域名反向代理。

## 🌟 特性

- **双面板架构**: Server Panel (公网服务器) + Client Panel (本地机器)
- **多用户管理**: 管理员 + 普通用户，完善的权限控制
- **TCP/UDP 映射**: 支持 TCP 和 UDP 端口映射，跨协议端口互斥
- **HTTP/HTTPS 域名**: 支持多域名绑定，自动 HTTPS 证书
- **Cloudflare 集成**: DNS 管理、小橙云代理、Token 生命周期管理
- **ACME 自动化**: Let's Encrypt / ZeroSSL 自动证书签发和续期
- **安全设计**: HMAC-SHA256 设备认证、Ed25519 配置签名、AES-256-GCM 加密
- **单活动会话**: 每个设备同一时间只允许一个浏览器会话
- **配置版本控制**: 版本化配置、原子替换、失败回滚
- **审计日志**: 完整的操作审计记录

## 📁 项目结构

```
frp-cloudflare-platform/
├── server-panel/          # 服务端面板 (Go + Vue 3)
│   ├── cmd/
│   │   ├── control/       # 控制进程入口
│   │   └── router/        # 路由进程入口
│   ├── internal/          # 内部实现
│   ├── migrations/        # 数据库迁移
│   ├── web-admin/         # 管理端前端 (Vue 3)
│   └── tests/
├── client-panel/          # 客户端面板 (Go + Vue 3)
│   ├── cmd/client/        # 客户端入口
│   ├── internal/          # 内部实现
│   ├── web-client/        # 客户端前端 (Vue 3)
│   └── tests/
├── shared-protocol/       # 共享协议包
│   ├── errors/            # 错误码定义
│   ├── schemas/           # 版本和状态定义
│   └── canonical/         # HMAC 和配置规范化
├── docker-compose.yml     # Docker 编排
└── Makefile               # 构建脚本
```

## 🚀 快速开始

### 环境要求

- Go 1.21+
- Node.js 20+
- npm 或 pnpm

### 开发模式

```bash
# 启动 Server Panel 开发模式
make dev-server

# 启动 Client Panel 开发模式
make dev-client
```

### 构建

```bash
# 构建所有组件
make all

# 仅构建 Server Panel
make build-server

# 仅构建 Client Panel
make build-client

# 交叉编译所有平台
make build-all-platforms
```

### Docker 部署

```bash
# 启动所有服务
docker-compose up -d

# 查看日志
docker-compose logs -f

# 停止服务
docker-compose down
```

## 🔧 技术栈

### 后端
- **Go** - 高性能并发语言
- **chi** - 轻量级 HTTP 路由
- **SQLite** - 嵌入式数据库 (WAL 模式)
- **sqlc** - 类型安全的 SQL

### 前端
- **Vue 3** - 渐进式 JavaScript 框架
- **TypeScript** - 类型安全的 JavaScript
- **Vite** - 下一代前端构建工具
- **Element Plus** - Vue 3 UI 组件库
- **Pinia** - Vue 状态管理

### 安全
- **HMAC-SHA256** - 设备 API 认证
- **Ed25519** - 配置签名
- **AES-256-GCM** - 数据加密
- **Argon2id** - 密码哈希

## 📖 文档

详细设计文档请参考: [设计文档](frp_cloudflare_platform_v2_3_single_session_final_design.md)

## 🔐 安全设计

- 所有敏感数据加密存储
- 单活动浏览器会话防止会话劫持
- HMAC 防重放攻击
- 配置签名防止篡改
- 密钥用途隔离
- 完整的审计日志

## 📄 许可证

MIT License

## 🤝 贡献

欢迎提交 Issue 和 Pull Request！
