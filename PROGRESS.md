# FRP 多用户云隧道管理平台 - 进度跟踪

> 最后更新: 2026-07-31

## 总体进度

| 阶段 | 描述 | 状态 | 完成度 |
|------|------|------|--------|
| 0 | 项目结构与基础搭建 | 🔄 进行中 | 0% |
| 1 | Server Panel 核心后端 | ⏳ 待开始 | 0% |
| 2 | Client Panel 核心后端 | ⏳ 待开始 | 0% |
| 3 | Server Panel 前端 | ⏳ 待开始 | 0% |
| 4 | Client Panel 前端 | ⏳ 待开始 | 0% |
| 5 | 数据库迁移 | ⏳ 待开始 | 0% |
| 6 | 认证与安全 | ⏳ 待开始 | 0% |
| 7 | 设备管理 | ⏳ 待开始 | 0% |
| 8 | 映射管理 | ⏳ 待开始 | 0% |
| 9 | 域名与DNS管理 | ⏳ 待开始 | 0% |
| 10 | Cloudflare集成 | ⏳ 待开始 | 0% |
| 11 | 证书管理(ACME) | ⏳ 待开始 | 0% |
| 12 | Router进程 | ⏳ 待开始 | 0% |
| 13 | FRPS插件 | ⏳ 待开始 | 0% |
| 14 | 后台任务系统 | ⏳ 待开始 | 0% |
| 15 | 备份与恢复 | ⏳ 待开始 | 0% |
| 16 | 审计日志 | ⏳ 待开始 | 0% |
| 17 | WebSocket通信 | ⏳ 待开始 | 0% |
| 18 | 集成测试 | ⏳ 待开始 | 0% |

## 详细任务

### 阶段 0: 项目结构与基础搭建
- [x] 创建项目目录结构
- [ ] 初始化Go模块(server-panel, client-panel)
- [ ] 初始化Vue3前端项目(server-web-admin, client-web)
- [ ] 创建GitHub仓库
- [ ] 配置构建脚本
- [ ] 配置CI/CD

### 阶段 1: Server Panel 核心后端
- [ ] Go HTTP Router (chi)
- [ ] SQLite数据库连接(WAL模式)
- [ ] 中间件(CORS, CSRF, 日志, 错误处理)
- [ ] API版本化路由
- [ ] 结构化JSON日志
- [ ] 嵌入前端静态资源

### 阶段 2: Client Panel 核心后端
- [ ] 本地HTTP API服务器
- [ ] SQLite本地数据库
- [ ] 文件锁/互斥量
- [ ] 前端静态资源嵌入

### 阶段 3: Server Panel 前端
- [ ] Vue 3 + TypeScript + Vite 脚手架
- [ ] Element Plus集成
- [ ] Pinia状态管理
- [ ] 路由配置
- [ ] 布局组件(专业配色方案)
- [ ] 登录页面
- [ ] 管理员仪表盘
- [ ] 用户管理页面
- [ ] 设备管理页面
- [ ] 映射管理页面
- [ ] 域名管理页面
- [ ] Cloudflare管理页面
- [ ] 证书管理页面
- [ ] 系统设置页面
- [ ] 审计日志页面
- [ ] 备份恢复页面

### 阶段 4: Client Panel 前端
- [ ] Vue 3 + TypeScript + Vite 脚手架
- [ ] Element Plus集成
- [ ] 登录页面(含服务端地址配置)
- [ ] 仪表盘(FRPC状态)
- [ ] 映射列表
- [ ] FRPC日志查看
- [ ] 设置页面

### 阶段 5: 数据库迁移
- [ ] system_identity表
- [ ] users表
- [ ] sessions表
- [ ] clients表
- [ ] device_credentials表
- [ ] device_request_nonces表
- [ ] idempotency_records表
- [ ] frp_credentials表
- [ ] mappings表
- [ ] mapping_revisions表
- [ ] port_leases表
- [ ] domain_bindings表
- [ ] dns_records表
- [ ] cloudflare_credentials表
- [ ] certificates表
- [ ] config_snapshots表
- [ ] config_signing_keys表
- [ ] router_snapshots表
- [ ] router_state表
- [ ] operations表
- [ ] jobs表
- [ ] audit_logs表

### 阶段 6: 认证与安全
- [ ] 首次管理员创建(随机12位密码)
- [ ] 密码哈希(bcrypt/argon2)
- [ ] Session管理(单活动会话)
- [ ] CSRF保护
- [ ] Cookie安全(HttpOnly, SameSite=Strict)
- [ ] 设备HMAC-SHA256签名
- [ ] Ed25519配置签名
- [ ] AES-256-GCM加密
- [ ] 主密钥管理
- [ ] 密钥隔离

### 阶段 7: 设备管理
- [ ] 设备注册
- [ ] 设备绑定
- [ ] 设备解绑
- [ ] Token轮换
- [ ] 设备状态管理

### 阶段 8: 映射管理
- [ ] TCP映射创建/修改/删除
- [ ] UDP映射创建/修改/删除
- [ ] HTTP映射创建/修改/删除
- [ ] 端口分配(手动/自动)
- [ ] 端口唯一约束
- [ ] 活动/待定端口租约
- [ ] FRPC配置渲染
- [ ] FRPC配置应用(reload/restart)
- [ ] FRPC配置回滚

### 阶段 9: 域名与DNS管理
- [ ] 域名标准化(IDNA/Punycode)
- [ ] 域名唯一约束
- [ ] 域名预检
- [ ] DNS记录管理(A/AAAA/CNAME)
- [ ] 一键更新服务器IP
- [ ] DNS同步(Cloudflare ↔ 面板)
- [ ] 域名operation状态机

### 阶段 10: Cloudflare集成
- [ ] Token上传/验证
- [ ] Token pending/active/retired
- [ ] Zone提取(标签后缀匹配)
- [ ] DNS CRUD操作
- [ ] 小橙云(proxied)管理
- [ ] Token替换流程
- [ ] Token清除流程

### 阶段 11: 证书管理(ACME)
- [ ] ACME账户管理
- [ ] DNS-01挑战
- [ ] Let's Encrypt集成
- [ ] ZeroSSL集成
- [ ] 证书签发
- [ ] 证书续期(自动/手动)
- [ ] 私钥加密存储
- [ ] 证书热加载

### 阶段 12: Router进程
- [ ] 独立Router进程
- [ ] 80/443监听
- [ ] SNI证书选择
- [ ] Host路由
- [ ] 反向代理到FRPS vhostHTTPPort
- [ ] 版本化快照
- [ ] 快照HMAC校验
- [ ] IPC通知(Unix Socket/Named Pipe)
- [ ] last-good回滚
- [ ] HTTP跳转策略
- [ ] 502离线页面

### 阶段 13: FRPS插件
- [ ] HTTP Plugin接口
- [ ] Login校验
- [ ] NewProxy校验
- [ ] NewWorkConn校验
- [ ] CloseProxy处理
- [ ] Ping处理

### 阶段 14: 后台任务系统
- [ ] 任务租约
- [ ] 心跳续租
- [ ] 去重
- [ ] 重试(指数退避+抖动)
- [ ] Worker接管
- [ ] SQLite事务边界

### 阶段 15: 备份与恢复
- [ ] SQLite快照
- [ ] 加密归档包
- [ ] 恢复流程
- [ ] 版本兼容检查

### 阶段 16: 审计日志
- [ ] 操作记录
- [ ] 敏感字段过滤
- [ ] 查询接口

### 阶段 17: WebSocket通信
- [ ] Server WebSocket Hub
- [ ] Client WebSocket连接
- [ ] 事件通知
- [ ] 重连(指数退避)
- [ ] 全量同步

### 阶段 18: 集成测试
- [ ] 安全测试
- [ ] 并发测试
- [ ] 故障注入
- [ ] 端到端测试

## UI 设计方案

### 配色方案 (参考Vercel/Linear/Stripe)
- 主色: #18181B (深黑)
- 强调色: #2563EB (蓝色)
- 成功色: #16A34A (绿色)
- 警告色: #EAB308 (黄色)
- 错误色: #DC2626 (红色)
- 背景色: #FFFFFF (白色)
- 次背景: #F4F4F5 (浅灰)
- 边框色: #E4E4E7 (灰色)
- 文字色: #18181B (深黑)
- 次文字: #71717A (中灰)

## 技术栈

### 后端
- Go 1.21+
- chi (HTTP Router)
- database/sql + sqlc
- SQLite (WAL模式)
- go-acme/lego
- Cloudflare Go SDK
- WebSocket库

### 前端
- Vue 3
- TypeScript
- Vite
- Element Plus
- Pinia
- Vue Router

### 数据库
- SQLite (WAL, foreign_keys ON, busy_timeout 5000)
