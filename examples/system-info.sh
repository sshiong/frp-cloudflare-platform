#!/bin/bash
# FRP Panel 系统信息脚本
# 显示系统和 FRP Panel 运行状态

set -e

# 颜色
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# 分隔线
separator() {
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
}

# 标题
header() {
    echo -e "${CYAN}$1${NC}"
}

# 系统信息
header "系统信息"
separator
echo -e "操作系统:    $(uname -s) $(uname -r)"
echo -e "架构:        $(uname -m)"
echo -e "主机名:      $(hostname)"
echo -e "当前时间:    $(date)"
echo -e "运行时间:    $(uptime -p 2>/dev/null || uptime)"
echo ""

# CPU 信息
header "CPU 信息"
separator
echo -e "型号:        $(grep 'model name' /proc/cpuinfo | head -1 | cut -d: -f2 | xargs)"
echo -e "核心数:      $(nproc)"
echo -e "负载:        $(cat /proc/loadavg | cut -d' ' -f1-3)"
echo ""

# 内存信息
header "内存信息"
separator
free -h | head -2
echo ""

# 磁盘信息
header "磁盘信息"
separator
df -h / /data 2>/dev/null || df -h /
echo ""

# 网络信息
header "网络信息"
separator
echo -e "IP 地址:     $(hostname -I | awk '{print $1}')"
echo -e "公网 IP:     $(curl -s ifconfig.me 2>/dev/null || echo '无法获取')"
echo ""

# FRP Panel 状态
header "FRP Panel 状态"
separator

# 检查进程
echo -e "进程状态:"
if pgrep -f "frp-panel-server control" > /dev/null; then
    echo -e "  Server Control:  ${GREEN}✓ 运行中${NC} (PID: $(pgrep -f 'frp-panel-server control'))"
else
    echo -e "  Server Control:  ${RED}✗ 未运行${NC}"
fi

if pgrep -f "frp-panel-server-router router" > /dev/null; then
    echo -e "  Server Router:   ${GREEN}✓ 运行中${NC} (PID: $(pgrep -f 'frp-panel-server-router router'))"
else
    echo -e "  Server Router:   ${RED}✗ 未运行${NC}"
fi

if pgrep -f "frps" > /dev/null; then
    echo -e "  FRPS:            ${GREEN}✓ 运行中${NC} (PID: $(pgrep -f 'frps'))"
else
    echo -e "  FRPS:            ${RED}✗ 未运行${NC}"
fi

if pgrep -f "frp-panel-client" > /dev/null; then
    echo -e "  Client Panel:    ${GREEN}✓ 运行中${NC} (PID: $(pgrep -f 'frp-panel-client'))"
else
    echo -e "  Client Panel:    ${RED}✗ 未运行${NC}"
fi

echo ""

# 端口状态
echo -e "端口状态:"
for port in 80 443 7000 9000 7410; do
    if nc -z -w1 localhost $port 2>/dev/null; then
        echo -e "  端口 ${port}:       ${GREEN}✓ 开放${NC}"
    else
        echo -e "  端口 ${port}:       ${RED}✗ 关闭${NC}"
    fi
done

echo ""

# API 状态
echo -e "API 状态:"
api_status=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:9000/api/v1/instance 2>/dev/null || echo "000")
if [ "$api_status" = "200" ]; then
    echo -e "  Server API:      ${GREEN}✓ 正常${NC} (HTTP ${api_status})"
else
    echo -e "  Server API:      ${RED}✗ 异常${NC} (HTTP ${api_status})"
fi

router_status=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:80/health 2>/dev/null || echo "000")
if [ "$router_status" = "200" ]; then
    echo -e "  Router:          ${GREEN}✓ 正常${NC} (HTTP ${router_status})"
else
    echo -e "  Router:          ${RED}✗ 异常${NC} (HTTP ${router_status})"
fi

echo ""

# 数据库信息
header "数据库信息"
separator
DB_PATH="/data/frp-panel.db"
if [ -f "$DB_PATH" ]; then
    db_size=$(stat -f%z "$DB_PATH" 2>/dev/null || stat -c%s "$DB_PATH" 2>/dev/null)
    echo -e "数据库大小:  $(numfmt --to=iec $db_size)"

    # WAL 大小
    if [ -f "${DB_PATH}-wal" ]; then
        wal_size=$(stat -f%z "${DB_PATH}-wal" 2>/dev/null || stat -c%s "${DB_PATH}-wal" 2>/dev/null)
        echo -e "WAL 大小:    $(numfmt --to=iec $wal_size)"
    fi

    # 完整性检查
    integrity=$(sqlite3 "$DB_PATH" "PRAGMA integrity_check;" 2>/dev/null)
    if [ "$integrity" = "ok" ]; then
        echo -e "完整性:      ${GREEN}✓ 正常${NC}"
    else
        echo -e "完整性:      ${RED}✗ 异常${NC}"
    fi

    # 统计信息
    echo ""
    echo -e "数据统计:"
    echo -e "  用户数:    $(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM users;" 2>/dev/null || echo 'N/A')"
    echo -e "  客户端数:  $(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM clients;" 2>/dev/null || echo 'N/A')"
    echo -e "  映射数:    $(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM mappings;" 2>/dev/null || echo 'N/A')"
    echo -e "  域名数:    $(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM domain_bindings;" 2>/dev/null || echo 'N/A')"
    echo -e "  在线客户端:$(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM clients WHERE status = 'online';" 2>/dev/null || echo 'N/A')"
else
    echo -e "数据库文件:  ${RED}不存在${NC}"
fi

echo ""

# 日志信息
header "日志信息"
separator
log_dir="/var/log/frp-panel"
if [ -d "$log_dir" ]; then
    log_size=$(du -sh "$log_dir" 2>/dev/null | cut -f1)
    log_count=$(find "$log_dir" -name "*.log" | wc -l)
    echo -e "日志目录:    ${log_dir}"
    echo -e "日志大小:    ${log_size}"
    echo -e "日志文件数:  ${log_count}"

    # 最近错误
    echo ""
    echo -e "最近错误 (最新 5 条):"
    find "$log_dir" -name "*.log" -exec grep -h "ERROR" {} \; 2>/dev/null | tail -5 | while read -r line; do
        echo -e "  ${RED}${line}${NC}"
    done
else
    echo -e "日志目录:    ${YELLOW}不存在${NC}"
fi

echo ""

# 证书信息
header "证书信息"
separator
cert_dir="/data/certificates"
if [ -d "$cert_dir" ]; then
    cert_count=$(find "$cert_dir" -name "fullchain.pem" | wc -l)
    echo -e "证书数量:    ${cert_count}"

    # 列出证书
    find "$cert_dir" -name "fullchain.pem" | while read -r cert; do
        domain=$(dirname "$cert" | xargs basename)
        expiry=$(openssl x509 -enddate -noout -in "$cert" 2>/dev/null | cut -d= -f2)
        days_left=$(( ( $(date -d "$expiry" +%s 2>/dev/null || date -jf "%b %d %T %Y %Z" "$expiry" +%s 2>/dev/null) - $(date +%s) ) / 86400 ))

        if [ $days_left -lt 0 ]; then
            echo -e "  ${domain}: ${RED}已过期${NC}"
        elif [ $days_left -lt 30 ]; then
            echo -e "  ${domain}: ${YELLOW}即将过期 (${days_left} 天)${NC}"
        else
            echo -e "  ${domain}: ${GREEN}有效 (${days_left} 天)${NC}"
        fi
    done
else
    echo -e "证书目录:    ${YELLOW}不存在${NC}"
fi

echo ""
separator
echo -e "${CYAN}FRP Panel 系统信息报告完成${NC}"
