# FRP 多用户云隧道管理平台技术设计方案 v2.3（单活动浏览器会话与工程约束最终定稿版）

> 文档状态：关键架构、认证、设备绑定、单活动浏览器会话、资源事务与路由决策均已定稿，可直接进入接口、数据库与代码开发  
> 架构形态：Server Panel 与 Client Panel 独立编译、独立发布、独立部署  
> 数据库：Server Panel 默认 SQLite（WAL）；不使用临时数据库  
> 能力基线：以 2026-08-01 评审确认的 FRP 与 Cloudflare 能力边界为准，发布时必须锁定并测试具体版本

---

## 0. 文档目标

本文档用于指导一个轻量、安全、可恢复的多用户 FRP 管理平台开发，并作为数据库、API、状态机、后台任务和验收测试的统一基线。

平台必须解决：

1. 普通用户通过本地 Client Panel 管理一个本地 FRPC，不需要直接编辑生产配置。
2. Server Panel 对用户、Client 设备、远程端口、完整域名、DNS、证书和权限拥有最终决定权。
3. 同时支持 TCP/UDP 的 IP + 端口映射，以及基于域名的 HTTP/HTTPS 反向代理。
4. Client Panel 修改配置时必须校验、备份、原子替换、应用验证和失败回滚。
5. Cloudflare Token、FRP Token、设备凭证、配置签名密钥和证书私钥必须按用途隔离保护。
6. Client 离线、配置失败、FRPC 异常、DNS 超时、证书失败或 Router 更新失败时，不破坏最后一份有效配置。
7. Server Panel 与 Client Panel 只通过版本化 HTTPS API 和 WebSocket 通信，不共享业务数据库或源码运行时。
8. 项目以单 Server Panel、单主数据库和轻量部署为第一目标，不引入 Redis、Kafka、Kubernetes 等非必要依赖。

本文档中的“必须”“禁止”“只允许”均属于实现和验收约束，不是建议项。

### 0.1 v2.3 最终定稿决策

以下内容不再保留备选实现：

1. **双产品架构**：Server Panel 与 Client Panel 独立编译、独立发布、独立部署、独立前后端；Client Panel 已承担 Agent 职责。
2. **用户身份来源**：用户账号、密码、角色、停用状态和权限只由 Server Panel 管理；Client Panel 不建立本地管理员、本地用户、本地密码、bootstrap code 或第二套授权体系。
3. **单活动浏览器会话**：每个 `client_id` 同一时间最多存在一个有效浏览器代理会话。新浏览器登录成功后，旧浏览器会话立即失效并返回 `401 SESSION_REPLACED`。
4. **设备与登录分离**：一个 Client Panel 安装实例固定对应一个 Server Panel、一个所属普通用户、一个 `client_id`、一个 FRPC 实例和一套配置。浏览器登录不得注册新设备或改变设备归属。
5. **设备 API 认证**：固定采用 `HTTPS + Timestamp + Nonce + HMAC-SHA256`。签名覆盖 `client_id`、凭证版本、时间戳、Nonce、HTTP 方法、规范化路径、查询串和请求体哈希。
6. **用户 Web 会话**：固定采用“Client Panel 本地代理 Cookie -> Server Panel Web Session”。浏览器不可获得设备 Token、FRP Token或 Server Session 明文。
7. **并发保护**：即使单活动会话，所有写操作仍必须携带 `expected_config_version`、资源 `revision` 和 `Idempotency-Key`。
8. **Router 更新**：Control 生成版本化只读快照，原子写入受保护目录，通过 Unix Socket 或 Windows Named Pipe 通知 Router；Router 验证后原子替换内存路由表，并保留 `last_good_snapshot`。
9. **Client 配置签名**：Server Panel 使用独立 Ed25519 私钥签名规范化完整配置；Client 首次绑定时固定公钥并验证 `key_id`、哈希和签名。
10. **控制 API 暴露**：Server Control 服务仅监听 loopback 或 Unix Socket；公网入口由 Server Router 或独立反向代理通过 HTTPS 暴露。`/internal/frp/*` 仅供本机 FRPS 调用。
11. **传输范围**：v2.3 同时支持 TCP 和 UDP。相同远程端口数字即使协议不同也不得重复，唯一约束为 `UNIQUE(server_id, remote_port)`。
12. **域名关系**：固定为 `mapping 1 -> N domain_bindings`。Mapping 描述本地 HTTP 服务；Domain Binding 描述完整域名；`mapping_revisions` 不保存单一 `custom_domain`。
13. **HTTPS 模式**：仅允许三种合法组合：自动公共证书 + 关闭代理；Cloudflare 代理 + 有效源站证书；仅 HTTP + 关闭代理。
14. **Cloudflare Token 替换**：新 Token 先作为 pending 密文验证，确认对现有 Zone 和 DNS 的能力后再切换 active 版本；失败时继续使用旧 Token。
15. **后台任务**：所有外部操作采用可租约、可接管、可去重、可重试的 jobs/operations 模型；外部 API 调用期间禁止长期持有 SQLite 写事务。
16. **密钥隔离**：业务主密钥、证书私钥封装密钥、Router 快照密钥、Client 配置签名密钥和设备 HMAC 密钥必须相互独立。
17. **正式备份**：完整恢复只使用管理员密码加密的归档包；JSON 仅用于非敏感预览或调试。
18. **首次管理员**：取消固定 `admin/123456`。首次启动创建 `admin` 并生成一次性随机 12 位密码，首次登录必须修改用户名和密码。


## 1. 核心设计原则

### 1.1 双面板，绝不合并

系统只包含两个产品：

- **Server Panel**：安装在 FRPS 公网服务器。
- **Client Panel**：安装在普通用户的本地机器。

Client Panel 已经承担 Agent 的职责，不再增加第三个 Client Agent。

### 1.2 控制面与数据面分离

- FRPS / FRPC 负责隧道数据转发。
- Server Router 负责公网 HTTP/HTTPS 入口、TLS 终止和域名路由。
- Server Panel 控制 API 负责账户、资源、配置、Cloudflare、证书和审计。
- Client Panel 负责本地 FRPC 配置、进程、服务探测和实际状态上报。

不允许让 Web 控制 API 进入每个隧道的数据转发热路径。

### 1.3 期望状态与实际状态分离

Server Panel 保存“期望配置”；Client Panel 保存并上报“实际应用结果”。

- Server Panel 不能因为数据库写入成功就直接显示“运行中”。
- Client Panel 不能因为本地配置存在就认定远程端口或域名归自己。
- WebSocket 只用于通知；最终一致性依靠配置版本和全量同步。

### 1.4 数据库占用优先于在线状态

只要端口或域名仍在数据库中正式注册，就视为占用，不受以下状态影响：

- Client Panel 在线或离线；
- FRPC 正常或异常；
- 用户启用或停用；
- 配置应用成功或失败；
- DNS 或证书申请成功或失败。

只有完成正式删除流程后才释放资源。

### 1.5 安全默认值优先

- Client Panel 默认仅监听 `127.0.0.1`。
- Server 内部接口默认仅监听 loopback 或 Unix Socket。
- 不提供任意命令执行、任意路径读写和任意进程管理接口。
- 所有敏感值默认不可见、不可搜索、不可写日志。
- 不提供“跳过 TLS 验证”的生产开关。

---

## 2. 总体架构

### 2.1 三个必须区分的概念

#### Client Panel 安装实例

指安装在某台本地机器上的 Client Panel 常驻服务。一个已绑定实例：

- 管理且只管理一个本地 FRPC 主进程；
- 拥有一个固定 `client_id`；
- 绑定一个 `server_instance_id` 和一个所属普通用户；
- 保存一份设备后台凭证、一份 FRP 设备凭证和一套本地配置；
- 通过文件锁、命名互斥量或等价机制保证同一数据目录只能启动一个 Supervisor；
- 对应 Server Panel 数据库中的一条 Client 设备记录。

安装实例在首次绑定前只有本地 `installation_instance_id`；注册成功后才获得 `client_id`。

#### 浏览器访问设备

指访问 Client Panel 页面的电脑、手机、平板或其他浏览器。多个浏览器可以访问同一 Client Panel，但不是多个 Client 设备：

```text
运行 Client Panel 的 NAS
├── NAS 本机浏览器
├── 局域网电脑浏览器
├── 局域网手机浏览器
└── 局域网平板浏览器
```

浏览器访问数量不会增加 `client_id`、FRPC 进程或 FRP 设备身份。

#### 活动浏览器会话

每个 Client Panel 同一时间只允许一个有效浏览器代理会话：

```text
每个 client_id 最多一个 active_proxy_session
```

不同浏览器都可以登录，但新浏览器登录成功后必须：

1. 请求 Server Panel 撤销旧 Server Web Session；
2. 删除旧本地代理 Session；
3. `session_generation + 1`；
4. 建立新的本地 Cookie、CSRF 状态和远程 Session 映射；
5. 使旧浏览器后续请求返回 `401 SESSION_REPLACED`。

该限制只作用于当前 `client_id`。同一个用户仍可同时登录并管理其拥有的其他 Client Panel。

### 2.2 正确控制结构

```text
电脑浏览器 ─┐
手机浏览器 ─┼──> 同一个 Client Panel 安装实例
平板浏览器 ─┘             |
                           |-- 最多一个活动浏览器会话
                           |-- 一个 client_id
                           |-- 一个 FRPC Supervisor
                           |-- 一套 frpc.toml
                           |-- 一个串行配置应用队列
                           |
                           | HTTPS REST + WSS
                           v
                    Server Panel Control Plane
                           |
             +-------------+-------------+
             |                           |
             v                           v
           FRPS                 Server Router :80/:443
             |                           |
             +-------------+-------------+
                           |
                           v
                       用户本地服务
```

浏览器只提交操作意图，不直接写配置、控制进程或绕过 Server Panel 申请远程资源。

### 2.3 两套业务访问路径

#### TCP/UDP：IP + 端口

```text
访问者 -> server_ip:remote_port -> FRPS -> FRPC -> local_ip:local_port
```

- 支持 TCP 和 UDP；
- 不需要域名、Cloudflare 或证书；
- 相同远程端口数字跨协议仍然互斥。

#### HTTP/HTTPS：域名模式

```text
访问者 -> Cloudflare（可选） -> Server Router :80/:443
       -> TLS 在 Router 终止 -> 普通 HTTP
       -> FRPS 127.0.0.1:vhost_http_port
       -> FRPC -> 用户本地 HTTP 服务
```

Server Router 统一处理公网 TLS。FRPS 不直接管理每个用户域名证书，也不把浏览器 TLS 原样透传给普通 HTTP 服务。

### 2.4 数据权威边界

Server Panel 是以下内容的最终依据：

- 用户、角色、停用和删除状态；
- Client 设备和所属用户；
- FRP 用户名、Token 和凭证版本；
- 远程端口、完整域名和配额；
- Cloudflare Token 状态、DNS 关系和证书状态；
- 期望配置版本和资源 revision；
- Router 期望版本。

Client Panel 是以下本地事实的最终依据：

- FRPC 是否运行、PID、版本和二进制哈希；
- 当前实际配置文件、哈希和最近成功应用版本；
- 本地目标端口健康；
- verify、reload、restart 和回滚结果；
- 本地日志与错误摘要。

Server 保存“期望状态”，Client 上报“实际状态”；两者不得混为一个 `status` 字段。


## 3. 项目边界与非目标

### 3.1 v2.3 必须完成

- 单管理员、多普通用户；
- 同一用户可拥有多个 Client Panel；
- 每个 Client Panel 一个 `client_id`、一个 FRPC、一个活动浏览器会话；
- TCP 和 UDP 远程端口映射；
- HTTP 域名映射，一个 Mapping 可关联多个完整域名；
- Cloudflare A、AAAA、CNAME、TTL 和小橙云管理；
- Cloudflare DNS 接管、覆盖、同步和删除；
- 自动证书、Cloudflare 代理、仅 HTTP 三种模式；
- FRPC verify、启动、停止、reload、restart、原子应用和回滚；
- FRPS Login、NewProxy、NewWorkConn 等阶段二次鉴权；
- 端口和域名数据库强唯一；
- 设备 HMAC、Client 配置 Ed25519 签名、Router 快照 HMAC；
- 后台任务租约、幂等、重试和补偿；
- 加密备份与恢复；
- 审计、版本兼容和升级阻断策略。

### 3.2 v2.3 非目标

- 多 Server Panel 主节点高可用；
- 跨地域 FRPS 自动调度；
- 用户计费、带宽结算；
- Kubernetes；
- 任意 DNS 服务商；
- 任意 Cloudflare 产品管理；
- STCP、XTCP、SUDP 的完整产品 UI；
- 用户自定义脚本、Shell、Hook 或任意命令执行；
- Client Panel 离线本地管理员绕过远程授权。

数据库可以保留 `server_id` 以便未来扩展，但当前实现以单 Server Panel 和单主 SQLite 为准。


## 4. 技术选型

## 4.1 服务端与客户端后端

统一使用 Go，Server Panel 与 Client Panel 分开仓库或至少分开模块、分开构建。

推荐：

- HTTP Router：`net/http` + `chi`；
- 数据访问：`database/sql` + `sqlc`；
- 数据迁移：`golang-migrate` 或自研顺序迁移器；
- SQLite 驱动：选择固定版本并完成 WAL 并发测试；
- Cloudflare：官方 Go SDK外包一层 Provider 接口；
- ACME：`go-acme/lego`，便于支持 DNS-01、Let's Encrypt、ZeroSSL；
- WebSocket：稳定的 Go WebSocket 库；
- 日志：结构化 JSON 日志，必须经过敏感字段过滤器；
- 前端静态资源嵌入 Go 二进制。

### 为什么不优先使用重量 ORM

端口修改、双端提交、资源锁定和删除清理需要明确事务边界。`sqlc + 显式 SQL` 更容易审计：

- 唯一索引是否真实生效；
- 事务何时开始和结束；
- 修改端口时旧租约何时释放；
- 删除用户时哪些记录仍在等待外部清理；
- SQLite 写锁是否被长时间占用。

## 4.2 前端

两套前端独立构建：

- Vue 3；
- TypeScript；
- Vite；
- Element Plus；
- Pinia；
- 不在 localStorage 保存 Session、密码、Token、设备凭证。

localStorage 仅允许保存无敏感性的界面偏好，以及用户要求的 Server Panel 地址提示值。

## 4.3 数据库

Server Panel 默认 SQLite，必须：

```sql
PRAGMA journal_mode = WAL;
PRAGMA foreign_keys = ON;
PRAGMA busy_timeout = 5000;
PRAGMA synchronous = FULL;
```

建议设置受控 checkpoint，避免 WAL 无限增长。

约束：

- 数据库文件必须位于本机磁盘；
- 禁止将 WAL 数据库放在 NFS、SMB 或其他网络文件系统；
- SQLite 运行版本必须包含 2026 年 WAL-reset 问题修复，最低使用官方已修复版本或其正式回移版本；
- 写事务必须短小，外部 Cloudflare/ACME/FRP 操作绝不能放在数据库事务内等待。

Client Panel 也可使用本地 SQLite 保存非敏感状态，但敏感设备凭证必须通过安全存储保护。

## 4.4 FRP 版本策略

- Server Panel 发布包内置一个经过测试的 FRPS 版本；
- Client Panel 发布包内置同一兼容系列的 FRPC；
- 二进制按 OS/ARCH 构建并随发行包提供 SHA-256；
- 启动时校验内置 FRP 二进制哈希；
- 不允许运行时从任意 URL 下载和执行 FRPC；
- 升级必须通过签名发布清单和受信任下载地址；
- Server Panel 保存兼容矩阵，而不是假设所有 FRP 版本都兼容。

---

## 5. 部署拓扑和端口

### 5.1 Server 端监听与公网入口

| 用途 | 监听地址 | 示例端口 | 暴露范围 |
|---|---:|---:|---|
| Server Router HTTP | `0.0.0.0` / `::` | 80 | 公网 |
| Server Router HTTPS | `0.0.0.0` / `::` | 443 | 公网 |
| FRPS bindPort | `0.0.0.0` / `::` | 7000 | 公网，仅 FRPC |
| FRPS vhostHTTPPort | `127.0.0.1` | 8080 | 仅本机 |
| Server Control | `127.0.0.1` 或 Unix Socket | 9000 | 不直接公网暴露 |
| FRPS 插件接口 | `127.0.0.1` 或 Unix Socket | 9001 | 仅本机 FRPS |
| FRPS Dashboard | `127.0.0.1` | 7500 | 默认关闭 |

固定规则：

- Server Control 只监听 loopback 或 Unix Socket。
- 公网控制面由 Server Router 或独立受信任反向代理通过 HTTPS 暴露。
- 公网入口可同时承载管理员页面、普通用户认证、Client 用户操作和设备同步。
- `/internal/frp/*` 只允许本机 FRPS 访问，必须在 Router 和防火墙层禁止公网进入。
- FRPS `vhostHTTPSPort` 不作为用户域名证书入口。
- 80、443、FRPS 自身端口、Control 端口、插件端口和管理员预留端口必须进入系统保留端口表。

推荐公网路径：

```text
/admin/*
/api/v1/auth/*
/api/v1/devices/*
/api/v1/client/*
/api/v1/mappings/*
/api/v1/domains/*
/api/v1/cloudflare/*
/api/v1/certificates/*
```

内部路径：

```text
/internal/frp/*
```

### 5.2 Client 端监听

| 用途 | 默认地址 | 示例端口 |
|---|---:|---:|
| Client Panel Web/API | `127.0.0.1` / `::1` | 7410 |
| FRPC Admin API | `127.0.0.1` | 动态受控端口 |

FRPC Admin API：

- 使用随机强密码；
- 只允许 Client Panel 调用；
- 不展示给浏览器；
- 不允许被 FRP 映射；
- 不允许绑定公网地址。

### 5.3 Server 地址使用 IP 时的 TLS

生产环境禁止跳过 TLS 验证。使用 `https://IP:port` 时只允许：

1. 服务端证书包含该 IP 的 SAN；
2. Client 导入明确受信任的自定义 CA；
3. 首次连接先只获取证书信息，用户通过独立可信渠道核对 SPKI SHA-256 指纹后进行固定。

指纹未确认前禁止发送用户名和密码。HTTP 仅允许显式开发模式，并持续显示风险警告。


## 6. Server Panel 组件划分

Server Panel 可以是一个代码库和一个发行包，但建议同一二进制支持两个运行模式，以实现进程隔离：

```text
frp-panel-server control
frp-panel-server router
```

### 6.1 Control 进程

包含：

- 管理员 Web UI；
- 用户 Web API；
- Session 服务；
- Client 设备注册；
- WebSocket Hub；
- 配置版本服务；
- 端口与域名分配器；
- Cloudflare Provider；
- ACME 调度器；
- FRPS HTTP Plugin；
- 后台任务；
- 审计日志；
- 备份恢复。

### 6.2 Router 进程

Router 是独立的最小权限数据面进程，只负责：

- 监听 80/443；
- SNI 证书选择与 TLS 终止；
- 精确 Host 路由；
- 按域名执行 HTTP -> HTTPS 策略；
- 反向代理到固定的 FRPS `vhostHTTPPort`；
- 404/502 错误页；
- 导出不含敏感信息的运行指标。

Router **不得**：

- 直接访问业务 SQLite；
- 持有 Cloudflare Token；
- 调用用户、DNS、证书申请等业务 API；
- 修改快照或证书文件；
- 接受公网配置管理请求。

#### 6.2.1 唯一配置分发方案

固定流程如下：

```text
Control 根据已提交的数据库状态生成 router_config_version=N
    ↓
生成版本化只读快照和证书引用清单
    ↓
写入同目录临时文件并 fsync
    ↓
原子 rename 为正式快照
    ↓
原子更新 current 指针文件
    ↓
通过 Unix Socket / Windows Named Pipe 通知 Router
    ↓
Router 读取、校验并构建新的不可变路由表
    ↓
Router 原子替换内存路由表
    ↓
Router 返回 ACK，Control 更新 router_applied_version
```

Linux 推荐目录：

```text
/var/lib/frp-panel/router/
├── snapshots/
│   ├── router-000000000123.json
│   ├── router-000000000123.sha256
│   └── current
├── certificates/
└── last-good.json

/run/frp-panel/router-control.sock
```

Windows 使用受 ACL 保护的数据目录和 Named Pipe，语义保持一致。

Control 和 Router 共享一个独立的 `router_snapshot_key`，仅用于快照 HMAC，不与 Cloudflare Token、证书私钥或设备 HMAC 共用。密钥通过受保护文件或系统 Secret 注入，不写数据库和日志。目录权限必须做到 Control 可写、Router 只读。

#### 6.2.2 快照内容

快照至少包含：

```text
schema_version
router_config_version
generated_at
admin_hosts
routes[]
  normalized_host
  upstream_id
  https_mode
  http_redirect
  certificate_ref
  certificate_hash
  route_status
error_page_profile
snapshot_hash
snapshot_hmac
```

快照只允许引用固定的本机上游标识，v2 中用户业务流量只能转发到受控的 FRPS `vhostHTTPPort`，不允许快照包含任意公网或文件路径上游。

#### 6.2.3 Router 校验

应用前必须检查：

- Schema 版本受支持；
- `router_config_version` 严格递增，除非执行显式回滚；
- 快照 SHA-256 与通知内容一致；
- 使用独立 `router_snapshot_key` 校验快照 HMAC-SHA256，防止受保护目录外的文件替换；
- 所有 Host 已标准化且无重复；
- 管理员域名和用户域名不存在冲突；
- 证书引用位于允许目录；
- 证书与私钥匹配，SAN 覆盖目标域名；
- 路由只能指向允许的本机 FRPS 入口；
- 未知字段按 Schema 兼容规则处理。

任何检查失败时：

- 不修改当前内存路由表；
- 继续使用上一份有效快照；
- 返回结构化错误；
- Control 记录 `last_router_apply_error`；
- 触发告警，但不影响已有域名继续服务。

#### 6.2.4 崩溃恢复和最终一致性

必须维护：

```text
router_config_version
router_applied_version
last_good_snapshot_version
last_good_snapshot_path
last_good_snapshot_hash
```

- Router 重启时首先加载 `last-good.json` 指向的最后有效快照；
- Control 短暂不可用时，Router 继续使用内存和磁盘中的最后有效配置；
- IPC 通知丢失时，Router 每 30 秒检查 `current` 指针作为恢复机制；该轮询不是另一套配置通道；
- 快照至少保留当前版本和最近 3 个有效版本；
- 快照落盘成功但 Router 未 ACK 时，不得把 `router_applied_version` 提前更新；
- Control 可以执行显式回滚，但必须生成新的单调递增版本，而不是直接把版本号倒退。

### 6.3 FRPS 进程

- 由 systemd、Windows Service 或容器编排器托管；
- Server Panel 可以生成和校验配置，但不通过 shell 拼接命令；
- FRPS 配置更新必须先 `frps verify`，再原子替换；
- 生产环境避免为了单个用户操作频繁重启 FRPS。

---

## 7. Client Panel 组件划分

Client Panel 是完整本地服务，不是纯静态页面，也不是独立身份提供方。

```text
Client Panel
├── Local Web UI
├── Local HTTP/HTTPS API
├── Active Browser Session Manager
├── Remote Session Proxy
├── Server Binding Manager
├── Server API Client
├── Device HMAC Signer
├── WebSocket Client
├── Sync Engine
├── Desired Config Renderer
├── Config Signature Verifier
├── Config Apply Queue（单写者）
├── FRPC Supervisor（单实例）
├── Local Port Probe
├── Status Collector
├── Log Manager
├── Secure Storage Adapter
└── Update Checker
```

Client Panel 明确不包含：

- 本地用户数据库；
- 本地管理员、用户名、密码或角色；
- bootstrap code；
- 独立于 Server Panel 的授权系统；
- 任意命令执行器。

### 7.1 安装实例与单 FRPC 约束

同一数据目录必须通过文件锁或系统互斥量保证：

- 只能运行一个 Client Panel 主进程；
- 只能托管一个 FRPC 主进程；
- 所有配置和进程操作进入一个串行队列；
- FRPC PID 必须同时校验启动时间和可执行文件哈希；
- 浏览器登录、刷新或抢占 Session 不得启动第二个 FRPC。

### 7.2 Client Panel 的本地最终决定权

Client Panel 负责确认：

- FRPC 实际运行状态；
- 当前配置哈希和已应用版本；
- 本地目标服务是否可达；
- verify、reload、restart 和回滚结果；
- FRPC 实际代理状态；
- 本地日志和错误摘要。

Client Panel 不得自行判定端口或域名归属，也不能使用设备 HMAC 替代用户 Session 执行用户授权型写操作。

### 7.3 Active Browser Session Manager

Client Panel 只维护一个 `active_proxy_session`，必须支持：

- 原子抢占；
- `session_generation`；
- 旧 Session 撤销；
- 旧 Cookie 立即失效；
- `401 SESSION_REPLACED`；
- Client 重启后会话清空但设备后台同步继续。

禁止实现多会话 Registry、全局持久化 Server Session 或“退出所有浏览器会话”功能。


## 8. Client Panel 自身访问安全

Client Panel 可修改配置和控制 FRPC，按高权限本地服务设计，但不建立第二套账号。

### 8.1 默认监听

- 默认仅监听 `127.0.0.1` 和可选 `::1`；
- 禁止默认监听 `0.0.0.0`；
- 监听地址只能通过受保护系统配置修改，不能由普通请求临时覆盖；
- 默认拒绝未知 Host；
- WebSocket 必须校验 Host 和 Origin；
- loopback HTTP 仍必须启用 Cookie、CSRF、CSP 和请求体限制。

### 8.2 局域网访问

用户显式开启后必须同时满足：

1. 绑定指定局域网地址，不无条件开放所有接口；
2. 配置 IP/CIDR 白名单；
3. 配置 Host 白名单；
4. 校验 WebSocket Origin；
5. 禁止 CORS 通配符；
6. 正式环境强制本地 HTTPS；
7. 对来源 IP 增加登录和 API 限流；
8. 限制请求体、上传类型和并发连接数；
9. 未登录请求只能访问登录页、静态资源、健康检查和 Server 地址预检；
10. 仍然只使用 Server Panel 用户名和密码，不创建本地密码。

### 8.3 认证与单会话边界

- 用户只由 Server Panel 认证；
- Client 只维护一个与远程 Server Session 绑定的活动代理会话；
- 新登录成功必须撤销旧 Server Session 和旧本地代理 Session；
- Server Session 失效、用户停用、设备撤销或 Session 代际不匹配时，本地权限立即失效；
- 设备 HMAC 仅用于后台心跳、配置同步和结果上报；
- 设备 HMAC 不得调用用户授权型创建、修改、删除 API。

### 8.4 高风险操作

以下操作要求有效 Server Session、CSRF 校验和短时 `reauth_ticket` 或等价二次认证票据：

- 停止或重启 FRPC；
- 修改需要 restart 的 FRP 公共参数；
- 清除 Cloudflare Token；
- 重置 FRP Token；
- 强制删除映射或域名；
- 切换 Server Panel；
- 更换设备所属用户；
- 退出并解除设备；
- 删除回滚配置。

不要求输入另一套本地密码。

### 8.5 禁止能力

禁止：

- 任意 Shell、PowerShell 或命令执行；
- 用户指定任意可执行文件；
- 任意系统服务控制；
- 任意目录读写；
- 任意下载后执行；
- 访问 FRPC 受控目录之外的配置路径；
- 未经 Server Panel 批准直接申请端口、域名或 DNS；
- 通过浏览器传入原始进程参数。

所有进程调用必须使用固定二进制路径和参数数组，不经过 Shell。

### 8.6 Web 安全

- 严格 CSP、点击劫持保护和 MIME 嗅探保护；
- 活动代理 Cookie 使用 `HttpOnly`、`SameSite=Strict`，HTTPS 时增加 `Secure`；
- 所有状态变更请求校验 CSRF；
- 浏览器不保存 Server Session、设备凭证、FRP Token 或 Cloudflare Token；
- 日志不记录登录请求体、Cookie、Authorization、CSRF Secret 或敏感响应；
- 旧 Session 被抢占后，HTTP 和 WebSocket 都必须立即终止。


## 9. Client Panel 本地存储设计

### 9.1 持久化内容

首次启动生成不可变 `installation_instance_id`。绑定后持久化：

- `server_instance_id`；
- 规范化主 `normalized_server_url` 和已验证别名；
- `server_binding_revision`；
- `owner_user_id` 非敏感标识；
- `client_id`；
- 加密后的 `device_token` 和 FRP 设备凭证；
- Token 版本与撤销状态；
- Server TLS 信任模式、自定义 CA 引用或固定 SPKI 指纹；
- `desired_config_version`、`applied_config_version`；
- 当前有效配置、上一份回滚配置及 SHA-256；
- Client 配置签名公钥、`config_signing_key_id`；
- FRPC PID、启动时间、二进制版本和哈希；
- 最近同步、心跳和错误摘要；
- 本地监听、HTTPS、Host、Origin 和 CIDR 白名单设置；
- Client Panel、FRPC、协议和 Schema 版本。

禁止持久化：

- 用户名和密码；
- Server Web Session；
- 活动代理 Session 或 Cookie；
- Cloudflare Token；
- 证书私钥明文。

### 9.2 浏览器 localStorage

每个浏览器可独立保存：

- Server Panel 地址输入值；
- 非敏感 UI 偏好。

禁止保存用户名、密码、任何 Session、Cookie、设备 Token、FRP Token、Cloudflare Token和私钥。

浏览器保存的地址只是输入便利项；Client 后端保存并验证的绑定地址才是后台连接真实配置。

### 9.3 活动代理 Session

Client Panel 只保存一个 `active_proxy_session`，默认仅存在内存：

```text
local_proxy_session_id
cookie_secret_hash
server_session_token
server_csrf_state
server_session_id
user_id
client_id
source_ip
user_agent
created_at
last_seen_at
expires_at
session_generation
```

- 不写本地 SQLite；
- 不写日志；
- Client 重启后失效；
- FRPC 和设备后台同步不因浏览器会话丢失而停止。

### 9.4 推荐目录

Linux system service：

```text
/etc/frp-client-panel/
/var/lib/frp-client-panel/
/var/lib/frp-client-panel/client.db
/var/lib/frp-client-panel/frpc/current/
/var/lib/frp-client-panel/frpc/rollback/
/var/lib/frp-client-panel/secrets/
/var/log/frp-client-panel/
```

Windows：

```text
C:\ProgramData\FRPClientPanel\
```

macOS：

```text
~/Library/Application Support/FRPClientPanel/
~/Library/Logs/FRPClientPanel/
```

Docker：

```text
/data
/run/secrets/frp_client_panel_master_key
```

`/data` 必须持久化，容器重建不得改变安装实例或设备身份。

### 9.5 设备秘密保护

优先使用：

1. Windows DPAPI；
2. macOS Keychain；
3. Linux Secret Service；
4. Docker Secret；
5. 受保护 `0600` 主密钥文件 + AES-256-GCM。

每次加密使用新 Nonce，AAD 包含 `client_id`、秘密类型、Token 版本和 Schema 版本。HMAC 签名密钥通过 HKDF-SHA256 从 `device_token` 派生。

### 9.6 卸载

- 普通卸载默认保留数据；
- Purge 必须二次确认；
- 在线 Purge 先撤销设备再删除本地秘密；
- 离线 Purge 明确提示服务端设备记录仍需管理员撤销。


## 10. Client Panel 连接、绑定、登录与登出

### 10.1 登录页面

所有浏览器看到相同页面：

```text
用户名
密码
登录

配置连接域名/IP
```

点击小字后展开地址输入框，支持：

```text
https://panel.example.com
https://203.0.113.10:8443
```

未配置时提示“请先配置服务端连接地址”。

### 10.2 登录本质

```text
浏览器
  -> Client Panel
  -> HTTPS
  -> Server Panel
  -> 验证用户名、密码、状态和设备归属
```

Client Panel 只负责页面、TLS 验证、密码短暂转发、远程 Session 保存和本地代理 Cookie。用户密码：

- 只用于当前登录请求；
- 不写数据库、localStorage 或日志；
- 不进入错误信息；
- 请求完成后尽快释放内存引用；
- 不用于后台心跳。

### 10.3 Server 地址规范化与实例识别

地址必须：

- 只允许 `https`；HTTP 仅开发模式；
- 禁止 URL 中携带用户名密码；
- 禁止任意路径，规范化为服务端根地址；
- 解析后执行 SSRF 防护；
- 验证证书链、SAN 或已固定 SPKI；
- 请求只读实例接口获取 `server_instance_id`。

相同 `server_instance_id` 的 IP、域名或别名可以更新为同一服务端地址，但新地址必须独立通过 TLS 验证。

不同 `server_instance_id` 不得覆盖，必须进入“切换服务端”危险流程：停止 FRPC、撤销旧设备、清除凭证、使会话失效、重新登录并注册。

### 10.4 设备首次注册

设备注册只发生于：

- 首次绑定；
- 本地设备凭证丢失或被撤销；
- 主动解除后重新绑定；
- 切换 Server Panel；
- 主动更换所属用户。

注册请求包含有效用户 Session、`installation_instance_id`、设备名称、版本信息和 `Idempotency-Key`。成功返回：

```text
client_id
device_token
frp_device_token
config_signing_public_key
config_signing_key_id
```

秘密仅返回一次并立即进入安全存储。

### 10.5 已绑定设备的浏览器登录

Server Panel 必须校验：

- 用户状态为 active；
- 设备未撤销、未停用；
- 当前用户等于 `owner_user_id`；
- 协议和客户端版本兼容。

不匹配返回 `CLIENT_OWNER_MISMATCH`。浏览器登录不得创建新设备、重置凭证、重启 FRPC 或改变归属。

### 10.6 单活动会话抢占流程

```text
新浏览器提交用户名和密码
  -> Server Panel 验证并创建新 Server Web Session
  -> Client Panel 获取 active_proxy_session 锁
  -> 若已有旧会话，先撤销旧 Server Session
  -> 删除旧本地代理会话并关闭旧 WebSocket
  -> session_generation + 1
  -> 创建新本地 Cookie、CSRF 和代理会话
  -> 新浏览器进入面板
```

旧浏览器后续请求：

```text
401 SESSION_REPLACED
```

提示：

```text
当前账号已在另一台设备登录，本次会话已失效。
```

抢占失败时不得同时保留两个活动 Session。若新 Server Session 已创建但本地切换失败，必须撤销新 Session并保留或恢复旧会话的一致状态。

### 10.7 登录后的后台通信

- 用户交互 API 使用当前 Server Web Session；
- 心跳、配置拉取、应用结果和设备 WebSocket 使用设备 HMAC；
- FRPC 与 FRPS 使用独立 FRP 设备凭证；
- 三种凭证不得混用。

### 10.8 Server 断线

已有且本地尚未过期的活动会话只允许查看：

- FRPC 运行状态；
- 当前配置版本和缓存映射；
- 本地服务健康；
- 脱敏日志。

禁止：

- 新登录；
- 创建、修改、删除映射；
- DNS、域名、证书、Token 操作；
- 解除或切换设备；
- 通过网页 start、stop、reload 或 restart FRPC。

紧急 FRPC 控制只能通过操作系统服务命令，不建立本地管理员绕过。

### 10.9 退出登录

只处理当前唯一活动会话：

1. 撤销当前 Server Web Session；
2. 删除 `active_proxy_session`；
3. 清除浏览器 Cookie；
4. 关闭用户 WebSocket；
5. 不停止 FRPC；
6. 不撤销设备；
7. 不删除配置；
8. 返回登录页。

不提供“退出当前 Client Panel 上的全部浏览器会话”，因为最多只有一个活动会话。

### 10.10 退出并解除当前设备

要求二次认证票据：

1. 撤销 `client_id`；
2. 撤销设备 HMAC 和 FRP 设备凭证；
3. 停止 FRPC；
4. 停止后台同步；
5. 删除本地设备秘密；
6. 使活动浏览器会话失效；
7. 将 Client Panel 改为未绑定状态；
8. 让用户选择保留或删除本地 FRPC 配置。

### 10.11 切换所属用户

其他用户不能直接接管设备。必须先解除当前设备，再由新用户重新注册；不得通过普通登录隐式修改 `owner_user_id`。


## 11. 用户、Server 实例、Client 设备与 FRP 凭证模型

### 11.1 用户创建

管理员创建普通用户时自动生成：

- 用户 ID、用户名和 `active` 状态；
- 初始随机密码或一次性邀请；
- FRP 用户名；
- 用户级手工 FRP Token；
- 凭证版本；
- 默认 Client、映射、域名和 Pending 配额。

初始秘密只显示一次，首次登录必须修改。

### 11.2 首次 Server Panel 启动

- 生成不可变 `server_instance_id`；
- 创建默认管理员用户名 `admin`；
- CSPRNG 生成 12 位一次性随机密码，排除易混淆字符；
- `must_change_password=true`；
- 首次登录必须修改用户名和密码；
- 密码不得进入常规日志。

Docker 环境不能只依赖标准输出，可选择：

- 初始化环境变量；
- 权限 `0600` 的一次性初始化文件；
- 本机 CLI 重置命令。

这是对固定 `admin/123456` 的安全修订。

### 11.3 Client 设备归属

一个已绑定安装实例固定对应：

```text
一个 server_instance_id
一个 owner_user_id
一个 client_id
一个 FRPC
一套配置
```

同一用户可拥有多个 Client Panel；其他用户登录返回 `CLIENT_OWNER_MISMATCH`。归属变化只能通过显式解除和重新注册。

### 11.4 设备注册与 HMAC 凭证

注册生成：

- `client_id`；
- 一次性返回的 `device_token`；
- 独立 `frp_device_token`；
- Token 版本；
- Client/FRPC/协议/Schema 版本。

设备 API 固定使用派生签名密钥：

```text
signing_key = HKDF-SHA256(
  input_key_material = device_token,
  salt = client_id,
  info = "frp-panel-device-api-v1"
)
```

Server 不保存原始 `device_token`，保存：

- Token 哈希，用于标识和撤销；
- 使用业务主密钥加密的派生签名密钥；
- Nonce、AAD、密钥版本和状态。

FRP 设备凭证与设备 API 凭证必须分离。

### 11.5 浏览器登录不改变设备身份

登录只创建或抢占一个活动代理 Session，禁止：

- 新建 Client 设备；
- 修改 `owner_user_id`；
- 生成新 `client_id`；
- 轮换设备 Token；
- 重置 FRP Token；
- 启动第二个 FRPC。

### 11.6 密码修改和停用

- 修改用户密码默认不撤销设备后台凭证；
- 用户 Web Session 按策略撤销，默认撤销除当前重新认证会话外的其他 Server Session；
- 当前 Client 仍只允许一个活动代理会话；
- 管理员停用用户时，全部 Server Session、活动代理 Session、设备 API 和 FRP 鉴权立即失效；
- 端口和域名继续占用。


## 12. 单活动用户 Web Session 与 Client 代理会话

### 12.1 唯一身份来源

用户身份只由 Server Panel 认证。Client Panel 没有本地用户、密码、角色或离线授权能力。

### 12.2 固定实现

```text
浏览器本地 Cookie
  -> Client active_proxy_session
  -> Server Web Session
  -> Server Panel 用户权限
```

浏览器只持有不可预测的本地 Cookie。Client 进程内存持有远程 Session；设备 Token 永不发送给浏览器。

### 12.3 活动会话结构

```text
local_proxy_session_id
cookie_secret_hash
server_session_token
server_csrf_state
server_session_id
user_id
client_id
source_ip
user_agent
created_at
last_seen_at
expires_at
session_generation
```

不得持久化 Server Session 原值。

### 12.4 会话唯一性

- 每个 `client_id` 最多一个未撤销活动会话；
- 新登录必须在互斥锁或事务保护下抢占；
- Server 数据库使用部分唯一索引或等价约束保证活动会话唯一；
- Client 内存只维护一个 `active_proxy_session`；
- 旧 Cookie 的 `session_generation` 不匹配时立即返回 `SESSION_REPLACED`；
- HTTP 与 WebSocket 必须使用相同代际校验。

### 12.5 Cookie 和 CSRF

本地 Cookie：

- 随机至少 256 bit；
- `HttpOnly`；
- `SameSite=Strict`；
- HTTPS 时 `Secure`；
- 绑定活动 Session 和 generation；
- 不包含 Server Session Token。

CSRF：

- 每次新登录和抢占后重新生成；
- 状态变更请求必须校验；
- 不接受 Cookie 外的设备凭证代替 CSRF。

### 12.6 撤销事件

以下事件使活动会话失效：

- 新浏览器登录成功；
- 用户主动退出；
- Server Session 过期或被撤销；
- 用户被停用或删除；
- 设备被撤销；
- 退出并解除设备；
- Client Panel 重启；
- 权限或认证版本要求重新登录。

单个浏览器退出不影响同一用户在其他 `client_id` 上的 Session。

### 12.7 二次认证

敏感操作使用 Server Panel 签发的短时 `reauth_ticket`，绑定：

- user_id；
- server_session_id；
- client_id；
- 操作类型；
- 资源范围；
- 过期时间；
- 一次性 Nonce。

设备 HMAC 不得替代该票据。


## 13. Server Panel 与 Client Panel 通信

### 13.1 协议

使用：

```text
HTTPS REST API + WebSocket over TLS
```

REST 是事实同步通道，WebSocket 是低延迟通知通道。

### 13.2 心跳

- 每 30～60 秒发送；
- 加入 ±20% 随机抖动；
- 状态变化时立即上报；
- 服务端以连续多个心跳窗口判断离线；
- 离线只影响状态，不释放端口和域名。

### 13.3 WebSocket 重连

- 指数退避，例如 1、2、4、8、16、30、60 秒；
- 添加随机抖动；
- 收到 `Retry-After` 时遵守；
- 网络恢复后先执行版本对比和全量同步，不盲目重放旧事件。

### 13.4 设备 API 请求认证（最终方案）

后台设备 API 和同步 WebSocket 固定使用：

```text
HTTPS + Timestamp + Nonce + HMAC-SHA256
```

不保留 Bearer Token 备选实现。

#### 13.4.1 请求头

```text
X-Client-ID: <client_id>
X-Device-Token-Version: <integer>
X-Request-Timestamp: <Unix UTC seconds>
X-Request-Nonce: <base64url 128-bit random>
X-Content-SHA256: <lowercase hex>
Authorization: Device-HMAC-SHA256 Signature=<lowercase hex>
Idempotency-Key: <uuid>        # 所有变更请求必须
```

GET/HEAD 无请求体时，Body Hash 使用空字节串的 SHA-256。

#### 13.4.2 规范签名串

```text
client_id + "\n" +
token_version + "\n" +
timestamp + "\n" +
nonce + "\n" +
uppercase(http_method) + "\n" +
normalized_path_and_query + "\n" +
body_sha256
```

签名：

```text
signature = HMAC-SHA256(signing_key, canonical_request)
```

要求：

- Path 必须按协议文档执行唯一规范化；
- Query 参数按键和值排序并进行固定百分号编码；
- 签名前不得自动把两个不同 URL 规范化为同一资源；
- Body Hash 基于实际发送的原始字节，而不是重新序列化后的 JSON；
- 签名使用常量时间比较。

#### 13.4.3 服务端验证顺序

1. TLS 验证通过；
2. 查找 `client_id` 和 Token 版本；
3. 检查用户、设备和凭证状态；
4. 检查时间戳，默认允许服务器时间前后 300 秒；
5. 检查 Body Hash；
6. 解密派生签名密钥并验证 HMAC；
7. 对 `(client_id, token_version, nonce)` 执行唯一插入；
8. 检查 API 权限、配置版本和幂等键；
9. 执行业务操作。

Nonce 记录保留至少 10 分钟，并定期清理。服务重启不能导致最近 Nonce 全部丢失，因此使用 SQLite 持久化唯一约束，而不是只用进程内缓存。

时间偏差错误返回服务端时间，但不回显签名串或秘密。

#### 13.4.4 幂等性

- 所有 POST/PATCH/DELETE 设备请求必须携带 `Idempotency-Key`；
- 唯一约束至少覆盖 `(client_id, method, path, idempotency_key)`；
- 重试相同请求返回首次执行结果；
- 相同幂等键配不同 Body Hash 时拒绝；
- HMAC 防重放与业务幂等是两个独立机制，不能互相替代。

#### 13.4.5 WebSocket

WebSocket 握手对 GET Path 和 Query 使用同一 HMAC 方案。连接建立后：

- 每条高价值客户端消息携带单调递增序号；
- 服务端拒绝倒退或重复序号；
- 断线重连必须重新签名握手；
- 用户/设备停用或 Token 撤销时立即关闭连接。

#### 13.4.6 Token 轮换

计划轮换采用两阶段：

1. Server 创建 pending 凭证并只返回一次新 `device_token`；
2. Client 安全保存后，用新签名密钥发送确认请求；
3. Server 激活新版本并撤销旧版本；
4. Client 删除旧秘密。

安全撤销和用户停用不提供宽限期，立即拒绝旧版本。

#### 13.4.7 日志要求

必须在 HTTP 中间件最前层过滤：

- `Authorization`；
- `Cookie`；
- `Set-Cookie`；
- `device_token`；
- HMAC 签名；
- Nonce 原值（最多记录不可逆短指纹）；
- 登录请求体。

反向代理、错误追踪和调试模式都不得绕过过滤器。

### 13.5 API 错误格式

```json
{
  "error": {
    "code": "PORT_ALREADY_RESERVED",
    "message": "远程端口已被占用",
    "details": {},
    "request_id": "..."
  }
}
```

外部接口不返回堆栈、SQL、Token、私钥路径或内部主机信息。

---

## 14. 配置版本、并发写入和同步机制

取消多活动浏览器会话不等于可以删除并发保护。冲突仍可能来自多标签页、重复点击、网络重试、管理员操作、后台同步和 WebSocket 通知。

### 14.1 版本字段

每个 Client 维护：

```text
desired_config_version
applied_config_version
last_failed_config_version
```

每个资源维护：

```text
resource_revision
mapping_revision
```

创建、修改、删除映射、FRP Token 重置、管理员变更和强制全量同步都必须增加期望版本。

### 14.2 写请求前置条件

所有资源写请求必须携带：

```text
expected_config_version
resource_revision 或 mapping_revision
Idempotency-Key
```

旧版本返回：

```text
409 CONFIG_VERSION_CONFLICT
409 RESOURCE_REVISION_CONFLICT
```

前端提示“配置已发生变化，请刷新后重新操作”。

### 14.3 幂等

`Idempotency-Key` 至少与以下信息绑定：

- 用户；
- client_id；
- HTTP 方法；
- 规范化路径；
- 请求体哈希；
- 操作类型。

相同键和相同请求必须返回同一 operation 或缓存结果；相同键但不同请求体返回 `409 IDEMPOTENCY_KEY_REUSED`。

HMAC Nonce 防重放与业务幂等是两套独立机制。

### 14.4 配置快照与 Ed25519 签名

Server 生成规范化完整配置：

```text
client_id
config_version
schema_version
generated_at
config_body
config_hash
config_signing_key_id
config_signature
```

规则：

- `config_hash = SHA-256(canonical_config)`；
- `config_signature = Ed25519.Sign(private_key, canonical_envelope)`；
- Client 首次绑定时固定签名公钥；
- 签名覆盖 Client ID、版本、Schema、哈希和完整配置；
- 私钥与业务主密钥、Router HMAC、设备 HMAC 分离；
- 支持 `key_id` 和受控密钥轮换；
- 签名失败、版本倒退、Schema 不兼容或 Client ID 不匹配时拒绝应用。

### 14.5 全量同步

WebSocket 只发送“版本变化”通知。Client 必须通过设备 HMAC 拉取完整配置并比较版本，不能依赖增量事件。

上线、重连、版本不一致、签名公钥轮换或 Server 强制同步时，必须执行全量同步。

### 14.6 串行应用

Client 只有一个配置应用 Worker。即使用户快速连续操作，也只能按期望版本顺序应用；过时任务在开始前应被合并或取消，不能让旧配置覆盖新配置。


## 15. FRPC 安装、托管与升级

### 15.1 FRPC 来源

Client Panel 发行包内置指定 FRPC：

- Linux amd64/arm64；
- Windows amd64/arm64；
- macOS amd64/arm64。

每个二进制：

- 固定版本；
- 固定 SHA-256；
- 启动前校验；
- 不允许用户替换为任意路径的可执行文件。

可提供高级设置选择管理员认可的兼容版本，但仍必须通过签名和哈希验证。

### 15.2 进程托管

- Client Panel 是父进程或监督者；
- 不通过 shell 启动；
- 使用独立进程组；
- 支持优雅停止，超时后强制结束；
- 保存 PID、进程启动时间、二进制哈希；
- 启动时识别遗留进程，不能只凭 PID 杀进程；
- Windows 使用 Job Object；
- Linux 使用进程组和可选 systemd scope；
- macOS 使用进程组。

### 15.3 FRPC Admin API

为了使用 `reload` 和 `status`：

- 绑定 `127.0.0.1`；
- 使用动态空闲端口；
- 使用随机强用户名和密码；
- 不在 UI 展示；
- 不允许通过 FRP 映射；
- Client Panel 调用时设置短超时。

### 15.4 升级策略

- Client Panel 检查 Server 返回的最低/最新版本；
- 普通过旧版本显示提醒；
- 存在协议或安全不兼容时禁止继续提交配置；
- 自动下载必须验证发布签名和 SHA-256；
- 默认不静默自动安装，除非用户明确开启；
- 升级失败回滚旧二进制。

---

## 16. FRPC 配置组织

推荐使用主配置 + `includes`：

```text
frpc/
├── current/
│   ├── frpc.toml               # 稳定公共配置
│   └── conf.d/
│       └── proxies.toml        # Server期望代理
├── candidate/
└── rollback/
```

主配置包含：

- Server 地址和端口；
- FRP 用户；
- FRP 传输认证；
- Client/设备元数据；
- FRPC Admin API；
- `includes`；
- 日志设置。

代理片段包含：

- 代理名称；
- 类型；
- localIP/localPort；
- remotePort 或 customDomains；
- `mapping_id`、`mapping_revision` 元数据；
- 启用状态。

代理名称使用稳定且不可碰撞格式，例如：

```text
m_<mapping_uuid_without_dash>
```

不允许使用用户自由输入的名称直接作为内部代理唯一键。

### 16.1 敏感值写入

- Cloudflare Token 永不进入 FRPC 配置；
- Server API device_token 永不进入 FRPC 配置；
- FRP 鉴权秘密优先通过受保护文件、环境模板或进程内注入；
- 若必须写文件，权限为 `0600`/严格 ACL，并从日志和备份预览中排除。

---

## 17. FRPC 配置安全应用流程

```text
收到配置版本通知
    ↓
拉取完整期望配置快照
    ↓
检查 protocol/config_schema 兼容性
    ↓
渲染到 candidate 临时目录
    ↓
限制路径并检查文件权限
    ↓
执行 frpc verify -c candidate/frpc.toml
    ↓
备份 current 到 rollback（原子目录切换前）
    ↓
原子替换 current 配置
    ↓
判断是 reload 还是 restart
    ↓
执行并等待
    ↓
调用 frpc status / Admin API 检查代理
    ↓
执行本地和远程状态核对
    ↓
成功：提交 applied_version
失败：回滚旧配置并恢复旧进程
```

### 17.1 reload 与 restart 判定

FRP 官方能力限制：`frpc reload` 主要用于代理项变化，公共配置参数不能普遍动态修改。

因此：

- 创建、删除、启停代理：优先 reload；
- 修改 localIP/localPort/remotePort/customDomains：先尝试 reload；
- 修改 Server 地址、Server 端口、FRP 用户、认证 Token、TLS、Admin API：必须 restart；
- FRP Token 重置：必须 restart，不能只 reload；
- Client Panel 应把判定逻辑写成明确的配置差异分类，而不是统一调用 reload。

### 17.2 原子替换

- 同一文件系统内写临时文件；
- `fsync` 文件；
- `fsync` 目录；
- rename 替换；
- Windows 使用等价原子替换方案；
- 保留最近至少 2 份成功配置；
- 回滚配置必须经过校验。

### 17.3 应用失败

失败时：

1. 停止错误的新 FRPC 或撤销错误 reload；
2. 恢复上一份有效配置；
3. 重新启动或 reload 旧配置；
4. 检查旧代理是否恢复；
5. 设置 observed 状态 `config_error`；
6. 保存脱敏错误摘要和完整本地日志；
7. 向 Server Panel 上报失败版本；
8. 不自动删除服务端资源。

---

## 18. 本地服务检测

创建或修改映射前，Client Panel 检查目标服务。检测只用于本地可用性提示，不能代替 Server Panel 资源授权。

### 18.1 TCP

- 连接 `local_ip:local_port`；
- 默认超时 2 秒；
- 最多重试 2 次；
- 不发送任意应用层数据。

### 18.2 UDP

UDP 没有可靠的通用“监听成功”握手。检测策略：

- 校验目标 IP 和端口格式；
- 可选发送用户明确启用的协议探测包；
- 默认不发送未知数据；
- 如果没有应用层响应，显示“UDP 端口无法通过通用方式确认”，而不是判定服务一定离线；
- 最终运行状态结合 FRPC/FRPS 代理状态和业务侧观测。

### 18.3 HTTP

可选检查：

- 默认 path `/`；
- 可配置健康状态码范围；
- 限制响应体读取大小；
- 限制重定向次数；
- 不跟随到未授权公网或本机敏感地址的重定向；
- 清理代理环境变量，避免探测经过非预期代理。

### 18.4 检测失败

本地服务不可达时：

- 默认阻止并提示；
- 用户可明确选择“保留服务端资源，稍后配置”；
- 状态为 `reserved/pending_apply`，不得显示 `running`；
- 仍受 Pending 配额限制。


## 19. 映射数据模型和状态机

### 19.1 Mapping 定义

Mapping 描述一个本地服务和 FRP 代理，不直接存储单一域名：

```text
id
user_id
client_id
name
proxy_type          # tcp / udp / http
local_ip
local_port
lifecycle_status
desired_state
observed_state
active_revision
pending_revision
```

- `tcp`、`udp` 使用远程端口；
- `http` 通过关联的 Domain Binding 生成 FRPC `customDomains`；
- `mapping_revisions` 禁止保存 `custom_domain` 字段。

### 19.2 Mapping 与 Domain

固定关系：

```text
mapping 1 -> N domain_bindings
```

一个本地 HTTP 服务可以绑定多个域名。删除 Mapping 时必须显式处理其所有 Domain Binding；删除单个 Domain 不应删除 Mapping。

### 19.3 状态

```text
reserved
pending_apply
running
offline
config_error
disabled
deleting
```

- `reserved`：服务端资源已保留；
- `pending_apply`：等待 Client 应用；
- `running`：Client 和 FRPS 均确认代理成功；
- `offline`：Client 或 FRPC 离线，资源仍占用；
- `config_error`：本地应用失败；
- `disabled`：主动停用；
- `deleting`：删除 operation 进行中。

创建成功必须区分“资源保留成功”和“客户端应用成功”。

### 19.4 Revision

修改 Mapping 生成 pending revision。旧 revision 在新配置应用成功前继续有效；失败时释放新资源并保留旧 revision。


## 20. 远程端口管理

### 20.1 唯一约束

```sql
CREATE UNIQUE INDEX ux_port_leases_server_port
ON port_leases(server_id, remote_port);
```

TCP 和 UDP 使用相同唯一空间，协议不进入唯一键。

### 20.2 禁止端口

维护：

- 可分配范围；
- 系统保留端口；
- 管理员预留端口；
- 80、443；
- FRPS bindPort；
- FRPS vhostHTTPPort；
- Control、插件和面板端口；
- 操作系统保留或安全敏感端口。

### 20.3 手动指定

前端预检查仅用于提示。最终占用只能依靠数据库短事务和唯一索引。

### 20.4 自动分配

1. 在允许范围选择候选；
2. `BEGIN IMMEDIATE` 开启短事务；
3. 尝试插入 `port_leases`；
4. 唯一冲突则换下一个候选；
5. 成功后提交；
6. 禁止把“先查询空闲、后插入”当最终锁定。

可以随机化候选起点，降低集中冲突。

### 20.5 占用规则

以下状态仍占用：

- Client 离线；
- FRPC 异常；
- 用户停用；
- config_error；
- pending_apply；
- mapping disabled；
- 修改过程中的 active 或 pending 租约。

只有正式删除租约才释放。

### 20.6 Pending 资源配额

为防止永久 Pending 占满资源，每个用户至少配置：

```text
max_pending_mappings
max_pending_port_leases
max_pending_domain_operations
max_certificate_jobs
```

规则：

- Pending 不因超时自动释放，避免破坏用户已保留资源；
- 超过配额拒绝新建并返回明确占用列表；
- 用户可主动取消；
- 管理员可取消；
- 长期 Pending 产生提醒；
- 面板显示占用原因、创建时间和关联 operation；
- 所有取消和强制释放写审计日志。


## 21. 创建映射完整流程

```text
用户在 Client Panel 填写 localIP/localPort/协议/remotePort
    ↓
Client Panel 校验字段并探测本地端口
    ↓
POST Server Panel，带 Idempotency-Key 和 expected_version
    ↓
Server Panel 权限和配额检查
    ↓
数据库短事务内插入端口租约和 mapping
    ↓
mapping.lifecycle = reserved
    ↓
增加 desired_config_version
    ↓
提交事务
    ↓
Client 拉取完整配置
    ↓
Client 原子应用 FRPC 配置
    ↓
FRPS Login/NewProxy 插件二次鉴权
    ↓
Client 检查代理建立结果
    ↓
上报 applied 或 failed
```

创建成功必须分两个阶段显示：

1. **资源保留成功**；
2. **客户端应用成功**。

若 Client 离线，端口仍保持 reserved/pending_apply，不自动释放。

---

## 22. 修改映射完整事务流程

以远程端口从 6000 修改为 7000 为例：

1. Client Panel 提交修改请求、当前 mapping revision 和 expected config version；
2. Server Panel 校验用户、设备、映射归属；
3. 在事务中插入新端口 7000 的 pending 租约；
4. 旧端口 6000 的 active 租约继续保留；
5. 保存 pending mapping revision；
6. 增加配置版本并提交；
7. Client Panel 拉取包含新端口的完整配置；
8. 校验、替换并 reload/restart FRPC；
9. FRPS 插件确认新代理只使用已授权的 7000；
10. Client 上报新版本成功；
11. Server Panel 在新事务中把新租约转 active；
12. 删除旧端口 6000 租约；
13. 提交新 mapping revision。

### 22.1 失败处理

若步骤 8～10 失败：

- Client 回滚旧配置；
- Server 保留旧端口 6000；
- Server 删除新端口 7000 的 pending 租约；
- mapping 继续指向旧 revision；
- 记录 operation failure；
- 显示具体错误；
- 不让用户同时失去旧端口。

### 22.2 Client 离线

修改操作可以保持 `pending_apply`，新旧端口同时占用，直到：

- Client 上线并成功应用；
- 用户取消修改；
- 管理员显式取消 pending operation。

默认不自动抢回新端口，避免出现状态不确定。

---

## 23. 删除映射完整流程

### 23.1 正常删除

```text
用户发起删除
    ↓
Server标记 mapping = deleting
    ↓
增加配置版本
    ↓
Client获取完整配置并删除本地代理
    ↓
frpc verify + reload/restart
    ↓
Client确认代理已不存在
    ↓
上报成功
    ↓
Server删除 mapping 和 port lease
    ↓
释放远程端口
```

### 23.2 Client 离线

用户或管理员可选择强制删除：

- Server 立即删除映射和端口租约；
- 增加配置版本；
- 写入删除 tombstone；
- Client 下次上线必须先拉取完整配置；
- Client 禁止把本地旧配置作为服务端事实回推；
- FRPS NewProxy 插件拒绝旧 mapping_id 和旧 revision。

删除 tombstone 保留一段时间，防止非常旧的 Client 重连后重建已删除代理。

---

## 24. FRPS 二次鉴权

Client Panel 不是安全边界。用户可能绕过它手工运行 FRPC，因此 FRPS 必须在服务端再次验证。

### 24.1 FRP 原生认证限制

FRP 原生 token 模式本质上是 frps 与 frpc 共享 Token，不等同于独立的多用户授权系统。平台不能只依赖 `user` 字段和一个全局 Token。

v2 推荐：

- FRP 原生认证作为传输层第一道门；
- FRPS HTTP Plugin 作为用户、设备、映射、端口和域名的最终授权门；
- 每个 Client 使用独立的 `frp_device_token` 元数据；
- 每个代理携带 `mapping_id`、revision、config_version 元数据。

未来可切换到 Server Panel 提供的 OIDC Client Credentials，以去除共享传输 Token，但这不是 v2 MVP 的前置条件。

### 24.2 Login 校验

检查：

- FRP 用户是否存在；
- 用户是否 active；
- `client_id` 是否存在且归属于该用户；
- 设备是否 active；
- `frp_device_token` 哈希是否匹配；
- Token 版本是否有效；
- Client Panel / FRPC / protocol 版本是否允许；
- 配置版本是否未被明确撤销；
- 请求来源和时间戳是否合理。

### 24.3 NewProxy 校验

检查：

- `proxy_name` 是否与 mapping_id 对应；
- mapping 是否属于当前用户和 client_id；
- mapping 是否 enabled；
- mapping 是否未删除；
- mapping revision 是否匹配；
- TCP/UDP remote_port 是否与数据库租约一致；
- HTTP customDomains 是否与 domain binding 一致；
- 不允许请求额外域名、端口或代理类型；
- 不允许用户通过手工配置增加未登记代理。

### 24.4 NewWorkConn / NewUserConn / Ping

- 用户停用后拒绝新的工作连接；
- 设备撤销或 Token 重置后拒绝新连接；
- 已删除代理拒绝新用户连接；
- Ping 可用于尽快发现并拒绝已失效客户端。

### 24.5 已建立连接的断开边界

FRPS HTTP Plugin 是操作前鉴权接口，不天然提供通用“按 run_id 主动杀掉所有已有连接”的标准控制能力。因此文档必须区分：

- **必须实现**：WebSocket 通知受管 Client 立即停止 FRPC；拒绝 Ping、NewWorkConn、NewUserConn、NewProxy；旧 Token 不能建立新连接。
- **尽快收敛**：通过 FRP 心跳/连接超时关闭控制连接。
- **严格即时断开可选增强**：维护一个小型 FRPS 扩展，提供受保护的按 run_id 断开接口。

禁止为了单个用户停用而重启整个 FRPS，除非是紧急管理员操作。

---

## 25. 域名标准化和唯一占用

### 25.1 数据库约束

```sql
CREATE UNIQUE INDEX ux_domain_bindings_normalized_domain
ON domain_bindings(normalized_domain);
```

完整域名在全平台只能属于一个用户。

### 25.2 标准化

输入域名必须：

- 去除首尾空白；
- 转小写；
- 去除末尾句点；
- 使用 IDNA/Punycode 转换；
- 校验每个 DNS label；
- 拒绝通配符，除非未来明确实现；
- 拒绝 IP 字面量；
- 拒绝与管理员面板域名、内部保留域名冲突。

### 25.3 占用规则

以下状态仍占用：

- Client 离线；
- 用户停用；
- DNS 错误；
- 证书错误；
- Token 缺失；
- pending_certificate；
- domain binding disabled。

只有删除 domain binding 后释放。

管理员不能直接覆盖其他用户域名，必须执行显式解绑或转移，并写审计日志。

---

## 26. Cloudflare Zone 提取

不能使用模糊的“最长公共后缀”。正式规则：

> 对标准化完整 hostname 与 Token 可访问的全部 Zone 执行最长合法 DNS 标签后缀匹配。

匹配仅允许：

```text
hostname == zone
```

或：

```text
hostname 以 "." + zone 结尾
```

处理步骤：

1. 调用 Cloudflare Zones API；
2. 获取所有分页，直到没有下一页；
3. 对 Zone 名称执行同样的小写、末尾点移除和 IDNA 规范化；
4. 过滤不满足 DNS 标签边界的字符串后缀；
5. 选择标签数量最多的 Zone；
6. 保存 `zone_id`；
7. 没有匹配 Zone 时拒绝创建并提示权限问题。

示例：

- hostname：`api.dev.example.com`
- Token 可访问：`example.com`、`dev.example.com`
- 应匹配：`dev.example.com`

---

## 27. Cloudflare Token 上传、替换与权限验证

### 27.1 上传传输与存储

```text
浏览器 -> Client Panel -> HTTPS -> Server Panel
```

Client Panel：

- 不持久化 Token；
- 不记录请求体；
- 转发完成后清理内存引用；
- 不向浏览器回显 Token。

Server Panel 使用 AES-256-GCM 保存，API 只返回 `has_token`、状态和能力，不返回明文。

### 27.2 权限验证

上传后必须检查：

- Token 有效性；
- Zone Read；
- DNS Read；
- DNS Write；
- 全部分页 Zone；
- 当前用户所有已绑定域名是否仍可管理；
- 如果产品要修改 Zone SSL 模式，再检查 Zone Settings Write。

状态：

```text
missing
valid
invalid
permission_denied
```

权限不足必须返回缺失能力列表，而不是统一“Token 错误”。

### 27.3 新 Token 替换流程

禁止直接覆盖 active Token：

```text
上传新 Token
 -> 加密保存为 pending token_version
 -> 验证有效性和能力
 -> 获取全部 Zone
 -> 检查现有域名可管理性
 -> 展示将失去权限的域名和后台任务
 -> 用户确认
 -> 原子切换 active_token_version
 -> 新任务使用新 Token
 -> 观察成功
 -> 安全删除或退休旧 Token
```

失败或用户取消时继续使用旧 Token。

### 27.4 任务 Token 版本

所有 Cloudflare 和 ACME job 创建时记录 `token_version`。执行前和关键步骤后重新检查：

- 该版本是否仍 active；
- Token 是否被清除或撤销；
- 能力是否仍满足。

版本不匹配时停止任务，防止旧 Token 在替换后继续写入。

### 27.5 Zone 提取

对标准化完整域名与 Token 可访问 Zone 执行最长合法 DNS 标签后缀匹配：

```text
hostname == zone
或
hostname 以 "." + zone 结尾
```

必须处理分页、小写、末尾句点、IDNA/Punycode 和格式校验。


## 28. DNS 数据模型与功能范围

至少支持：

- A；
- AAAA；
- CNAME；
- TTL；
- proxied（小橙云）；
- 同步状态；
- 一键更新；
- 删除同步；
- Cloudflare `zone_id` 和 `record_id`；
- 面板创建与面板接管区分。

建议字段：

```text
id
user_id
domain_binding_id
type
name
normalized_name
content
ttl
proxied
zone_id
record_id
managed_by_panel
adopted
locked
sync_status
last_error_code
last_error_message
last_synced_at
created_at
updated_at
```

### 28.1 一键更新服务器 IP

- Server Panel 系统设置保存权威公网 IPv4/IPv6；
- 可选自动检测，但检测结果必须由管理员确认或通过多源一致性校验；
- 一键更新只处理面板中选中的记录；
- A 更新 IPv4，AAAA 更新 IPv6；
- CNAME 更新为配置的服务端 canonical hostname；
- `locked=true` 的记录禁止批量修改；
- adopted 记录更新前再次确认。

### 28.2 同步

“同步”执行：

- 拉取 Cloudflare 当前记录；
- 比较 type/name/content/ttl/proxied；
- 展示 drift；
- 用户选择以面板覆盖 Cloudflare，或以 Cloudflare 更新面板；
- 任何覆盖写审计日志。

---

## 29. DNS 冲突与域名创建完整流程

### 29.1 当前用户面板已存在该域名

不得再次添加，提供：

- 查看；
- 修改；
- DNS 同步；
- 证书管理；
- 删除；
- 取消。

### 29.2 Cloudflare 已有 DNS、面板不存在

提供：

1. 取消；
2. 仅添加到面板，不修改 DNS；
3. 覆盖 DNS 并添加。

“仅添加”必须：

- 保存 `zone_id` 和 `record_id`；
- `adopted=true`；
- `managed_by_panel=false` 或按接管确认策略设置；
- 不修改 Cloudflare；
- 如果未指向当前服务端，显示无法正常访问警告。

“覆盖”必须先保存原记录快照用于审计和失败补偿。

### 29.3 完整域名创建 operation

```text
用户填写域名、本地 IP 和端口
 -> Client 检查本地 HTTP 服务
 -> Server 标准化域名
 -> 数据库检查 UNIQUE(normalized_domain)
 -> 提取 Zone 并检查 Token 权限
 -> 查询已有 DNS
 -> 用户选择取消、接管或覆盖
 -> 数据库保留 Domain Binding
 -> 执行 DNS 操作
 -> 建立或更新 FRP HTTP Mapping
 -> 增加 Client 配置版本
 -> Client 验证签名并应用配置
 -> 确认 FRPS HTTP proxy
 -> 申请或加载证书
 -> 生成 Router 快照
 -> Router 校验并 ACK
 -> Domain 状态 active
```

每一步写入 operation step，不能只依赖一个 `domain.status`。

### 29.4 失败分支

必须分别处理：

- DNS 成功但 Client 应用失败；
- Client 应用成功但证书失败；
- 证书成功但 Router 失败；
- Cloudflare 超时但实际成功；
- Client 离线；
- Token 在任务中被替换或清除；
- Router 快照 ACK 超时；
- 用户取消或管理员强制完成。

外部 API 超时后先通过 `record_id`、名称和内容查询实际状态，再决定重试，避免重复创建。

### 29.5 域名占用

`UNIQUE(normalized_domain)`。Client 离线、用户停用、DNS 失败、证书失败和 Router 失败时仍占用，只有正式删除 Domain Binding 后释放。


## 30. 删除域名、DNS 清理与补偿

删除使用 operation 阶段：

```text
preparing
revoking_access
removing_external_resources
removing_local_resources
completed
```

流程：

1. 标记 Domain 为 `deleting`；
2. 从 Router 新快照移除路由并等待 ACK；
3. 增加 Client 配置版本，移除 FRPC `customDomains`；
4. Client 应用并上报；
5. 删除面板管理的 Cloudflare DNS；
6. 删除证书和私钥；
7. 删除 Domain Binding；
8. 释放唯一占用。

规则：

- `preparing` 且未修改外部资源时可直接取消；
- DNS、证书或路由已删除后不能只把数据库状态改回 active；
- 取消必须执行补偿：重建 DNS、证书和 Router；
- 无法完全恢复时不得显示“取消成功”；
- 进入不可逆阶段后只能重试或强制完成；
- adopted 记录默认不自动删除，除非用户明确授权；
- 外部 DNS 清理失败支持重试、取消（仅可逆阶段）或强制删除本地数据；
- 强制删除必须留下未清理外部资源清单和高风险审计。


## 31. HTTPS 总体架构

所有用户域名由 Server Router 统一终止 TLS：

```text
浏览器
    ↓
Cloudflare（可选）
    ↓
Server Router :80/:443
    ↓ TLS终止
普通 HTTP
    ↓
FRPS vhostHTTPPort 127.0.0.1:8080
    ↓
FRPC HTTP proxy
    ↓
用户本地 HTTP 服务
```

不让 FRPS 管理每个用户证书，不把浏览器 TLS 原样透传到普通 HTTP 服务。

---

## 32. 三种 HTTPS 模式与合法组合

| 模式 | Cloudflare `proxied` | 源站证书 | 行为 |
|---|---:|---|---|
| `auto_certificate` | `false` | Let's Encrypt / ZeroSSL 公共证书 | 浏览器直连源站也可信 |
| `cloudflare_proxy` | `true` | 有效源站证书 | Cloudflare 到源站使用 HTTPS，推荐 Full (strict) |
| `http_only` | `false` | 无 | 该域名仅 HTTP |

禁止组合：

```text
http_only + proxied=true
cloudflare_proxy + proxied=false
auto_certificate + proxied=true
```

切换到 Cloudflare 代理：

```text
先确认证书有效
 -> 开启小橙云
 -> 查询确认 DNS 状态
 -> 标记模式 active
```

关闭小橙云：

- 默认源站继续使用公共 CA 证书，因此可直接访问；
- 如果该域名使用 Origin CA，不得直接关闭；
- 必须先申请普通浏览器信任的公共证书，再关闭代理。

默认优先公共 CA 证书。Origin CA 只允许用于明确保证始终经过 Cloudflare 的域名，不能作为普通默认推荐。


## 33. Server Router 行为

### 33.1 Host 与 SNI

- 域名匹配必须使用标准化后的精确 Host；
- TLS 证书通过 SNI 精确选择；
- 不允许未知域名回退到其他用户证书；
- 管理员面板域名与用户业务域名使用独立路由空间和证书；
- 未绑定 Host 返回 404；
- 未知 SNI 返回 TLS unrecognized_name 或受控默认行为。

### 33.2 反向代理头

Router 必须：

- 保留原始 Host；
- 删除客户端伪造的 `Forwarded`、`X-Forwarded-*`；
- 重新设置可信的 `X-Forwarded-For`；
- 设置 `X-Forwarded-Proto`；
- 可设置 `X-Forwarded-Host`；
- 删除 hop-by-hop headers；
- 对 Cloudflare 来源仅在请求源 IP 属于官方 Cloudflare 网段时信任 `CF-Connecting-IP`。

### 33.3 协议能力

必须支持：

- WebSocket Upgrade；
- 流式响应；
- SSE；
- 大文件转发；
- 可配置连接、响应头、空闲超时；
- 正确取消客户端断开的上游请求。

不得默认缓冲整个响应体。

### 33.4 HTTP 跳转

- 按域名单独配置；
- 自动证书或代理模式可开启 301/308；
- 仅 HTTP 模式不能强制跳转；
- 防止 Cloudflare Flexible 等错误设置造成重定向循环；
- 产品默认不推荐 Flexible。

### 33.5 离线页面

FRPC/本地服务不可用时：

- 返回 502；
- 显示统一“隧道离线”页面；
- 不泄露 Client IP、端口、内部错误或用户信息；
- 对 API 请求可返回简洁 JSON；
- 可携带 request_id 供排查。

---

## 34. 证书存储和热加载

建议目录：

```text
/data/certificates/<normalized-domain>/
├── cert.pem
├── chain.pem
├── fullchain.pem
├── private-key.enc
└── metadata.json
```

### 34.1 私钥保护

- 证书公钥链可以明文保存；
- 私钥优先使用 Server 主密钥进行 AES-256-GCM 加密；
- Router 启动或收到更新时解密到内存中的 `tls.Certificate`；
- 不长期落地明文私钥；
- 若 ACME 库必须使用临时明文文件，使用 `0600`、随机目录，操作结束立即删除；
- 内存缓存按域名索引并可原子替换。

### 34.2 热加载

- Router 使用 `tls.Config.GetCertificate`；
- 控制面完成新证书验证后发布新版本；
- Router 构造新证书对象并原子交换；
- 已建立连接不受影响；
- 新连接立即使用新证书；
- 加载失败继续使用旧有效证书并报警。

### 34.3 证书验证

加载前检查：

- 私钥与证书匹配；
- SAN 包含目标域名；
- 证书链可解析；
- notBefore/notAfter 合理；
- 文件和数据库哈希一致。

---

## 35. ACME 自动化

### 35.1 环境和账户

- 支持 Staging 与 Production，默认先在 Staging 验证部署；
- 保存 ACME 账户、Provider、邮箱和注册信息；
- 切换 Production 必须显式配置；
- Let's Encrypt 和 ZeroSSL 作为可配置 Provider，不并行重复申请。

### 35.2 DNS-01 流程

```text
获取域名单任务锁
 -> 校验 active Cloudflare token_version
 -> 创建 TXT
 -> 权威 DNS 传播检测
 -> ACME challenge
 -> 无论成功失败都清理 TXT
 -> 保存证书
 -> 原子替换文件/密文
 -> Router 热加载
```

TXT 清理失败必须进入独立重试 job。

### 35.3 锁、重试和限流

- 同一域名同一时间只能有一个证书任务；
- 手动续期 60 秒冷却由服务端强制，不依赖前端；
- 失败采用指数退避和随机抖动；
- 遵守 CA 和 Cloudflare `Retry-After`；
- 区分可重试、权限错误、配额错误和永久域名错误；
- Token 被清除、失效或权限不足时暂停续期；
- 到期前 30 天进入自动续期窗口；
- 每日检查，不需要每小时高频轮询。

### 35.4 文件和私钥

- 证书和私钥写入临时文件后 `fsync` 并原子替换；
- 私钥使用专用 `certificate_wrapping_key` 加密或存于严格权限文件；
- 文件权限仅服务进程可读；
- Router 使用 `GetCertificate` 或等价内存表热加载；
- 新证书加载失败时继续使用旧证书；
- 不能让一个无证书域名影响其他域名监听 443。

### 35.5 状态

```text
pending
valid
renewing
expired
blocked_missing_token
blocked_invalid_token
error
```

错误信息脱敏，不包含 Token、TXT API 请求体或私钥。


## 36. Cloudflare Token 清除

按钮必须明确命名为：

```text
清除 Cloudflare Token
```

流程：

1. Client Panel 显示红色确认框；
2. 前端倒计时 3 秒；
3. 用户确认；
4. Server Panel 再次验证当前 Session 和 CSRF；
5. 增加 Token version；
6. 清除 ciphertext、nonce、状态和 Zone cache；
7. 取消尚未开始的 Cloudflare 任务；
8. 运行中任务检查 Token version 后停止；
9. 证书自动续期进入暂停状态；
10. 返回 `has_token=false`。

提示必须说明：

- 已有 Cloudflare DNS 不会自动消失；
- 已有证书不会立即删除；
- 后续 DNS 操作和证书续期将失败或暂停。

---

## 37. FRP Token / FRP 设备凭证重置

按钮明确命名为：

```text
重置 FRP Token
```

用户级 Token 重置：

1. 生成新高强度 Token；
2. 增加 Token version；
3. 旧 Token 立即不能建立新 Login/NewProxy/NewWorkConn；
4. 增加相关 Client 的配置版本；
5. WebSocket 通知 Client；
6. Client 拉取新凭证；
7. 因公共认证配置变化，Client 必须 restart FRPC；
8. 受管 Client 停止旧 FRPC 后启动新配置；
9. Server 尽快拒绝旧连接；
10. 写审计日志。

设备级凭证可单独重置，不影响同用户其他设备。

---

## 38. 敏感密钥生命周期与用途隔离

### 38.1 密钥集合

必须使用不同密钥：

```text
server_master_key
certificate_wrapping_key
router_snapshot_key
config_signing_key
device_hmac_key（由设备 Token 派生）
backup_encryption_key（由管理员备份密码经 KDF 派生）
```

用途不得交叉：

- `server_master_key`：Cloudflare Token、可恢复 FRP Token和设备派生签名密钥；
- `certificate_wrapping_key`：证书私钥；
- `router_snapshot_key`：Router 快照 HMAC；
- `config_signing_key`：Ed25519 Client 配置签名；
- `device_hmac_key`：单设备 API 请求签名；
- `backup_encryption_key`：整个备份归档。

Router 不得获得业务主密钥或设备密钥。

### 38.2 主密钥生命周期

- 优先从系统 Secret 或环境变量读取；
- 没有时只在首次启动生成一次并持久化到受保护文件；
- 不能每次启动重新生成；
- 主密钥不能写普通数据库和日志；
- 文件权限仅服务账户可读；
- 数据库密文保存 `ciphertext`、`nonce`、`key_version`；
- 每次加密使用新的随机 Nonce；
- AAD 绑定用户、资源 ID、数据类型和密钥版本；
- 解密失败必须安全失败，禁止静默清空。

### 38.3 轮换

轮换流程：

1. 创建新版本密钥；
2. 新写入使用新版本；
3. 后台分批重新封装旧密文；
4. 校验完成；
5. 退休旧密钥；
6. 审计整个过程。

Ed25519 公钥轮换必须通过当前受信任私钥签名新公钥声明，Client 验证后更新固定公钥。


## 39. 用户停用和删除

### 39.1 停用用户

立即执行：

- 设置 `disabled` 并增加 auth/status version；
- 禁止新登录；
- 撤销全部 Server Web Session和 Client 活动代理会话；
- 拒绝用户 API、设备 API 和 WebSocket；
- FRPS Login/NewProxy/NewWorkConn 拒绝；
- 通知受管 Client 停止 FRPC；
- 尽快断开已有 FRP 连接。

停用后端口和域名继续占用，DNS 默认不删除。证书自动续期默认暂停外部写操作并显示原因。

### 39.2 删除用户 operation

删除不是单个 HTTP 请求内的同步事务。顺序：

1. 停用用户；
2. 撤销 Session；
3. 撤销设备 HMAC 和 FRP 设备凭证；
4. 通知并断开 FRP；
5. 将资源标记 deleting；
6. 从 Router 移除路由；
7. 删除面板管理的 Cloudflare DNS；
8. 清理 ACME challenge；
9. 删除证书和私钥；
10. 删除 Domain Binding；
11. 删除端口租约和 Mapping；
12. 删除 FRP 凭证；
13. 删除 Cloudflare 凭证；
14. 匿名化或保留必要审计；
15. 删除用户主体。

同一用户同一时间只能存在一个删除 operation。

### 39.3 取消、重试和强制完成

删除阶段：

```text
preparing
revoking_access
removing_external_resources
removing_local_resources
completed
```

- `preparing` 且未修改外部资源时可直接取消并恢复为 disabled；
- 已撤销访问但外部资源未变更时，可按明确步骤恢复；
- DNS、证书或 Router 已删除后，取消必须执行补偿恢复；
- 补偿失败不得显示“取消成功”；
- 进入不可逆阶段后只允许重试或强制完成；
- 外部清理失败可重试；
- 强制完成必须展示并保存外部残留清单；
- 不得静默忽略 Cloudflare、证书或路由残留。


## 40. 数据库设计

Server Panel 默认 SQLite WAL，通过 migration 固化。所有敏感字段进入日志和序列化黑名单。

### 40.0 system_identity

```text
singleton_id PRIMARY KEY CHECK(singleton_id = 1)
server_instance_id UNIQUE
created_at
restored_from_backup_at
```

### 40.1 users

```text
id
username UNIQUE
password_hash
role
status
must_change_password
auth_version
max_clients
max_mappings
max_domains
max_pending_mappings
max_pending_port_leases
max_pending_domain_operations
max_certificate_jobs
created_at
updated_at
deleted_at
```

### 40.2 sessions

Server Panel 用户 Web Session：

```text
id
user_id
client_id NULL
installation_instance_id NULL
local_proxy_session_id
session_hash
csrf_secret_hash
auth_version
session_generation
login_channel
browser_source_ip
client_panel_source_ip
user_agent
expires_at
idle_expires_at
last_seen_at
revoked_at
revoke_reason
created_at
```

索引：

```sql
UNIQUE(client_id, local_proxy_session_id);
CREATE UNIQUE INDEX one_active_client_session
ON sessions(client_id)
WHERE client_id IS NOT NULL AND revoked_at IS NULL;
```

Server 看到的 Client 网络地址与 Client 上报的浏览器来源地址必须分别保存，不能混用。

### 40.3 clients

```text
id
server_instance_id
owner_user_id
installation_instance_id
name
status
binding_revision
session_generation
active_device_credential_version
frp_device_token_hash
frp_device_token_version
desired_config_version
applied_config_version
last_failed_config_version
last_error_code
last_error_message
client_panel_version
frpc_version
protocol_version
config_schema_version
last_seen_at
last_ip
registered_at
unbound_at
created_at
updated_at
UNIQUE(server_instance_id, installation_instance_id)
```

### 40.4 device_credentials

```text
id
client_id
token_version
device_token_hash
signing_key_ciphertext
signing_key_nonce
master_key_version
status
created_at
activated_at
revoked_at
UNIQUE(client_id, token_version)
```

### 40.5 device_request_nonces

```text
client_id
token_version
nonce_hash
request_timestamp
expires_at
created_at
UNIQUE(client_id, token_version, nonce_hash)
```

### 40.6 idempotency_records

```text
id
actor_type
actor_id
client_id
http_method
normalized_path
idempotency_key
request_body_hash
response_status
response_body_json
operation_id
expires_at
created_at
UNIQUE(actor_type, actor_id, http_method, normalized_path, idempotency_key)
```

### 40.7 frp_credentials

```text
id
user_id
frp_username
manual_token_hash
manual_token_ciphertext
manual_token_nonce
key_version
token_version
created_at
rotated_at
```

### 40.8 mappings

```text
id
user_id
client_id
name
proxy_type              # tcp / udp / http
local_ip
local_port
lifecycle_status
desired_state
observed_state
active_revision
pending_revision
created_at
updated_at
```

### 40.9 mapping_revisions

```text
id
mapping_id
revision
remote_port NULL        # tcp/udp 使用
config_json
status
created_at
applied_at
UNIQUE(mapping_id, revision)
```

禁止 `custom_domain` 字段。HTTP 域名通过 `domain_bindings.mapping_id` 关联。

### 40.10 port_leases

```text
id
server_id
mapping_id
mapping_revision_id
remote_port
lease_role              # active / pending
created_at
UNIQUE(server_id, remote_port)
```

协议不进入唯一键。

### 40.11 domain_bindings

```text
id
user_id
client_id
mapping_id
hostname
normalized_domain UNIQUE
zone_id
https_mode
http_redirect
status
revision
created_at
updated_at
```

### 40.12 dns_records

```text
id
user_id
domain_binding_id
type
name
normalized_name
content
ttl
proxied
zone_id
record_id
managed_by_panel
adopted
locked
sync_status
last_synced_at
last_error_code
last_error_message
```

### 40.13 cloudflare_credentials

支持 active/pending 版本：

```text
id
user_id
token_version
ciphertext
nonce
key_version
status                 # pending / active / retired / invalid
capabilities_json
verified_at
activated_at
retired_at
created_at
UNIQUE(user_id, token_version)
```

另在 users 或单独 state 表保存 `active_cloudflare_token_version`。

### 40.14 certificates

```text
id
domain_binding_id
provider
status
not_before
not_after
renew_after
cert_path
private_key_ciphertext
private_key_nonce
wrapping_key_version
cert_hash
last_error_code
last_error_message
updated_at
```

### 40.15 config_snapshots

```text
id
client_id
version
schema_version
config_json
config_hash
config_signing_key_id
config_signature
created_at
UNIQUE(client_id, version)
```

### 40.16 config_signing_keys

```text
key_id PRIMARY KEY
public_key
private_key_ciphertext
private_key_nonce
status
not_before
not_after
created_at
retired_at
```

### 40.17 router_snapshots

```text
version PRIMARY KEY
schema_version
snapshot_path
snapshot_hash
snapshot_hmac
status
generated_at
applied_at
last_error
```

### 40.18 router_state

```text
singleton_id PRIMARY KEY CHECK(singleton_id = 1)
router_config_version
router_applied_version
last_good_snapshot_version
last_good_snapshot_path
last_good_snapshot_hash
last_router_apply_error
updated_at
```

### 40.19 operations

```text
id
user_id
client_id
resource_type
resource_id
operation_type
status
phase
step
idempotency_key
cancelable
compensation_status
error_code
error_message
created_at
updated_at
completed_at
```

### 40.20 jobs

```text
id
type
resource_type
resource_id
status
run_after
attempts
max_attempts
lock_owner
locked_at
lock_expires_at
heartbeat_at
deduplication_key
token_version NULL
last_error
payload_json
created_at
updated_at
completed_at
UNIQUE(type, deduplication_key)
```

### 40.21 audit_logs

```text
id
actor_type
actor_id
server_session_id NULL
client_id NULL
local_proxy_session_id NULL
browser_source_ip NULL
client_panel_source_ip NULL
user_agent
request_id
operation_id
action
resource_type
resource_id
result
metadata_json
created_at
```

metadata 使用字段白名单。

### 40.22 Client 本地数据库

只保存安装实例和 FRPC 管理状态：

```text
client_installation
client_config_state
frpc_runtime_state
server_binding_aliases
```

活动浏览器 Session 只在内存，不进入本地数据库。


## 41. 后台任务系统

后台任务用于 Cloudflare、ACME、Router、删除、补偿和长期同步。

### 41.1 租约

Worker 获取任务时原子设置：

```text
lock_owner
locked_at
lock_expires_at
heartbeat_at
```

- 长任务定期续租；
- Worker 崩溃后锁过期可由其他 Worker 接管；
- 完成或失败时清除租约；
- 不依赖永久 `locked=true`。

### 41.2 去重

- 同一域名同时只能有一个 ACME job；
- 同一 DNS 记录不能同时创建、修改、删除；
- 同一用户同时只能有一个删除 operation；
- Token 替换、Router 快照和配置生成使用稳定 `deduplication_key`；
- 重复请求返回现有 operation。

### 41.3 SQLite 事务边界

禁止在 Cloudflare、ACME、文件系统、Router IPC 或 Client 网络调用期间持有 SQLite 写事务。

正确模式：

1. 短事务领取任务并记录意图；
2. 释放事务；
3. 调用外部系统；
4. 短事务记录结果；
5. 根据结果排队下一步。

### 41.4 重试

- 指数退避 + 随机抖动；
- 遵守 `Retry-After`；
- 明确可重试和永久错误；
- 每次重试前重新检查资源 revision、Token 版本和删除状态；
- 操作被取消或资源已变化时停止旧任务。


## 42. Server API 草案

所有公网 API 只经 HTTPS 暴露；设备 API 使用 HMAC，用户写操作使用 Server Session、CSRF、版本和幂等校验。

### 42.1 实例身份和认证

```text
GET  /api/v1/instance
POST /api/v1/auth/login
POST /api/v1/auth/logout
POST /api/v1/auth/reauth
GET  /api/v1/auth/session
```

登录成功后 Client Panel 抢占当前 `client_id` 的旧活动会话。旧请求返回 `SESSION_REPLACED`。

### 42.2 设备

```text
POST   /api/v1/devices/register
GET    /api/v1/devices/current
POST   /api/v1/devices/current/rotate
POST   /api/v1/devices/current/unbind
GET    /api/v1/devices
DELETE /api/v1/devices/{client_id}
```

设备注册与浏览器登录分离。

### 42.3 设备后台同步（HMAC）

```text
GET  /api/v1/client/bootstrap
GET  /api/v1/client/config
POST /api/v1/client/config/apply-result
POST /api/v1/client/heartbeat
POST /api/v1/client/status
GET  /api/v1/client/events
```

Canonical request 至少覆盖：

```text
client_id
token_version
timestamp
nonce
HTTP method
normalized path
canonical query
body SHA-256
```

时间窗口、Nonce 唯一插入和 Token 状态必须校验。

### 42.4 Mapping

```text
GET    /api/v1/mappings
POST   /api/v1/mappings
GET    /api/v1/mappings/{id}
PATCH  /api/v1/mappings/{id}
POST   /api/v1/mappings/{id}/enable
POST   /api/v1/mappings/{id}/disable
DELETE /api/v1/mappings/{id}
POST   /api/v1/mappings/{id}/force-delete
```

请求携带 `expected_config_version`、`mapping_revision` 和 `Idempotency-Key`。

### 42.5 Domain 和 DNS

```text
GET    /api/v1/domains
POST   /api/v1/domains/preflight
POST   /api/v1/domains
PATCH  /api/v1/domains/{id}
DELETE /api/v1/domains/{id}
POST   /api/v1/domains/{id}/sync
POST   /api/v1/domains/{id}/proxy
POST   /api/v1/domains/{id}/update-server-ip

GET    /api/v1/domains/{id}/dns
POST   /api/v1/domains/{id}/dns
PATCH  /api/v1/domains/{id}/dns/{record_id}
DELETE /api/v1/domains/{id}/dns/{record_id}
```

### 42.6 Cloudflare

```text
GET    /api/v1/cloudflare/token/status
POST   /api/v1/cloudflare/token/pending
POST   /api/v1/cloudflare/token/{version}/verify
POST   /api/v1/cloudflare/token/{version}/activate
DELETE /api/v1/cloudflare/token
GET    /api/v1/cloudflare/zones
```

上传、替换和清除是不同操作。

### 42.7 证书

```text
GET  /api/v1/certificates
POST /api/v1/certificates/{domain_id}/issue
POST /api/v1/certificates/{domain_id}/renew
GET  /api/v1/certificates/{domain_id}/status
```

手动续期冷却在服务端强制。

### 42.8 Operation

```text
GET  /api/v1/operations/{id}
POST /api/v1/operations/{id}/cancel
POST /api/v1/operations/{id}/retry
POST /api/v1/operations/{id}/force-complete
```

取消只在 operation 标记 `cancelable=true` 时允许。

### 42.9 管理员

```text
GET    /api/v1/admin/users
POST   /api/v1/admin/users
PATCH  /api/v1/admin/users/{id}
POST   /api/v1/admin/users/{id}/disable
POST   /api/v1/admin/users/{id}/enable
DELETE /api/v1/admin/users/{id}
POST   /api/v1/admin/users/{id}/force-delete
GET    /api/v1/admin/audit
GET    /api/v1/admin/system/status
POST   /api/v1/admin/backups
POST   /api/v1/admin/restores/preflight
POST   /api/v1/admin/restores
```

### 42.10 FRPS 内部插件

```text
POST /internal/frp/login
POST /internal/frp/new-proxy
POST /internal/frp/new-work-conn
POST /internal/frp/close-proxy
POST /internal/frp/ping
```

仅本机访问，不接受公网反代。


## 43. 状态定义

### 43.1 Client Panel 与绑定状态

运行状态：

```text
online
offline
disabled
outdated
config_pending
config_error
```

绑定状态：

```text
unbound
binding
bound
switching_server
credential_revoked
unbinding
```

浏览器 Session 状态：

```text
active
server_unreachable_readonly
expired
revoked
```

浏览器 Session 状态不改变 Client 设备在线状态；全部浏览器退出后，设备仍可通过 HMAC 进行后台心跳和配置同步。


### 43.2 Cloudflare Token

```text
missing
valid
invalid
permission_denied
```

### 43.3 域名

```text
reserved
pending_dns
pending_certificate
active
offline
dns_error
certificate_error
disabled
deleting
```

### 43.4 证书

```text
pending
valid
renewing
expired
blocked_missing_token
blocked_invalid_token
error
```

### 43.5 operation

```text
pending
running
waiting_client
waiting_external
succeeded
failed
cancelled
```

每个状态必须定义允许转换，不能由任意接口直接赋值。

---

## 44. 审计日志

所有重要操作记录：

- 用户登录、抢占旧会话、退出；
- 管理员创建、停用、启用和删除用户；
- 设备注册、轮换、撤销、切换服务端和解除；
- Mapping 创建、修改、删除、强制删除；
- Domain、DNS、小橙云和证书操作；
- Cloudflare Token 上传、验证、替换和清除；
- FRP Token 查看与重置；
- Router 快照生成和应用；
- 数据备份与恢复。

字段至少包含：

```text
user_id / actor_id
server_session_id
client_id
local_proxy_session_id
browser_source_ip
client_panel_source_ip
user_agent
request_id
operation_id
action
resource_type
resource_id
result
created_at
```

必须区分：

- 浏览器来源 IP：Client Panel 观察并上报；
- Client Panel 来源 IP：Server TCP 层实际观察；
- 两者都不应被当作绝对可信身份，只用于审计。

日志禁止记录密码、Cookie、Session Token、CSRF Secret、device_token、设备签名密钥、FRP Token、Cloudflare Token和证书私钥。

会话抢占必须记录旧 Session ID、新 Session ID、client_id、来源地址和结果，但不记录任何秘密。


## 45. 日志和可观测性

### 45.1 日志

- Server 与 Client 均结构化；
- 支持级别和轮转；
- 默认保留有限天数；
- 错误日志包含 request_id、operation_id、mapping_id；
- 不包含敏感数据；
- Client 用户可查看 FRPC 脱敏日志。

### 45.2 指标

建议：

- 在线 Client 数；
- FRP 登录接受/拒绝次数；
- NewProxy 拒绝原因；
- 映射状态数；
- Router 2xx/4xx/5xx；
- 502 数；
- Cloudflare API 错误；
- ACME 任务状态；
- SQLite busy 次数；
- WAL 大小和 checkpoint；
- 后台任务积压。

FRPS Dashboard API 官方标记为不规范，不作为唯一业务事实来源；可以使用 Prometheus 和插件事件作为辅助观察。

---

## 46. 正式备份和恢复

### 46.1 格式定稿

正式备份使用加密归档包，例如：

```text
backup-YYYYMMDD-HHMM.frppanel-backup
```

JSON 仅用于非敏感配置预览、排错和迁移辅助，不作为完整恢复格式。

### 46.2 备份内容

- SQLite 一致性快照；
- migration 版本；
- 加密后的 Cloudflare Token；
- 加密后的可恢复 FRP Token；
- 主密钥；
- 证书和加密私钥；
- Server 设置；
- Router 配置；
- FRPS 配置模板；
- 版本清单；
- 每个文件校验和；
- 备份 manifest。

### 46.3 整包加密

- 管理员输入备份密码；
- 使用成熟的 passphrase 加密容器，例如 age scrypt；
- 不自创加密格式；
- 备份密码不保存；
- 输出前完成校验；
- 下载接口使用一次性授权和短时有效 URL；
- 写审计日志。

把主密钥放入备份包是为了恢复已有密文，但主密钥只存在于整包加密层内。

### 46.4 SQLite 快照

- 使用 SQLite Backup API 或受控 checkpoint 后快照；
- 不能只复制 `.db` 而忽略 WAL；
- 备份过程不长时间阻塞业务写入；
- 备份后执行完整性检查。

### 46.5 恢复流程

1. 上传到隔离目录；
2. 验证归档格式、密码和校验和；
3. 检查版本兼容性；
4. 进入维护模式；
5. 创建当前系统安全快照；
6. 停止后台写任务和 Router 配置更新；
7. 恢复数据库、主密钥、证书和设置；
8. 执行 migration；
9. 完整性检查；
10. 启动服务；
11. 让全部 Client 执行版本对比；
12. 记录恢复审计。

恢复失败必须自动回到恢复前快照。

---

## 47. 数据导入导出规则

### 47.1 JSON 导出

只允许导出：

- 非敏感用户信息；
- 映射和域名配置；
- 状态；
- 配额；
- 不包含秘密的系统设置。

必须排除：

- Cloudflare Token 明文；
- FRP Token 明文；
- device_token；
- Session；
- 主密钥；
- 证书私钥。

即使 JSON 中保留密文，也不得宣称它是可独立恢复的完整备份，因为缺少主密钥将无法解密。

### 47.2 JSON 导入

- 仅用于配置迁移；
- Schema 严格校验；
- 提供 dry-run；
- 检查端口和域名冲突；
- 不覆盖现有用户秘密；
- 不允许导入未知字段触发命令或路径行为；
- 导入操作写审计日志。

---

## 48. 版本与协议兼容

Client 心跳和注册必须携带：

```text
client_panel_version
frpc_version
protocol_version
config_schema_version
os
arch
```

Server 返回：

```text
minimum_client_version
latest_client_version
minimum_frpc_version
recommended_frpc_version
protocol_version
supported_config_schema_versions
upgrade_policy
```

### 48.1 兼容规则

- 新字段默认可选，并定义默认值；
- 删除字段必须经过弃用周期；
- 协议大版本不兼容时拒绝配置变更；
- 安全漏洞版本可强制禁止登录或 NewProxy；
- 只过旧但仍安全的版本显示提醒；
- Server 与 Client 分开发布，不能依赖共享源码或同进程调用；
- 所有通信都通过版本化 HTTPS API/WSS。

### 48.2 配置 Schema

- 完整配置包含 `schema_version`；
- Client 只接受已声明支持的版本；
- 不支持时不覆盖当前有效配置；
- 上报 `UNSUPPORTED_CONFIG_SCHEMA`；
- Server 可生成旧 Schema 快照以兼容过渡期客户端。

---

## 49. 安全威胁与防护摘要

| 威胁 | 防护 |
|---|---|
| 局域网设备接管 Client Panel | loopback 默认、指定监听接口、IP/CIDR 白名单、本地 HTTPS、Host/Origin/CSRF 校验、来源限流 |
| 新浏览器登录后旧会话仍有效 | 单一 `active_proxy_session`、Session generation、先撤销旧 Server Session、旧 HTTP/WSS 返回 `SESSION_REPLACED` |
| 抢占流程异常产生两个活动 Session | Client 互斥锁、Server 部分唯一索引、补偿撤销新/旧 Session、原子代际切换 |
| 旧页面覆盖新配置 | `expected_config_version`、资源 revision、409 冲突、幂等键 |
| 浏览器登录误注册 Client | 登录与设备注册分离、安装实例唯一约束、注册幂等 |
| 其他用户接管设备 | `owner_user_id`、`CLIENT_OWNER_MISMATCH`、显式解绑后重注册 |
| 恶意切换 Server Panel | `server_instance_id` 核对、TLS 重新验证、危险确认、撤销旧设备 |
| IP 地址 TLS 被中间人攻击 | IP SAN、自定义 CA 或人工核对 SPKI；确认前禁止发送密码；无跳过验证 |
| Server 断线绕过授权 | 只读白名单、修改和 FRPC Web 控制全部在线验证、无离线管理员 |
| 用户手工伪造 FRPC | FRPS Login/NewProxy/NewWorkConn 二次鉴权、凭证版本和数据库资源校验 |
| 端口并发抢占 | `UNIQUE(server_id, remote_port)`、短事务插入、active/pending 租约 |
| TCP/UDP 跨协议重复端口 | 协议不进入唯一键 |
| 域名抢占 | IDNA 标准化、`UNIQUE(normalized_domain)` |
| Client 配置被篡改 | HTTPS、设备 HMAC、SHA-256、Ed25519 配置签名、版本校验 |
| Router 读取到坏配置 | 只读版本快照、HMAC、Schema/哈希校验、last-good 回滚 |
| Token 日志泄漏 | 请求体不记录、字段过滤、日志白名单、Authorization/Cookie 黑名单 |
| 主密钥重启丢失 | 首次生成后持久化、版本管理、受保护文件、加密备份 |
| 密钥用途串用 | 主密钥、证书封装、Router HMAC、配置签名、设备 HMAC 分离 |
| FRPC 配置损坏 | verify、备份、原子替换、单队列、失败回滚 |
| WebSocket 丢事件 | 配置版本 + 全量同步 |
| 旧 Client 重建已删除代理 | tombstone/revision、全量同步、FRPS NewProxy 拒绝 |
| 错误 TLS 证书串域 | SNI 精确匹配、未知 SNI 不回退、管理员域名隔离 |
| Cloudflare Token 替换破坏现有域名 | pending 验证、能力差异展示、确认后切换、旧版本退休 |
| ACME 限流 | Staging、域名任务锁、服务端冷却、退避、Retry-After |
| Worker 崩溃造成永久锁 | 任务租约、心跳、锁过期接管 |
| SQLite 长事务阻塞 | 外部调用不持写事务、短事务、WAL、checkpoint |
| Pending 资源滥用 | 用户配额、管理员治理、长期 Pending 提醒和取消审计 |
| 删除取消造成假恢复 | 可逆阶段取消、不可逆阶段补偿、失败时不显示成功 |
| 备份泄露 | 整包强加密、KDF、校验和、短时下载授权 |


## 50. 开发目录建议

### 50.1 Server Panel

```text
server-panel/
├── cmd/
│   ├── control/
│   └── router/
├── internal/
│   ├── api/
│   ├── auth/
│   ├── session/
│   ├── users/
│   ├── devices/
│   ├── mappings/
│   ├── domains/
│   ├── dns/
│   ├── cloudflare/
│   ├── certificates/
│   ├── routerconfig/
│   ├── frpauth/
│   ├── configsync/
│   ├── signing/
│   ├── jobs/
│   ├── operations/
│   ├── backup/
│   ├── audit/
│   ├── crypto/
│   └── storage/
├── migrations/
├── web-admin/
├── web-user-shared/
├── packaging/
└── tests/
```

### 50.2 Client Panel

```text
client-panel/
├── cmd/client/
├── internal/
│   ├── localapi/
│   ├── active_session/
│   ├── sessionproxy/
│   ├── serverbinding/
│   ├── hmacsigner/
│   ├── serverclient/
│   ├── websocket/
│   ├── sync/
│   ├── configrender/
│   ├── configverify/
│   ├── configapply/
│   ├── frpc/
│   ├── supervisor/
│   ├── health/
│   ├── securestore/
│   ├── logs/
│   ├── updates/
│   └── storage/
├── web-client/
├── embedded/frpc/
├── packaging/
└── tests/
```

### 50.3 协议共享

Server 与 Client 可共享独立版本化协议包，仅包含：

- OpenAPI/JSON Schema；
- WebSocket 事件 Schema；
- 错误码；
- Canonical HMAC 规范；
- 配置 Canonicalization 规范；
- 生成客户端代码。

不得直接引用对方业务实现、数据库模型或内部服务。


## 51. 开发阶段和验收标准

### 阶段 0：技术验证

完成：

- FRPS Plugin 的 Login/NewProxy/NewWorkConn 拒绝；
- TCP 与 UDP 代理验证；
- FRPC verify/reload/restart/status；
- Router -> FRPS vhostHTTPPort -> FRPC；
- SQLite WAL 并发唯一约束；
- Cloudflare Zone/DNS；
- ACME DNS-01 Staging；
- HMAC Canonical Request、Nonce 和时钟偏差；
- Ed25519 配置签名和 Client 固定公钥；
- Router 快照 HMAC、IPC、last-good；
- IP SAN、自定义 CA 和 SPKI 固定。

验收：手工 FRPC 不能建立未授权端口、协议或域名。

### 阶段 1：身份、安全和单活动会话

- 首次管理员随机密码；
- 用户、Server Session、设备；
- Client 无本地用户体系；
- Active Browser Session Manager；
- 新登录抢占旧登录；
- Server 部分唯一会话索引；
- Cookie、CSRF、reauth ticket；
- 设备 HMAC、Nonce 和幂等；
- owner mismatch；
- Server 实例身份与切换；
- 主密钥和用途隔离；
- 审计与 migration。

验收：任何 `client_id` 不得同时存在两个有效代理会话；旧浏览器 HTTP 和 WSS 均失效。

### 阶段 2：FRPC 管理闭环

- 内置固定 FRPC；
- 配置渲染和 Ed25519 验证；
- verify；
- 备份和原子替换；
- reload/restart 分类；
- 回滚；
- 单一 Supervisor 和配置 Worker；
- 全量版本同步；
- 状态上报。

验收：错误配置、签名错误或版本倒退均不得覆盖旧配置。

### 阶段 3：TCP/UDP 端口映射

- 手动和自动端口；
- TCP/UDP UI 和模型；
- 跨协议强唯一；
- 创建、修改、删除；
- active/pending 端口租约；
- Client 离线和强制删除；
- FRPS NewProxy 校验。

验收：并发和跨协议申请同一端口数字只能一个成功。

### 阶段 4：Cloudflare 和域名

- Token pending/active/retired；
- 权限能力和 Zone 标签匹配；
- A/AAAA/CNAME、TTL、小橙云；
- adopt/overwrite；
- 一键更新服务器 IP；
- Mapping 1:N Domain；
- 域名 operation 状态闭环；
- Pending 配额。

### 阶段 5：Router 和证书

- 80/443 Router；
- 版本化快照和本地 IPC；
- SNI 精确证书；
- Host、WebSocket、Streaming、Forwarded Header；
- 404/502；
- HTTPS 合法组合；
- ACME Staging/Production；
- 证书热加载；
- 自动续期和服务端冷却。

### 阶段 6：任务、删除、备份和兼容

- jobs 租约、心跳、去重和接管；
- 删除可逆阶段与补偿；
- Cloudflare 外部残留治理；
- 加密备份和恢复；
- 密钥轮换；
- 协议与客户端版本兼容；
- 自动更新检查；
- 安全审计和故障注入。

验收：Worker、Router、Client 或 Server 在任意中间步骤崩溃后，系统可依据数据库和 last-good 状态恢复，不错误释放端口或域名。


## 52. 必测场景

### 52.1 安装实例、绑定和单活动会话

1. 同一数据目录不能启动两个 Client Panel。
2. 一个安装实例只能托管一个 FRPC。
3. 浏览器登录不得创建新 `client_id`。
4. 电脑登录后，手机登录必须使电脑返回 `401 SESSION_REPLACED`。
5. 抢占仅影响当前 `client_id`，不影响同一用户的其他 Client。
6. 新登录切换失败时不得同时留下两个活动 Session。
7. Client 重启后浏览器需重新登录，但 FRPC 和设备同步继续。
8. 其他用户登录已绑定 Client 返回 `CLIENT_OWNER_MISMATCH`。
9. 退出登录不停止 FRPC、不撤销设备。
10. 退出并解除设备必须撤销设备 HMAC、FRP 凭证并停止 FRPC。

### 52.2 地址和 TLS

11. 同一 `server_instance_id` 从 IP 切换到域名可更新。
12. 不同实例必须走危险切换流程。
13. IP 证书无 IP SAN 时必须拒绝。
14. 自定义 CA 和固定 SPKI 分别测试。
15. 指纹未确认前禁止发送密码。
16. 生产模式不存在跳过 TLS 验证开关。

### 52.3 HMAC 和 Session

17. HMAC 原样重放被拒绝。
18. 时间偏差超窗被拒绝。
19. Nonce 重复被拒绝。
20. Token 轮换后旧版本失效。
21. 用户 Session 不能替代设备 HMAC。
22. 设备 HMAC 不能替代用户授权。
23. 旧 Cookie generation 被拒绝。
24. 用户停用后 Session、HMAC 和 FRPS 鉴权同时失效。

### 52.4 配置版本、签名和 FRPC 队列

25. 多标签页旧版本提交返回 409。
26. 相同幂等键重复请求返回同一 operation。
27. 相同幂等键不同请求体被拒绝。
28. Ed25519 签名错误、Client ID 不匹配、版本倒退均拒绝应用。
29. 签名公钥轮换流程验证。
30. verify 失败不替换现有配置。
31. reload 失败自动回滚。
32. 公共参数变化使用 restart 而非仅 reload。
33. start、stop、verify、reload、restart 和回滚串行执行。
34. 不得启动第二个 FRPC。

### 52.5 TCP、UDP 和端口

35. TCP 创建成功。
36. UDP 创建成功。
37. TCP 占用端口后 UDP 申请相同数字被唯一约束拒绝。
38. Client 离线时端口仍占用。
39. 修改端口先锁新端口，成功后释放旧端口。
40. 修改失败释放新端口并保留旧端口。
41. 自动分配在事务冲突后继续寻找。

### 52.6 Domain、DNS 和证书

42. 一个 Mapping 可绑定多个 Domain。
43. `mapping_revisions` 不存在 `custom_domain`。
44. 面板已有域名不能重复添加。
45. Cloudflare 已有 DNS 支持取消、接管、覆盖。
46. adopted 记录保存 record_id 且不擅自修改。
47. 域名全流程各 step 可恢复。
48. DNS 超时后查询实际结果避免重复创建。
49. DNS 成功但 Client 失败保持正确状态。
50. Client 成功但证书失败保持正确状态。
51. 证书成功但 Router 失败继续使用 last good snapshot。
52. 三种 HTTPS 合法组合校验。
53. Origin CA 域名关闭小橙云前必须先有公共证书。
54. 仅 HTTP 域名不影响其他域名 443。
55. Client 离线返回统一 502。
56. 未绑定 Host 返回 404。

### 52.7 Cloudflare Token

57. 权限不足返回具体缺失能力。
58. pending Token 验证失败时 active Token 不变。
59. 新 Token 会失去现有 Zone 权限时要求确认。
60. 切换后旧 Token job 检测版本并停止。
61. 清除 Token 后 DNS 不自动消失。
62. 清除后续期进入 blocked_missing_token。

### 52.8 Router、任务和删除

63. Router 不访问业务数据库。
64. 快照 HMAC 错误拒绝应用。
65. IPC 通知丢失后通过 current 指针恢复。
66. Router 重启加载 last good snapshot。
67. Worker 崩溃后租约过期任务可接管。
68. 同一域名不能并发执行多个 ACME job。
69. 外部 API 调用期间不持有 SQLite 写事务。
70. Pending 配额超限被拒绝。
71. 删除在 preparing 阶段可取消。
72. 外部资源已删除后取消必须执行补偿。
73. 补偿不完整不得显示取消成功。
74. 强制删除留下外部残留清单和审计。

### 52.9 密钥、备份和初始化

75. 不同用途密钥互不复用。
76. 重启后仍能解密旧 Token。
77. 密钥轮换期间新旧数据均可读。
78. 备份包密码错误不能恢复。
79. 备份包含主密钥恢复方案和校验和。
80. JSON 导出不包含敏感明文。
81. 首次管理员使用随机 12 位一次性密码。
82. 固定 `admin/123456` 不存在。
83. Docker 初始化秘密不只写普通容器日志。


## 53. 设计结论

最终实现必须遵守：

> Server Panel 和 Client Panel 是两个独立产品。Client Panel 不建立本地用户体系，一个安装实例固定绑定一个 Server Panel、一个所属用户、一个 `client_id` 和一个 FRPC。

> 同一个 Client Panel 可以被本机、电脑、手机或平板访问，但同一时间只允许一个有效浏览器代理会话；新登录抢占旧会话，抢占范围仅限当前 `client_id`。

> 浏览器 Session、设备 HMAC 凭证和 FRP 设备凭证是三种不同安全上下文，禁止相互替代。

> 单活动会话不能替代配置版本、资源 revision、幂等键、数据库唯一约束和 FRPC 单队列；这些保护用于应对多标签页、重试、管理员并发和后台任务。

> TCP 与 UDP 同时支持，远程端口数字跨协议全局互斥。Mapping 与 Domain 固定为一对多关系。

> 域名从 DNS、Client 配置、FRPS、证书到 Router 必须形成可观察、可重试、可补偿的 operation 闭环，不能用一个简单状态掩盖中间失败。

> Server Router 只使用版本化、签名校验的本地只读快照；Client 配置使用 Ed25519 签名；不同秘密用途使用不同密钥。

> Cloudflare Token 替换采用 pending -> verify -> confirm -> activate，不直接覆盖；完整备份采用加密归档包，不以 JSON 作为恢复格式。

这套设计的“轻量”指减少外部基础设施和不必要组件，不代表放弃事务、鉴权、密钥生命周期、失败恢复和审计。


## 54. 官方能力参考

开发时应锁定具体版本并重新验证以下官方文档：

- FRP 服务端插件：`https://gofrp.org/zh-cn/docs/features/common/server-plugin/`
- FRP 客户端动态配置：`https://gofrp.org/zh-cn/docs/features/common/client/`
- FRP 配置校验和 includes：`https://gofrp.org/zh-cn/docs/features/common/configure/`
- FRP 客户端配置：`https://gofrp.org/zh-cn/docs/reference/client-configures/`
- FRP 服务端配置：`https://gofrp.org/zh-cn/docs/reference/server-configures/`
- FRP 身份认证：`https://gofrp.org/zh-cn/docs/features/common/authentication/`
- Cloudflare API Token 权限：`https://developers.cloudflare.com/fundamentals/api/reference/permissions/`
- Cloudflare Token Verify：`https://developers.cloudflare.com/api/resources/user/subresources/tokens/methods/verify/`
- Cloudflare Full (strict)：`https://developers.cloudflare.com/ssl/origin-configuration/ssl-modes/full-strict/`
- Cloudflare Origin CA：`https://developers.cloudflare.com/ssl/origin-configuration/origin-ca/`
- SQLite WAL：`https://sqlite.org/wal.html`

