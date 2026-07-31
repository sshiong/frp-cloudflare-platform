# 变更日志

本项目遵循 [语义化版本](https://semver.org/lang/zh-CN/) 规范。

## [未发布]

### 新增
- 初始项目结构
- Server Panel 核心后端 (Go + chi)
- Client Panel 核心后端 (Go + chi)
- Server Panel 前端 (Vue 3 + Element Plus)
- Client Panel 前端 (Vue 3 + Element Plus)
- SQLite 数据库 (WAL 模式)
- 用户管理系统 (管理员 + 普通用户)
- 设备管理 (注册、绑定、解绑)
- 映射管理 (TCP/UDP/HTTP)
- 域名管理 (IDNA/Punycode 标准化)
- Cloudflare 集成 (DNS 管理、Token 生命周期)
- ACME 证书自动化 (Let's Encrypt / ZeroSSL)
- Router 进程 (HTTP/HTTPS 反向代理)
- FRPS 插件 (二次鉴权)
- 后台任务系统 (租约、去重、重试)
- 备份与恢复 (加密归档)
- 审计日志
- WebSocket 实时通信
- HMAC-SHA256 设备认证
- Ed25519 配置签名
- AES-256-GCM 数据加密
- 单活动浏览器会话
- 配置版本控制和原子替换

### 安全
- 实施最小权限原则
- 密钥用途隔离
- 敏感数据加密存储
- 日志敏感信息过滤
- CSRF 保护
- 速率限制

## [2.3.0] - 2026-07-31

### 新增
- 完整的双面板架构实现
- 所有核心功能模块
- 专业 UI 设计 (Vercel/Linear/Stripe 风格)
- Docker 支持
- systemd 服务文件
- CI/CD 配置

### 文档
- 详细的设计文档
- API 文档
- 部署指南
- 贡献指南
- 安全政策

---

## 版本说明

### 版本号格式

- **主版本号 (MAJOR)**: 不兼容的 API 变更
- **次版本号 (MINOR)**: 向后兼容的功能性新增
- **修订号 (PATCH)**: 向后兼容的问题修正

### 变更类型

- **新增**: 新功能
- **变更**: 对现有功能的变更
- **弃用**: 即将移除的功能
- **移除**: 已移除的功能
- **修复**: Bug 修复
- **安全**: 安全相关的变更
