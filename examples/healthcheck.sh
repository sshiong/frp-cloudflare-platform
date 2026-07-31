#!/bin/bash
# FRP Panel 健康检查脚本
# 用于监控服务状态

set -e

# 配置
SERVER_URL="http://localhost:9000"
ROUTER_URL="http://localhost:80"
TIMEOUT=5
RETRY=3

# 颜色
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# 检查结果
CHECKS_PASSED=0
CHECKS_FAILED=0

# 检查函数
check_service() {
    local name=$1
    local url=$2
    local expected_status=${3:-200}

    echo -n "检查 ${name}..."

    for i in $(seq 1 $RETRY); do
        status=$(curl -s -o /dev/null -w "%{http_code}" --connect-timeout $TIMEOUT "$url" 2>/dev/null || echo "000")

        if [ "$status" = "$expected_status" ]; then
            echo -e " ${GREEN}✓ 正常${NC}"
            CHECKS_PASSED=$((CHECKS_PASSED + 1))
            return 0
        fi

        if [ $i -lt $RETRY ]; then
            echo -n " 重试 ${i}/${RETRY}..."
            sleep 1
        fi
    done

    echo -e " ${RED}✗ 异常 (状态码: ${status})${NC}"
    CHECKS_FAILED=$((CHECKS_FAILED + 1))
    return 1
}

# 检查端口
check_port() {
    local name=$1
    local host=$2
    local port=$3

    echo -n "检查 ${name} 端口..."

    if nc -z -w $TIMEOUT $host $port 2>/dev/null; then
        echo -e " ${GREEN}✓ 开放${NC}"
        CHECKS_PASSED=$((CHECKS_PASSED + 1))
        return 0
    else
        echo -e " ${RED}✗ 关闭${NC}"
        CHECKS_FAILED=$((CHECKS_FAILED + 1))
        return 1
    fi
}

# 检查进程
check_process() {
    local name=$1
    local process=$2

    echo -n "检查 ${name} 进程..."

    if pgrep -f "$process" > /dev/null; then
        echo -e " ${GREEN}✓ 运行中${NC}"
        CHECKS_PASSED=$((CHECKS_PASSED + 1))
        return 0
    else
        echo -e " ${RED}✗ 未运行${NC}"
        CHECKS_FAILED=$((CHECKS_FAILED + 1))
        return 1
    fi
}

# 检查磁盘空间
check_disk() {
    local path=$1
    local threshold=${2:-90}

    echo -n "检查磁盘空间..."

    usage=$(df -h "$path" | tail -1 | awk '{print $5}' | sed 's/%//')

    if [ "$usage" -lt "$threshold" ]; then
        echo -e " ${GREEN}✓ ${usage}% 已用${NC}"
        CHECKS_PASSED=$((CHECKS_PASSED + 1))
        return 0
    else
        echo -e " ${RED}✗ ${usage}% 已用 (超过 ${threshold}% 阈值)${NC}"
        CHECKS_FAILED=$((CHECKS_FAILED + 1))
        return 1
    fi
}

# 检查内存
check_memory() {
    local threshold=${1:-90}

    echo -n "检查内存使用..."

    usage=$(free | grep Mem | awk '{printf "%.0f", $3/$2 * 100}')

    if [ "$usage" -lt "$threshold" ]; then
        echo -e " ${GREEN}✓ ${usage}% 已用${NC}"
        CHECKS_PASSED=$((CHECKS_PASSED + 1))
        return 0
    else
        echo -e " ${RED}✗ ${usage}% 已用 (超过 ${threshold}% 阈值)${NC}"
        CHECKS_FAILED=$((CHECKS_FAILED + 1))
        return 1
    fi
}

# 主程序
echo -e "${GREEN}=== FRP Panel 健康检查 ===${NC}"
echo -e "时间: $(date)"
echo ""

# 进程检查
echo -e "${YELLOW}进程状态:${NC}"
check_process "FRP Panel Server" "frp-panel-server control"
check_process "FRP Panel Router" "frp-panel-server-router router"
check_process "FRPS" "frps"
echo ""

# 端口检查
echo -e "${YELLOW}端口状态:${NC}"
check_port "FRP Panel API" "127.0.0.1" "9000"
check_port "HTTP" "0.0.0.0" "80"
check_port "HTTPS" "0.0.0.0" "443"
check_port "FRPS" "0.0.0.0" "7000"
echo ""

# HTTP 检查
echo -e "${YELLOW}HTTP 服务:${NC}"
check_service "FRP Panel API" "${SERVER_URL}/api/v1/instance"
check_service "FRP Panel Router" "${ROUTER_URL}/health"
echo ""

# 系统资源
echo -e "${YELLOW}系统资源:${NC}"
check_disk "/data" 90
check_memory 90
echo ""

# 数据库检查
echo -e "${YELLOW}数据库:${NC}"
echo -n "检查数据库完整性..."
if [ -f "/data/frp-panel.db" ]; then
    result=$(sqlite3 /data/frp-panel.db "PRAGMA integrity_check;" 2>/dev/null)
    if [ "$result" = "ok" ]; then
        echo -e " ${GREEN}✓ 完整${NC}"
        CHECKS_PASSED=$((CHECKS_PASSED + 1))
    else
        echo -e " ${RED}✗ 损坏${NC}"
        CHECKS_FAILED=$((CHECKS_FAILED + 1))
    fi
else
    echo -e " ${YELLOW}⚠ 文件不存在${NC}"
fi

# WAL 检查
echo -n "检查 WAL 文件..."
if [ -f "/data/frp-panel.db-wal" ]; then
    wal_size=$(stat -f%z "/data/frp-panel.db-wal" 2>/dev/null || stat -c%s "/data/frp-panel.db-wal" 2>/dev/null)
    if [ "$wal_size" -lt 104857600 ]; then  # 100MB
        echo -e " ${GREEN}✓ $(numfmt --to=iec $wal_size)${NC}"
        CHECKS_PASSED=$((CHECKS_PASSED + 1))
    else
        echo -e " ${YELLOW}⚠ $(numfmt --to=iec $wal_size) (较大)${NC}"
    fi
else
    echo -e " ${GREEN}✓ 无 WAL 文件${NC}"
fi

echo ""

# 总结
echo -e "${GREEN}=== 检查完成 ===${NC}"
echo -e "通过: ${GREEN}${CHECKS_PASSED}${NC}"
echo -e "失败: ${RED}${CHECKS_FAILED}${NC}"

if [ $CHECKS_FAILED -gt 0 ]; then
    echo -e "\n${RED}警告: 存在失败的检查项！${NC}"
    exit 1
else
    echo -e "\n${GREEN}所有检查通过！${NC}"
    exit 0
fi
