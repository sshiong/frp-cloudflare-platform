# API 文档

## 概述

FRP Panel API 使用 RESTful 风格，所有请求和响应均使用 JSON 格式。

### 基础 URL

- Server Panel: `https://your-server/api/v1`
- Client Panel: `http://127.0.0.1:7410/api`

### 认证

#### 浏览器用户认证 (Server Panel)

使用 Session Cookie 认证：

```
POST /api/v1/auth/login
Content-Type: application/json

{
  "username": "admin",
  "password": "your-password"
}

Response:
{
  "success": true,
  "user_id": "uuid",
  "username": "admin",
  "role": "admin"
}
```

#### 设备 API 认证 (HMAC-SHA256)

设备 API 使用 HMAC-SHA256 签名认证：

```
GET /api/v1/client/config
X-Client-ID: client-uuid
X-Device-Token-Version: 1
X-Request-Timestamp: 1625097600
X-Request-Nonce: random-base64url
X-Content-SHA256: e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
Authorization: Device-HMAC-SHA256 Signature=hex-signature
```

### 错误响应

所有错误响应格式：

```json
{
  "error": {
    "code": "ERROR_CODE",
    "message": "错误描述",
    "details": {},
    "request_id": "uuid"
  }
}
```

## API 端点

### 实例信息

#### GET /api/v1/instance

获取服务器实例信息（无需认证）。

**响应：**
```json
{
  "server_instance_id": "uuid",
  "server_version": "2.3.0",
  "protocol_version": 1,
  "min_client_version": "2.3.0"
}
```

### 认证

#### POST /api/v1/auth/login

用户登录。

**请求：**
```json
{
  "username": "admin",
  "password": "password"
}
```

**响应：**
```json
{
  "success": true,
  "must_change_password": false,
  "user_id": "uuid",
  "username": "admin",
  "role": "admin"
}
```

#### POST /api/v1/auth/logout

用户登出。

#### GET /api/v1/auth/session

获取当前会话信息。

### 用户管理 (管理员)

#### GET /api/v1/admin/users

获取用户列表。

#### POST /api/v1/admin/users

创建用户。

**请求：**
```json
{
  "username": "newuser",
  "role": "user",
  "max_clients": 5,
  "max_mappings": 20,
  "max_domains": 10
}
```

**响应：**
```json
{
  "user_id": "uuid",
  "username": "newuser",
  "initial_password": "random-password"
}
```

#### PATCH /api/v1/admin/users/:id

更新用户信息。

#### POST /api/v1/admin/users/:id/disable

停用用户。

#### POST /api/v1/admin/users/:id/enable

启用用户。

#### DELETE /api/v1/admin/users/:id

删除用户（需要二次认证）。

### 设备管理

#### POST /api/v1/devices/register

注册新设备（需要用户 Session）。

**请求：**
```json
{
  "installation_instance_id": "uuid",
  "device_name": "My NAS",
  "client_panel_version": "2.3.0",
  "frpc_version": "0.58.0",
  "protocol_version": 1,
  "config_schema_version": 1,
  "os": "linux",
  "arch": "amd64"
}
```

**响应：**
```json
{
  "client_id": "uuid",
  "device_token": "secret-token",
  "frp_device_token": "frp-secret",
  "config_signing_public_key": "ed25519-public-key",
  "config_signing_key_id": "key-id"
}
```

#### GET /api/v1/devices/current

获取当前设备信息（HMAC 认证）。

#### POST /api/v1/devices/current/rotate

轮换设备 Token（HMAC 认证）。

#### POST /api/v1/devices/current/unbind

解绑设备（需要二次认证）。

### 映射管理

#### GET /api/v1/mappings

获取映射列表。

#### POST /api/v1/mappings

创建映射。

**请求：**
```json
{
  "name": "My Web Server",
  "proxy_type": "tcp",
  "local_ip": "127.0.0.1",
  "local_port": 8080,
  "remote_port": 8080,
  "expected_config_version": 1,
  "idempotency_key": "uuid"
}
```

#### PATCH /api/v1/mappings/:id

更新映射。

#### POST /api/v1/mappings/:id/enable

启用映射。

#### POST /api/v1/mappings/:id/disable

禁用映射。

#### DELETE /api/v1/mappings/:id

删除映射。

### 域名管理

#### GET /api/v1/domains

获取域名列表。

#### POST /api/v1/domains/preflight

域名预检。

**请求：**
```json
{
  "hostname": "api.example.com"
}
```

**响应：**
```json
{
  "domain_exists": false,
  "cloudflare_dns": null,
  "zone_id": "zone-id",
  "zone_name": "example.com",
  "has_permission": true,
  "missing_permissions": []
}
```

#### POST /api/v1/domains

创建域名绑定。

### Cloudflare 管理

#### GET /api/v1/cloudflare/token/status

获取 Token 状态。

**响应：**
```json
{
  "has_token": true,
  "status": "valid",
  "token_version": 1,
  "verified_at": 1625097600,
  "capabilities": ["Zone Read", "DNS Write"]
}
```

#### POST /api/v1/cloudflare/token/pending

上传新 Token（待验证）。

#### POST /api/v1/cloudflare/token/:version/verify

验证 Token。

#### POST /api/v1/cloudflare/token/:version/activate

激活 Token。

#### DELETE /api/v1/cloudflare/token

清除 Token（需要二次认证）。

### 证书管理

#### GET /api/v1/certificates

获取证书列表。

#### POST /api/v1/certificates/:domain_id/issue

签发证书。

#### POST /api/v1/certificates/:domain_id/renew

续期证书。

### 操作管理

#### GET /api/v1/operations/:id

获取操作详情。

#### POST /api/v1/operations/:id/cancel

取消操作。

#### POST /api/v1/operations/:id/retry

重试操作。

### 客户端 API (HMAC 认证)

#### GET /api/v1/client/bootstrap

获取客户端引导数据。

#### GET /api/v1/client/config

获取配置快照。

#### POST /api/v1/client/config/apply-result

上报配置应用结果。

#### POST /api/v1/client/heartbeat

发送心跳。

#### POST /api/v1/client/status

上报状态。

### FRPS 内部插件

#### POST /internal/frp/login

FRPS Login 钩子。

#### POST /internal/frp/new-proxy

FRPS NewProxy 钩子。

#### POST /internal/frp/new-work-conn

FRPS NewWorkConn 钩子。

## WebSocket 事件

### 连接

```
ws://localhost:9000/api/v1/client/events
```

### 事件类型

| 事件 | 描述 |
|------|------|
| config_update | 配置更新 |
| config_version_change | 配置版本变更 |
| force_sync | 强制同步 |
| frp_token_reset | FRP Token 重置 |
| cloudflare_token_changed | Cloudflare Token 变更 |
| user_disabled | 用户停用 |
| device_revoked | 设备撤销 |
| mapping_created | 映射创建 |
| mapping_updated | 映射更新 |
| mapping_deleted | 映射删除 |
| domain_created | 域名创建 |
| domain_deleted | 域名删除 |
| certificate_issued | 证书签发 |
| certificate_error | 证书错误 |

### 事件格式

```json
{
  "type": "config_update",
  "payload": {
    "config_version": 2,
    "action": "reload"
  },
  "timestamp": 1625097600
}
```
