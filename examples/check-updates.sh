#!/bin/bash
# FRP Panel 更新检查脚本
# 检查是否有新版本可用

set -e

# 配置
GITHUB_REPO="sshiong/frp-cloudflare-platform"
CURRENT_VERSION="2.3.0"
LOG_FILE="/var/log/frp-panel/updates.log"
NOTIFY_EMAIL=""

# 颜色
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log() {
    echo "$(date -u +%Y-%m-%dT%H:%M:%SZ) [INFO] $1" >> "$LOG_FILE"
    echo -e "${GREEN}[INFO]${NC} $1"
}

warn() {
    echo "$(date -u +%Y-%m-%dT%H:%M:%SZ) [WARN] $1" >> "$LOG_FILE"
    echo -e "${YELLOW}[WARN]${NC} $1"
}

error() {
    echo "$(date -u +%Y-%m-%dT%H:%M:%SZ) [ERROR] $1" >> "$LOG_FILE"
    echo -e "${RED}[ERROR]${NC} $1"
}

log "=== 更新检查开始 ==="
log "当前版本: ${CURRENT_VERSION}"

# 获取最新版本
log "检查 GitHub Release..."
latest_release=$(curl -s "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" 2>/dev/null)

if [ -z "$latest_release" ]; then
    error "无法获取版本信息"
    exit 1
fi

latest_version=$(echo "$latest_release" | jq -r '.tag_name' | sed 's/^v//')
release_url=$(echo "$latest_release" | jq -r '.html_url')
release_notes=$(echo "$latest_release" | jq -r '.body' | head -20)

log "最新版本: ${latest_version}"

# 版本比较
if [ "$latest_version" = "$CURRENT_VERSION" ]; then
    log "已是最新版本"
elif [ "$(printf '%s\n' "$CURRENT_VERSION" "$latest_version" | sort -V | head -n1)" = "$CURRENT_VERSION" ]; then
    warn "发现新版本: ${latest_version}"
    warn "下载地址: ${release_url}"
    warn "更新说明:"
    echo "$release_notes" | while read -r line; do
        warn "  $line"
    done

    # 发送通知邮件
    if [ -n "$NOTIFY_EMAIL" ]; then
        echo "FRP Panel 发现新版本: ${latest_version}\n\n下载地址: ${release_url}\n\n更新说明:\n${release_notes}" | \
            mail -s "FRP Panel 新版本通知" "$NOTIFY_EMAIL" 2>/dev/null || true
    fi
else
    log "当前版本比最新版本更新（可能是开发版）"
fi

# 检查 FRP 版本
log "检查 FRP 版本..."
frp_release=$(curl -s "https://api.github.com/repos/fatedier/frp/releases/latest" 2>/dev/null)

if [ -n "$frp_release" ]; then
    frp_latest=$(echo "$frp_release" | jq -r '.tag_name' | sed 's/^v//')
    log "FRP 最新版本: ${frp_latest}"

    # 检查内置 FRP 版本
    if [ -f "/usr/local/bin/frpc" ]; then
        frpc_version=$(/usr/local/bin/frpc --version 2>/dev/null | head -1)
        log "内置 FRPC 版本: ${frpc_version}"
    fi
fi

# 检查安全公告
log "检查安全公告..."
security_advisories=$(curl -s "https://api.github.com/repos/${GITHUB_REPO}/security/advisories" 2>/dev/null)

if [ -n "$security_advisories" ] && [ "$security_advisories" != "[]" ]; then
    warn "发现安全公告！"
    echo "$security_advisories" | jq -r '.[] | "  - \(.summary) (\(.severity))"'
fi

log "=== 更新检查完成 ==="

# 返回状态
if [ "$latest_version" != "$CURRENT_VERSION" ] && [ "$(printf '%s\n' "$CURRENT_VERSION" "$latest_version" | sort -V | head -n1)" = "$CURRENT_VERSION" ]; then
    exit 2  # 有更新可用
else
    exit 0  # 已是最新
fi
