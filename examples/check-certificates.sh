#!/bin/bash
# FRP Panel 证书检查和续期脚本
# 检查证书状态并自动续期

set -e

# 配置
SERVER_URL="http://localhost:9000"
RENEWAL_THRESHOLD_DAYS=30
LOG_FILE="/var/log/frp-panel/certificates.log"

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

log "=== 证书检查开始 ==="

# 获取证书列表
certificates=$(curl -s "${SERVER_URL}/api/v1/certificates" \
    -H "Cookie: session=${FRP_PANEL_SESSION}" 2>/dev/null)

if [ -z "$certificates" ]; then
    error "无法获取证书列表"
    exit 1
fi

# 解析证书信息
echo "$certificates" | jq -r '.[] | @base64' | while read -r cert; do
    domain=$(echo "$cert" | base64 -d | jq -r '.domain')
    status=$(echo "$cert" | base64 -d | jq -r '.status')
    not_after=$(echo "$cert" | base64 -d | jq -r '.not_after')
    cert_id=$(echo "$cert" | base64 -d | jq -r '.id')

    # 计算剩余天数
    now=$(date +%s)
    days_left=$(( (not_after - now) / 86400 ))

    log "域名: ${domain}"
    log "  状态: ${status}"
    log "  剩余天数: ${days_left}"

    # 检查是否需要续期
    if [ "$status" = "valid" ] && [ "$days_left" -lt "$RENEWAL_THRESHOLD_DAYS" ]; then
        warn "证书即将过期，尝试续期: ${domain}"

        # 调用续期 API
        response=$(curl -s -X POST "${SERVER_URL}/api/v1/certificates/${cert_id}/renew" \
            -H "Cookie: session=${FRP_PANEL_SESSION}" \
            -H "Content-Type: application/json" 2>/dev/null)

        if echo "$response" | jq -e '.success' > /dev/null 2>&1; then
            log "续期任务已创建: ${domain}"
        else
            error "续期失败: ${domain} - $(echo "$response" | jq -r '.error.message')"
        fi
    elif [ "$status" = "expired" ]; then
        error "证书已过期: ${domain}"
    elif [ "$status" = "error" ]; then
        error "证书错误: ${domain}"
    fi

    echo ""
done

log "=== 证书检查完成 ==="
