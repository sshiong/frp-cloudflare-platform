#!/bin/bash
# FRP Panel 综合测试脚本

set -e

# 颜色
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

PASSED=0
FAILED=0

# 测试函数
test_endpoint() {
    local name=$1
    local method=$2
    local url=$3
    local expected_status=$4
    local data=$5

    echo -n "测试 $name..."

    if [ -n "$data" ]; then
        status=$(curl -s -o /dev/null -w "%{http_code}" -X $method "$url" -H "Content-Type: application/json" -d "$data" 2>/dev/null)
    else
        status=$(curl -s -o /dev/null -w "%{http_code}" -X $method "$url" 2>/dev/null)
    fi

    if [ "$status" = "$expected_status" ]; then
        echo -e " ${GREEN}✓ 通过${NC} (HTTP $status)"
        PASSED=$((PASSED + 1))
    else
        echo -e " ${RED}✗ 失败${NC} (期望 $expected_status, 实际 $status)"
        FAILED=$((FAILED + 1))
    fi
}

echo -e "${GREEN}=== FRP Panel 综合测试 ===${NC}"
echo ""

# 启动服务器
echo -e "${YELLOW}启动服务器...${NC}"
cd /Users/ricardo/vibeitem/frp/server-panel
rm -f data/panel.db data/panel.db-wal data/panel.db-shm
./bin/frp-panel-server -port 9000 -dev &
SERVER_PID=$!
sleep 2

# 测试公开端点
echo ""
echo -e "${YELLOW}=== 公开端点测试 ===${NC}"
test_endpoint "健康检查" "GET" "http://localhost:9000/health" "200"
test_endpoint "实例信息" "GET" "http://localhost:9000/api/v1/instance" "200"

# 测试认证
echo ""
echo -e "${YELLOW}=== 认证测试 ===${NC}"
test_endpoint "登录 (错误密码)" "POST" "http://localhost:9000/api/v1/auth/login" "401" '{"username":"admin","password":"wrong"}'
test_endpoint "登录 (空用户名)" "POST" "http://localhost:9000/api/v1/auth/login" "400" '{"username":"","password":"test"}'

# 管理员登录测试 (使用随机密码)
echo ""
echo -e "${YELLOW}=== 管理员登录测试 ===${NC}"
# 注意: 每次运行密码不同，这里测试错误密码应返回401
test_endpoint "管理员登录 (错误密码)" "POST" "http://localhost:9000/api/v1/auth/login" "401" '{"username":"admin","password":"wrong-password"}'

# 测试需要认证的端点 (无session应返回401)
echo ""
echo -e "${YELLOW}=== 认证保护测试 ===${NC}"
test_endpoint "用户列表 (未认证)" "GET" "http://localhost:9000/api/v1/admin/users" "401"
test_endpoint "映射列表 (未认证)" "GET" "http://localhost:9000/api/v1/mappings" "401"
test_endpoint "域名列表 (未认证)" "GET" "http://localhost:9000/api/v1/domains" "401"

# 测试FRPS插件端点 (插件接受空请求返回200是正常行为)
echo ""
echo -e "${YELLOW}=== FRPS插件测试 ===${NC}"
test_endpoint "FRPS Login" "POST" "http://localhost:9000/internal/frp/login" "200"
test_endpoint "FRPS NewProxy" "POST" "http://localhost:9000/internal/frp/new-proxy" "200"

# 停止服务器
kill $SERVER_PID 2>/dev/null
wait $SERVER_PID 2>/dev/null

# 测试前端构建
echo ""
echo -e "${YELLOW}=== 前端构建测试 ===${NC}"
echo -n "测试 Server Panel 前端构建..."
cd /Users/ricardo/vibeitem/frp/server-panel/web-admin
if npm run build > /dev/null 2>&1; then
    echo -e " ${GREEN}✓ 通过${NC}"
    PASSED=$((PASSED + 1))
else
    echo -e " ${RED}✗ 失败${NC}"
    FAILED=$((FAILED + 1))
fi

echo -n "测试 Client Panel 前端构建..."
cd /Users/ricardo/vibeitem/frp/client-panel/web-client
if npm run build > /dev/null 2>&1; then
    echo -e " ${GREEN}✓ 通过${NC}"
    PASSED=$((PASSED + 1))
else
    echo -e " ${RED}✗ 失败${NC}"
    FAILED=$((FAILED + 1))
fi

# 测试Go代码
echo ""
echo -e "${YELLOW}=== Go代码测试 ===${NC}"
echo -n "测试 Server Panel go vet..."
cd /Users/ricardo/vibeitem/frp/server-panel
if go vet ./... > /dev/null 2>&1; then
    echo -e " ${GREEN}✓ 通过${NC}"
    PASSED=$((PASSED + 1))
else
    echo -e " ${RED}✗ 失败${NC}"
    FAILED=$((FAILED + 1))
fi

echo -n "测试 Client Panel go vet..."
cd /Users/ricardo/vibeitem/frp/client-panel
if go vet ./... > /dev/null 2>&1; then
    echo -e " ${GREEN}✓ 通过${NC}"
    PASSED=$((PASSED + 1))
else
    echo -e " ${RED}✗ 失败${NC}"
    FAILED=$((FAILED + 1))
fi

# 运行单元测试
echo ""
echo -e "${YELLOW}=== 单元测试 ===${NC}"
echo -n "测试 Server Panel 单元测试..."
cd /Users/ricardo/vibeitem/frp/server-panel
if go test ./internal/crypto/ -v > /dev/null 2>&1; then
    echo -e " ${GREEN}✓ 通过${NC}"
    PASSED=$((PASSED + 1))
else
    echo -e " ${RED}✗ 失败${NC}"
    FAILED=$((FAILED + 1))
fi

echo -n "测试 Client Panel 单元测试..."
cd /Users/ricardo/vibeitem/frp/client-panel
if go test ./internal/hmacsigner/ -v > /dev/null 2>&1; then
    echo -e " ${GREEN}✓ 通过${NC}"
    PASSED=$((PASSED + 1))
else
    echo -e " ${RED}✗ 失败${NC}"
    FAILED=$((FAILED + 1))
fi

# 总结
echo ""
echo -e "${GREEN}=== 测试总结 ===${NC}"
echo -e "通过: ${GREEN}$PASSED${NC}"
echo -e "失败: ${RED}$FAILED${NC}"

if [ $FAILED -gt 0 ]; then
    echo -e "\n${RED}存在失败的测试！${NC}"
    exit 1
else
    echo -e "\n${GREEN}所有测试通过！${NC}"
    exit 0
fi
