# 部署指南

## 概述

本指南介绍如何部署 FRP 多用户云隧道管理平台。

## 系统要求

### 服务器 (Server Panel)

- **操作系统**: Linux (推荐 Ubuntu 22.04+, Debian 11+, CentOS 8+)
- **CPU**: 2 核+
- **内存**: 2GB+
- **存储**: 20GB+
- **网络**: 公网 IP，开放 80、443、7000 端口

### 客户端 (Client Panel)

- **操作系统**: Linux, Windows, macOS
- **CPU**: 1 核+
- **内存**: 512MB+
- **存储**: 1GB+

## 快速部署 (Docker)

### 1. 克隆项目

```bash
git clone https://github.com/sshiong/frp-cloudflare-platform.git
cd frp-cloudflare-platform
```

### 2. 配置环境

```bash
# 复制配置模板
cp .env.example .env

# 编辑配置
vim .env
```

### 3. 启动服务

```bash
docker-compose up -d
```

### 4. 查看日志

```bash
docker-compose logs -f
```

### 5. 访问面板

- **管理面板**: `https://your-server-ip`
- **初始用户名**: `admin`
- **初始密码**: 查看日志输出

## 手动部署

### 1. 安装依赖

```bash
# Ubuntu/Debian
sudo apt update
sudo apt install -y wget unzip

# CentOS/RHEL
sudo yum install -y wget unzip
```

### 2. 下载二进制文件

```bash
# 下载最新版本
wget https://github.com/sshiong/frp-cloudflare-platform/releases/latest/download/frp-panel-server-linux-amd64.zip
wget https://github.com/sshiong/frp-cloudflare-platform/releases/latest/download/frp-panel-server-router-linux-amd64.zip

# 解压
unzip frp-panel-server-linux-amd64.zip
unzip frp-panel-server-router-linux-amd64.zip

# 移动到系统目录
sudo mv frp-panel-server /usr/local/bin/
sudo mv frp-panel-server-router /usr/local/bin/
sudo chmod +x /usr/local/bin/frp-panel-*
```

### 3. 创建用户和目录

```bash
# 创建用户
sudo useradd -r -s /bin/false frp-panel

# 创建目录
sudo mkdir -p /data /var/lib/frp-panel/router/{snapshots,certificates} /var/log/frp-panel

# 设置权限
sudo chown -R frp-panel:frp-panel /data /var/lib/frp-panel /var/log/frp-panel
```

### 4. 安装 systemd 服务

```bash
# 复制服务文件
sudo cp server-panel/packaging/systemd/frp-panel-control.service /etc/systemd/system/
sudo cp server-panel/packaging/systemd/frp-panel-router.service /etc/systemd/system/

# 重新加载 systemd
sudo systemctl daemon-reload
```

### 5. 启动服务

```bash
# 启动控制面板
sudo systemctl start frp-panel-control

# 启动路由器
sudo systemctl start frp-panel-router

# 设置开机自启
sudo systemctl enable frp-panel-control frp-panel-router
```

### 6. 查看状态

```bash
# 查看服务状态
sudo systemctl status frp-panel-control
sudo systemctl status frp-panel-router

# 查看日志
sudo journalctl -u frp-panel-control -f
sudo journalctl -u frp-panel-router -f
```

## 客户端部署

### Linux

```bash
# 下载客户端
wget https://github.com/sshiong/frp-cloudflare-platform/releases/latest/download/frp-panel-client-linux-amd64.zip

# 解压并安装
unzip frp-panel-client-linux-amd64.zip
sudo mv frp-panel-client /usr/local/bin/
sudo chmod +x /usr/local/bin/frp-panel-client

# 安装 systemd 服务
sudo cp client-panel/packaging/systemd/frp-panel-client.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl start frp-panel-client
sudo systemctl enable frp-panel-client
```

### Windows

1. 下载 Windows 版本
2. 解压到目录
3. 运行 `frp-panel-client.exe`

### macOS

```bash
# 下载 macOS 版本
wget https://github.com/sshiong/frp-cloudflare-platform/releases/latest/download/frp-panel-client-darwin-arm64.zip

# 解压并安装
unzip frp-panel-client-darwin-arm64.zip
sudo mv frp-panel-client /usr/local/bin/
chmod +x /usr/local/bin/frp-panel-client

# 运行
frp-panel-client
```

## 配置说明

### 服务端配置

配置文件位置: `/etc/frp-panel/config.toml`

```toml
[server]
port = 9000
host = "127.0.0.1"

[database]
path = "/data/frp-panel.db"

[router]
snapshot_dir = "/var/lib/frp-panel/router/snapshots"
cert_dir = "/var/lib/frp-panel/router/certificates"

[frps]
bind_port = 7000
vhost_http_port = 8080
```

### 客户端配置

配置文件位置: `~/.frp-panel/config.toml`

```toml
[client]
port = 7410
host = "127.0.0.1"

[server]
url = "https://your-server-ip"

[frpc]
admin_port = 0  # 随机端口
```

## SSL/TLS 配置

### 使用 Let's Encrypt

```bash
# 安装 certbot
sudo apt install certbot

# 获取证书
sudo certbot certonly --standalone -d your-domain.com

# 配置证书路径
# /etc/letsencrypt/live/your-domain.com/fullchain.pem
# /etc/letsencrypt/live/your-domain.com/privkey.pem
```

### 使用自签名证书

```bash
# 生成自签名证书
openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout /etc/frp-panel/server.key \
  -out /etc/frp-panel/server.crt
```

## 防火墙配置

### UFW (Ubuntu)

```bash
# 允许 HTTP
sudo ufw allow 80/tcp

# 允许 HTTPS
sudo ufw allow 443/tcp

# 允许 FRP 端口
sudo ufw allow 7000/tcp

# 禁止外部访问控制面板
sudo ufw deny 9000/tcp
```

### firewalld (CentOS)

```bash
# 允许 HTTP
sudo firewall-cmd --permanent --add-service=http

# 允许 HTTPS
sudo firewall-cmd --permanent --add-service=https

# 允许 FRP 端口
sudo firewall-cmd --permanent --add-port=7000/tcp

# 重新加载
sudo firewall-cmd --reload
```

## 反向代理配置

### Nginx

```nginx
server {
    listen 80;
    server_name your-domain.com;
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name your-domain.com;

    ssl_certificate /etc/letsencrypt/live/your-domain.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/your-domain.com/privkey.pem;

    location / {
        proxy_pass http://127.0.0.1:9000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

## 备份与恢复

### 创建备份

```bash
# 通过 API 创建备份
curl -X POST https://your-server/api/v1/admin/backups \
  -H "Cookie: session=..." \
  -H "Content-Type: application/json" \
  -d '{"password": "backup-password"}'
```

### 恢复备份

```bash
# 上传备份文件
curl -X POST https://your-server/api/v1/admin/restores \
  -H "Cookie: session=..." \
  -F "backup=@backup.frppanel-backup" \
  -F "password=backup-password"
```

## 监控

### 健康检查

```bash
# 检查控制面板
curl http://localhost:9000/api/v1/instance

# 检查路由器
curl http://localhost:80/health
```

### 日志查看

```bash
# 控制面板日志
sudo journalctl -u frp-panel-control -f

# 路由器日志
sudo journalctl -u frp-panel-router -f

# 客户端日志
sudo journalctl -u frp-panel-client -f
```

## 故障排除

### 常见问题

1. **无法访问面板**
   - 检查防火墙设置
   - 检查服务状态
   - 检查日志

2. **数据库错误**
   - 检查数据库文件权限
   - 检查磁盘空间

3. **证书错误**
   - 检查证书文件
   - 检查证书过期时间

### 日志级别

```bash
# 设置日志级别
export FRP_PANEL_LOG_LEVEL=debug

# 可选级别: debug, info, warn, error
```

## 性能优化

### 数据库优化

```sql
-- 定期清理旧数据
DELETE FROM audit_logs WHERE created_at < datetime('now', '-90 days');
DELETE FROM jobs WHERE status = 'completed' AND completed_at < datetime('now', '-30 days');

-- 执行 VACUUM
VACUUM;
```

### 系统优化

```bash
# 增加文件描述符限制
echo "frp-panel soft nofile 65536" | sudo tee -a /etc/security/limits.conf
echo "frp-panel hard nofile 65536" | sudo tee -a /etc/security/limits.conf
```

## 升级

### Docker 升级

```bash
# 拉取最新镜像
docker-compose pull

# 重启服务
docker-compose up -d
```

### 手动升级

```bash
# 停止服务
sudo systemctl stop frp-panel-control frp-panel-router

# 备份数据
sudo cp -r /data /data.backup

# 替换二进制文件
sudo mv frp-panel-server /usr/local/bin/
sudo mv frp-panel-server-router /usr/local/bin/

# 启动服务
sudo systemctl start frp-panel-control frp-panel-router
```
