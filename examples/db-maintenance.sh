#!/bin/bash
# FRP Panel 数据库维护脚本
# 定期清理和优化数据库

set -e

# 配置
DB_PATH="/data/frp-panel.db"
LOG_FILE="/var/log/frp-panel/db-maintenance.log"
AUDIT_RETENTION_DAYS=90
JOB_RETENTION_DAYS=30
NONCE_RETENTION_HOURS=24

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

# 检查数据库文件
if [ ! -f "$DB_PATH" ]; then
    error "数据库文件不存在: $DB_PATH"
    exit 1
fi

log "=== 数据库维护开始 ==="
log "数据库路径: $DB_PATH"

# 记录初始大小
initial_size=$(stat -f%z "$DB_PATH" 2>/dev/null || stat -c%s "$DB_PATH" 2>/dev/null)
log "初始大小: $(numfmt --to=iec $initial_size)"

# 清理旧审计日志
log "清理 ${AUDIT_RETENTION_DAYS} 天前的审计日志..."
deleted=$(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM audit_logs WHERE created_at < datetime('now', '-${AUDIT_RETENTION_DAYS} days');")
sqlite3 "$DB_PATH" "DELETE FROM audit_logs WHERE created_at < datetime('now', '-${AUDIT_RETENTION_DAYS} days');"
log "已删除 ${deleted} 条审计日志"

# 清理旧任务记录
log "清理 ${JOB_RETENTION_DAYS} 天前的已完成任务..."
deleted=$(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM jobs WHERE status = 'completed' AND completed_at < datetime('now', '-${JOB_RETENTION_DAYS} days');")
sqlite3 "$DB_PATH" "DELETE FROM jobs WHERE status = 'completed' AND completed_at < datetime('now', '-${JOB_RETENTION_DAYS} days');"
log "已删除 ${deleted} 条任务记录"

# 清理过期 Nonce
log "清理 ${NONCE_RETENTION_HOURS} 小时前的 Nonce..."
deleted=$(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM device_request_nonces WHERE created_at < datetime('now', '-${NONCE_RETENTION_HOURS} hours');")
sqlite3 "$DB_PATH" "DELETE FROM device_request_nonces WHERE created_at < datetime('now', '-${NONCE_RETENTION_HOURS} hours');"
log "已删除 ${deleted} 条 Nonce 记录"

# 清理过期幂等记录
log "清理过期的幂等记录..."
deleted=$(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM idempotency_records WHERE expires_at < datetime('now');")
sqlite3 "$DB_PATH" "DELETE FROM idempotency_records WHERE expires_at < datetime('now');"
log "已删除 ${deleted} 条幂等记录"

# 清理过期 Session
log "清理过期的 Session..."
deleted=$(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM sessions WHERE expires_at < datetime('now') AND revoked_at IS NULL;")
sqlite3 "$DB_PATH" "UPDATE sessions SET revoked_at = datetime('now'), revoke_reason = 'expired' WHERE expires_at < datetime('now') AND revoked_at IS NULL;"
log "已撤销 ${deleted} 条过期 Session"

# 统计信息
log "数据库统计:"
log "  用户数: $(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM users;")"
log "  客户端数: $(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM clients;")"
log "  映射数: $(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM mappings;")"
log "  域名数: $(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM domain_bindings;")"
log "  活跃 Session: $(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM sessions WHERE revoked_at IS NULL;")"
log "  待处理任务: $(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM jobs WHERE status IN ('pending', 'running');")"

# 检查完整性
log "检查数据库完整性..."
integrity=$(sqlite3 "$DB_PATH" "PRAGMA integrity_check;")
if [ "$integrity" = "ok" ]; then
    log "数据库完整性: 正常"
else
    error "数据库完整性检查失败: $integrity"
fi

# 分析表
log "分析表统计信息..."
sqlite3 "$DB_PATH" "ANALYZE;"

# 执行 VACUUM
log "执行 VACUUM..."
sqlite3 "$DB_PATH" "VACUUM;"

# 记录最终大小
final_size=$(stat -f%z "$DB_PATH" 2>/dev/null || stat -c%s "$DB_PATH" 2>/dev/null)
log "最终大小: $(numfmt --to=iec $final_size)"

# 计算节省空间
saved=$((initial_size - final_size))
if [ $saved -gt 0 ]; then
    log "节省空间: $(numfmt --to=iec $saved)"
fi

log "=== 数据库维护完成 ==="
