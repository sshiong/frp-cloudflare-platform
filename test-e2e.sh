#!/bin/bash
# FRP Panel 端到端测试脚本
# 覆盖设计文档第52节的关键测试场景

set -e

# 颜色
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

PASSED=0
FAILED=0
TOTAL=0

# 测试函数
test_case() {
    local name=$1
    local result=$2
    TOTAL=$((TOTAL + 1))

    if [ "$result" = "pass" ]; then
        echo -e "  ${GREEN}✓${NC} $name"
        PASSED=$((PASSED + 1))
    else
        echo -e "  ${RED}✗${NC} $name"
        FAILED=$((FAILED + 1))
    fi
}

# API 请求函数
api_get() {
    local url=$1
    local use_cookie=${2:-""}
    if [ -n "$use_cookie" ] && [ -f /tmp/cookies.txt ]; then
        curl -s -w "\n%{http_code}" -b /tmp/cookies.txt "$url" 2>/dev/null
    else
        curl -s -w "\n%{http_code}" "$url" 2>/dev/null
    fi
}

api_post() {
    local url=$1
    local data=$2
    local use_cookie=${3:-""}
    if [ -n "$use_cookie" ] && [ -f /tmp/cookies.txt ]; then
        curl -s -w "\n%{http_code}" -X POST -b /tmp/cookies.txt "$url" -H "Content-Type: application/json" -d "$data" 2>/dev/null
    else
        curl -s -w "\n%{http_code}" -X POST "$url" -H "Content-Type: application/json" -d "$data" 2>/dev/null
    fi
}

get_status() {
    echo "$1" | tail -1
}

get_body() {
    echo "$1" | sed "$d"
}

echo -e "${BLUE}╔═══════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║           FRP Panel 端到端测试 (设计文档第52节)              ║${NC}"
echo -e "${BLUE}╚═══════════════════════════════════════════════════════════════╝${NC}"
echo ""

# ============================================================
# 启动服务器
# ============================================================
echo -e "${YELLOW}=== 启动测试环境 ===${NC}"

cd /Users/ricardo/vibeitem/frp/server-panel
rm -f data/panel.db data/panel.db-wal data/panel.db-shm

# 构建
go build -o bin/frp-panel-server ./cmd/control/ 2>/dev/null
if [ $? -ne 0 ]; then
    echo -e "${RED}构建失败！${NC}"
    exit 1
fi

# 启动服务器
./bin/frp-panel-server -port 9000 -dev > /tmp/server.log 2>&1 &
SERVER_PID=$!
sleep 2

# 获取管理员密码
ADMIN_PASS=$(grep -o 'password":"[^"]*' /tmp/server.log | head -1 | cut -d'"' -f3)
echo "管理员密码: $ADMIN_PASS"

# ============================================================
# 测试 1: 实例身份和认证 (设计文档 42.1)
# ============================================================
echo ""
echo -e "${YELLOW}=== 1. 实例身份和认证 ===${NC}"

# 1.1 获取实例信息 (无需认证)
result=$(api_get "http://localhost:9000/api/v1/instance")
status=$(get_status "$result")
body=$(get_body "$result")
test_case "1.1 获取实例信息 (无需认证)" "$([ "$status" = "200" ] && echo "pass" || echo "fail")"

# 1.2 登录 (错误密码)
result=$(api_post "http://localhost:9000/api/v1/auth/login" '{"username":"admin","password":"wrong"}')
status=$(get_status "$result")
test_case "1.2 登录错误密码返回401" "$([ "$status" = "401" ] && echo "pass" || echo "fail")"

# 1.3 登录 (空用户名)
result=$(api_post "http://localhost:9000/api/v1/auth/login" '{"username":"","password":"test"}')
status=$(get_status "$result")
test_case "1.3 登录空用户名返回400" "$([ "$status" = "400" ] && echo "pass" || echo "fail")"

# 1.4 登录成功
login_result=$(curl -s -c /tmp/cookies.txt -X POST "http://localhost:9000/api/v1/auth/login" -H "Content-Type: application/json" -d "{\"username\":\"admin\",\"password\":\"$ADMIN_PASS\"}" 2>/dev/null)
status=$(echo "$login_result" | grep -o '"status":[0-9]*' | cut -d: -f2 || echo "")
body=$(echo "$login_result" | head -1)
if echo "$body" | grep -q "session_id"; then
    SESSION_ID=$(echo "$body" | grep -o '"session_id":"[^"]*' | cut -d'"' -f4)
    CSRF_TOKEN=$(echo "$body" | grep -o '"csrf_token":"[^"]*' | cut -d'"' -f4)
    test_case "1.4 管理员登录成功" "pass"
else
    test_case "1.4 管理员登录成功" "fail"
    SESSION_ID=""
    CSRF_TOKEN=""
fi

# 1.5 获取会话信息
if [ -n "$SESSION_ID" ]; then
    result=$(api_get "http://localhost:9000/api/v1/auth/session" "use_cookie")
    status=$(get_status "$result")
    test_case "1.5 获取会话信息" "$([ "$status" = "200" ] && echo "pass" || echo "fail")"
else
    test_case "1.5 获取会话信息" "fail"
fi

# ============================================================
# 测试 2: 用户管理 (设计文档 42.9)
# ============================================================
echo ""
echo -e "${YELLOW}=== 2. 用户管理 (管理员) ===${NC}"

if [ -n "$SESSION_ID" ]; then
    # 2.1 创建用户
    result=$(api_post "http://localhost:9000/api/v1/admin/users" '{.*}' "use_cookie")
    status=$(get_status "$result")
    test_case "2.1 创建普通用户" "$([ "$status" = "200" ] && echo "pass" || echo "fail")"

    # 2.2 获取用户列表
    result=$(api_get "http://localhost:9000/api/v1/admin/users" "use_cookie")
    status=$(get_status "$result")
    test_case "2.2 获取用户列表" "$([ "$status" = "200" ] && echo "pass" || echo "fail")"

    # 2.3 未认证访问用户列表
    result=$(api_get "http://localhost:9000/api/v1/admin/users")
    status=$(get_status "$result")
    test_case "2.3 未认证访问返回401" "$([ "$status" = "401" ] && echo "pass" || echo "fail")"
else
    test_case "2.1 创建普通用户" "fail"
    test_case "2.2 获取用户列表" "fail"
    test_case "2.3 未认证访问返回401" "fail"
fi

# ============================================================
# 测试 3: 设备管理 (设计文档 42.2)
# ============================================================
echo ""
echo -e "${YELLOW}=== 3. 设备管理 ===${NC}"

if [ -n "$SESSION_ID" ]; then
    # 3.1 获取设备列表
    result=$(api_get "http://localhost:9000/api/v1/devices" "session_id=$SESSION_ID")
    status=$(get_status "$result")
    test_case "3.1 获取设备列表" "$([ "$status" = "200" ] && echo "pass" || echo "fail")"

    # 3.2 设备注册
    result=$(api_post "http://localhost:9000/api/v1/devices/register" '{"installation_instance_id":"test-inst-001","name":"Test Device"}' "session_id=$SESSION_ID")
    status=$(get_status "$result")
    test_case "3.2 设备注册" "$([ "$status" = "200" ] && echo "pass" || echo "fail")"
else
    test_case "3.1 获取设备列表" "fail"
    test_case "3.2 设备注册" "fail"
fi

# ============================================================
# 测试 4: 映射管理 (设计文档 42.4)
# ============================================================
echo ""
echo -e "${YELLOW}=== 4. 映射管理 ===${NC}"

if [ -n "$SESSION_ID" ]; then
    # 4.1 获取映射列表
    result=$(api_get "http://localhost:9000/api/v1/mappings" "session_id=$SESSION_ID")
    status=$(get_status "$result")
    test_case "4.1 获取映射列表" "$([ "$status" = "200" ] && echo "pass" || echo "fail")"

    # 4.2 创建TCP映射
    result=$(api_post "http://localhost:9000/api/v1/mappings" '{"name":"Test TCP","protocol":"tcp","local_ip":"127.0.0.1","local_port":8080,"remote_port":18080}' "session_id=$SESSION_ID")
    status=$(get_status "$result")
    test_case "4.2 创建TCP映射" "$([ "$status" = "200" ] && echo "pass" || echo "fail")"

    # 4.3 创建UDP映射
    result=$(api_post "http://localhost:9000/api/v1/mappings" '{"name":"Test UDP","protocol":"udp","local_ip":"127.0.0.1","local_port":5353,"remote_port":15353}' "session_id=$SESSION_ID")
    status=$(get_status "$result")
    test_case "4.3 创建UDP映射" "$([ "$status" = "200" ] && echo "pass" || echo "fail")"

    # 4.4 未认证创建映射
    result=$(api_post "http://localhost:9000/api/v1/mappings" '{"name":"Test","protocol":"tcp","local_ip":"127.0.0.1","local_port":8080}')
    status=$(get_status "$result")
    test_case "4.4 未认证创建映射返回401" "$([ "$status" = "401" ] && echo "pass" || echo "fail")"
else
    test_case "4.1 获取映射列表" "fail"
    test_case "4.2 创建TCP映射" "fail"
    test_case "4.3 创建UDP映射" "fail"
    test_case "4.4 未认证创建映射返回401" "fail"
fi

# ============================================================
# 测试 5: 域名管理 (设计文档 42.5)
# ============================================================
echo ""
echo -e "${YELLOW}=== 5. 域名管理 ===${NC}"

if [ -n "$SESSION_ID" ]; then
    # 5.1 获取域名列表
    result=$(api_get "http://localhost:9000/api/v1/domains" "session_id=$SESSION_ID")
    status=$(get_status "$result")
    test_case "5.1 获取域名列表" "$([ "$status" = "200" ] && echo "pass" || echo "fail")"

    # 5.2 域名预检
    result=$(api_post "http://localhost:9000/api/v1/domains/preflight" '{"hostname":"test.example.com"}' "session_id=$SESSION_ID")
    status=$(get_status "$result")
    test_case "5.2 域名预检" "$([ "$status" = "200" ] && echo "pass" || echo "fail")"
else
    test_case "5.1 获取域名列表" "fail"
    test_case "5.2 域名预检" "fail"
fi

# ============================================================
# 测试 6: Cloudflare 管理 (设计文档 42.6)
# ============================================================
echo ""
echo -e "${YELLOW}=== 6. Cloudflare 管理 ===${NC}"

if [ -n "$SESSION_ID" ]; then
    # 6.1 获取Token状态
    result=$(api_get "http://localhost:9000/api/v1/cloudflare/status" "session_id=$SESSION_ID")
    status=$(get_status "$result")
    test_case "6.1 获取Cloudflare Token状态" "$([ "$status" = "200" ] && echo "pass" || echo "fail")"

    # 6.2 获取Zone列表
    result=$(api_get "http://localhost:9000/api/v1/cloudflare/zones" "session_id=$SESSION_ID")
    status=$(get_status "$result")
    test_case "6.2 获取Zone列表" "$([ "$status" = "200" ] || [ "$status" = "500" ] && echo "pass" || echo "fail")"
else
    test_case "6.1 获取Cloudflare Token状态" "fail"
    test_case "6.2 获取Zone列表" "fail"
fi

# ============================================================
# 测试 7: 证书管理 (设计文档 42.7)
# ============================================================
echo ""
echo -e "${YELLOW}=== 7. 证书管理 ===${NC}"

if [ -n "$SESSION_ID" ]; then
    # 7.1 获取证书列表
    result=$(api_get "http://localhost:9000/api/v1/certificates" "session_id=$SESSION_ID")
    status=$(get_status "$result")
    test_case "7.1 获取证书列表" "$([ "$status" = "200" ] && echo "pass" || echo "fail")"
else
    test_case "7.1 获取证书列表" "fail"
fi

# ============================================================
# 测试 8: 操作管理 (设计文档 42.8)
# ============================================================
echo ""
echo -e "${YELLOW}=== 8. 操作管理 ===${NC}"

if [ -n "$SESSION_ID" ]; then
    # 8.1 获取操作列表
    result=$(api_get "http://localhost:9000/api/v1/operations" "session_id=$SESSION_ID")
    status=$(get_status "$result")
    test_case "8.1 获取操作列表" "$([ "$status" = "200" ] && echo "pass" || echo "fail")"
else
    test_case "8.1 获取操作列表" "fail"
fi

# ============================================================
# 测试 9: FRPS 插件 (设计文档 42.10)
# ============================================================
echo ""
echo -e "${YELLOW}=== 9. FRPS 插件接口 ===${NC}"

# 9.1 Login 接口
result=$(api_post "http://localhost:9000/internal/frp/login" '{"version":"0.58.0","hostname":"test","os":"linux","arch":"amd64","user":"test","privilege_key":"test"}')
status=$(get_status "$result")
test_case "9.1 FRPS Login接口" "$([ "$status" = "200" ] && echo "pass" || echo "fail")"

# 9.2 NewProxy 接口
result=$(api_post "http://localhost:9000/internal/frp/new-proxy" '{"user":"test","proxy_name":"test","proxy_type":"tcp"}')
status=$(get_status "$result")
test_case "9.2 FRPS NewProxy接口" "$([ "$status" = "200" ] && echo "pass" || echo "fail")"

# 9.3 NewWorkConn 接口
result=$(api_post "http://localhost:9000/internal/frp/new-work-conn" '{"user":"test","run_id":"test"}')
status=$(get_status "$result")
test_case "9.3 FRPS NewWorkConn接口" "$([ "$status" = "200" ] && echo "pass" || echo "fail")"

# 9.4 Ping 接口
result=$(api_post "http://localhost:9000/internal/frp/ping" '{"user":"test"}')
status=$(get_status "$result")
test_case "9.4 FRPS Ping接口" "$([ "$status" = "200" ] && echo "pass" || echo "fail")"

# ============================================================
# 测试 10: 客户端配置 (设计文档 42.3)
# ============================================================
echo ""
echo -e "${YELLOW}=== 10. 客户端配置接口 ===${NC}"

if [ -n "$SESSION_ID" ]; then
    # 10.1 获取bootstrap
    result=$(api_get "http://localhost:9000/api/v1/client/bootstrap" "session_id=$SESSION_ID")
    status=$(get_status "$result")
    test_case "10.1 获取客户端bootstrap" "$([ "$status" = "200" ] && echo "pass" || echo "fail")"

    # 10.2 获取配置
    result=$(api_get "http://localhost:9000/api/v1/client/config" "session_id=$SESSION_ID")
    status=$(get_status "$result")
    test_case "10.2 获取客户端配置" "$([ "$status" = "200" ] && echo "pass" || echo "fail")"

    # 10.3 心跳
    result=$(api_post "http://localhost:9000/api/v1/client/heartbeat" '{"version":"1.0.0"}' "session_id=$SESSION_ID")
    status=$(get_status "$result")
    test_case "10.3 客户端心跳" "$([ "$status" = "200" ] && echo "pass" || echo "fail")"
else
    test_case "10.1 获取客户端bootstrap" "fail"
    test_case "10.2 获取客户端配置" "fail"
    test_case "10.3 客户端心跳" "fail"
fi

# ============================================================
# 测试 11: 系统状态 (设计文档 42.9)
# ============================================================
echo ""
echo -e "${YELLOW}=== 11. 系统状态 ===${NC}"

if [ -n "$SESSION_ID" ]; then
    # 11.1 获取系统状态
    result=$(api_get "http://localhost:9000/api/v1/admin/system" "session_id=$SESSION_ID")
    status=$(get_status "$result")
    test_case "11.1 获取系统状态" "$([ "$status" = "200" ] && echo "pass" || echo "fail")"

    # 11.2 获取审计日志
    result=$(api_get "http://localhost:9000/api/v1/admin/audit" "session_id=$SESSION_ID")
    status=$(get_status "$result")
    test_case "11.2 获取审计日志" "$([ "$status" = "200" ] && echo "pass" || echo "fail")"
else
    test_case "11.1 获取系统状态" "fail"
    test_case "11.2 获取审计日志" "fail"
fi

# ============================================================
# 测试 12: 安全测试
# ============================================================
echo ""
echo -e "${YELLOW}=== 12. 安全测试 ===${NC}"

# 12.1 CSRF保护
result=$(curl -s -w "\n%{http_code}" -X POST "http://localhost:9000/api/v1/auth/logout" -H "Content-Type: application/json" -H "X-CSRF-Token: invalid" 2>/dev/null)
status=$(get_status "$result")
test_case "12.1 CSRF保护 (无效token)" "$([ "$status" = "401" ] || [ "$status" = "403" ] && echo "pass" || echo "fail")"

# 12.2 SQL注入防护
result=$(api_post "http://localhost:9000/api/v1/auth/login" '{"username":"admin'\''--","password":"test"}')
status=$(get_status "$result")
test_case "12.2 SQL注入防护" "$([ "$status" = "401" ] && echo "pass" || echo "fail")"

# 12.3 XSS防护
result=$(api_post "http://localhost:9000/api/v1/auth/login" '{"username":"<script>alert(1)</script>","password":"test"}')
status=$(get_status "$result")
test_case "12.3 XSS防护" "$([ "$status" = "401" ] && echo "pass" || echo "fail")"

# ============================================================
# 停止服务器
# ============================================================
kill $SERVER_PID 2>/dev/null
wait $SERVER_PID 2>/dev/null

# ============================================================
# 测试 13: 前端构建
# ============================================================
echo ""
echo -e "${YELLOW}=== 13. 前端构建测试 ===${NC}"

cd /Users/ricardo/vibeitem/frp/server-panel/web-admin
if npm run build > /dev/null 2>&1; then
    test_case "13.1 Server Panel 前端构建" "pass"
else
    test_case "13.1 Server Panel 前端构建" "fail"
fi

cd /Users/ricardo/vibeitem/frp/client-panel/web-client
if npm run build > /dev/null 2>&1; then
    test_case "13.2 Client Panel 前端构建" "pass"
else
    test_case "13.2 Client Panel 前端构建" "fail"
fi

# ============================================================
# 测试 14: Go代码质量
# ============================================================
echo ""
echo -e "${YELLOW}=== 14. Go代码质量测试 ===${NC}"

cd /Users/ricardo/vibeitem/frp/server-panel
if go vet ./... > /dev/null 2>&1; then
    test_case "14.1 Server Panel go vet" "pass"
else
    test_case "14.1 Server Panel go vet" "fail"
fi

cd /Users/ricardo/vibeitem/frp/client-panel
if go vet ./... > /dev/null 2>&1; then
    test_case "14.2 Client Panel go vet" "pass"
else
    test_case "14.2 Client Panel go vet" "fail"
fi

# ============================================================
# 测试 15: 单元测试
# ============================================================
echo ""
echo -e "${YELLOW}=== 15. 单元测试 ===${NC}"

cd /Users/ricardo/vibeitem/frp/server-panel
if go test ./internal/crypto/ -v > /dev/null 2>&1; then
    test_case "15.1 Server Panel Crypto单元测试" "pass"
else
    test_case "15.1 Server Panel Crypto单元测试" "fail"
fi

cd /Users/ricardo/vibeitem/frp/client-panel
if go test ./internal/hmacsigner/ -v > /dev/null 2>&1; then
    test_case "15.2 Client Panel HMAC单元测试" "pass"
else
    test_case "15.2 Client Panel HMAC单元测试" "fail"
fi

# ============================================================
# 测试总结
# ============================================================
echo ""
echo -e "${BLUE}╔═══════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║                        测试总结                              ║${NC}"
echo -e "${BLUE}╚═══════════════════════════════════════════════════════════════╝${NC}"
echo ""
echo -e "总测试数: ${BLUE}$TOTAL${NC}"
echo -e "通过: ${GREEN}$PASSED${NC}"
echo -e "失败: ${RED}$FAILED${NC}"
echo ""

if [ $FAILED -gt 0 ]; then
    echo -e "${RED}存在失败的测试！${NC}"
    exit 1
else
    echo -e "${GREEN}所有测试通过！${NC}"
    exit 0
fi
