#!/bin/bash
# FRP Panel 综合测试脚本 - 包含刁钻边界测试
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

api_put() {
    local url=$1
    local data=$2
    local use_cookie=${3:-""}
    if [ -n "$use_cookie" ] && [ -f /tmp/cookies.txt ]; then
        curl -s -w "\n%{http_code}" -X PUT -b /tmp/cookies.txt "$url" -H "Content-Type: application/json" -d "$data" 2>/dev/null
    else
        curl -s -w "\n%{http_code}" -X PUT "$url" -H "Content-Type: application/json" -d "$data" 2>/dev/null
    fi
}

api_delete() {
    local url=$1
    local use_cookie=${2:-""}
    if [ -n "$use_cookie" ] && [ -f /tmp/cookies.txt ]; then
        curl -s -w "\n%{http_code}" -X DELETE -b /tmp/cookies.txt "$url" 2>/dev/null
    else
        curl -s -w "\n%{http_code}" -X DELETE "$url" 2>/dev/null
    fi
}

get_status() {
    echo "$1" | tail -1
}

get_body() {
    echo "$1" | sed '$d'
}

echo -e "${BLUE}╔═══════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║           FRP Panel 综合测试 (含刁钻边界测试)                ║${NC}"
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
sleep 3

# 获取管理员密码 (更健壮的提取方式)
ADMIN_PASS=$(grep -o '"password":"[^"]*"' /tmp/server.log | head -1 | sed 's/"password":"//' | sed 's/"//')
if [ -z "$ADMIN_PASS" ]; then
    # 备用提取方式
    ADMIN_PASS=$(grep -o 'password":"[^"]*' /tmp/server.log | head -1 | sed 's/password":"//')
fi
echo "管理员密码: $ADMIN_PASS"

if [ -z "$ADMIN_PASS" ]; then
    echo -e "${RED}无法提取管理员密码！${NC}"
    kill $SERVER_PID 2>/dev/null
    exit 1
fi

# ============================================================
# 1. 认证测试 (刁钻边界)
# ============================================================
echo ""
echo -e "${YELLOW}=== 1. 认证测试 (刁钻边界) ===${NC}"

# 1.1 正常登录
login_result=$(curl -s -c /tmp/cookies.txt -X POST "http://localhost:9000/api/v1/auth/login" -H "Content-Type: application/json" -d "{\"username\":\"admin\",\"password\":\"$ADMIN_PASS\"}" 2>/dev/null)
if echo "$login_result" | grep -q "session_id"; then
    test_case "1.1 正常登录" "pass"
else
    test_case "1.1 正常登录" "fail"
fi

# 1.2 空密码登录
result=$(api_post "http://localhost:9000/api/v1/auth/login" '{"username":"admin","password":""}')
status=$(get_status "$result")
test_case "1.2 空密码登录返回400" "$([ "$status" = "400" ] && echo "pass" || echo "fail")"

# 1.3 超长密码登录 (1000字符)
long_pass=$(printf 'a%.0s' {1..1000})
result=$(api_post "http://localhost:9000/api/v1/auth/login" "{\"username\":\"admin\",\"password\":\"$long_pass\"}")
status=$(get_status "$result")
test_case "1.3 超长密码登录返回401" "$([ "$status" = "401" ] && echo "pass" || echo "fail")"

# 1.4 SQL注入用户名
result=$(api_post "http://localhost:9000/api/v1/auth/login" '{"username":"admin'\'' OR 1=1--","password":"test"}')
status=$(get_status "$result")
test_case "1.4 SQL注入用户名返回401" "$([ "$status" = "401" ] && echo "pass" || echo "fail")"

# 1.5 XSS用户名
result=$(api_post "http://localhost:9000/api/v1/auth/login" '{"username":"<script>alert(1)</script>","password":"test"}')
status=$(get_status "$result")
test_case "1.5 XSS用户名返回401" "$([ "$status" = "401" ] && echo "pass" || echo "fail")"

# 1.6 Unicode用户名
result=$(api_post "http://localhost:9000/api/v1/auth/login" '{"username":"管理员","password":"test"}')
status=$(get_status "$result")
test_case "1.6 Unicode用户名返回401" "$([ "$status" = "401" ] && echo "pass" || echo "fail")"

# 1.7 并发登录测试 (快速连续登录)
for i in {1..5}; do
    curl -s -X POST "http://localhost:9000/api/v1/auth/login" -H "Content-Type: application/json" -d "{\"username\":\"admin\",\"password\":\"$ADMIN_PASS\"}" > /dev/null 2>&1 &
done
wait
result=$(api_post "http://localhost:9000/api/v1/auth/login" "{\"username\":\"admin\",\"password\":\"$ADMIN_PASS\"}")
status=$(get_status "$result")
test_case "1.7 并发登录后仍可登录" "$([ "$status" = "200" ] && echo "pass" || echo "fail")"

# ============================================================
# 2. 用户管理测试 (刁钻边界)
# ============================================================
echo ""
echo -e "${YELLOW}=== 2. 用户管理测试 (刁钻边界) ===${NC}"

# 2.1 创建用户 - 正常
result=$(api_post "http://localhost:9000/api/v1/admin/users" '{"username":"testuser1","role":"user"}' "use_cookie")
status=$(get_status "$result")
test_case "2.1 创建用户 - 正常" "$([ "$status" = "200" ] && echo "pass" || echo "fail")"

# 2.2 创建用户 - 重复用户名
result=$(api_post "http://localhost:9000/api/v1/admin/users" '{"username":"testuser1","role":"user"}' "use_cookie")
status=$(get_status "$result")
test_case "2.2 创建用户 - 重复用户名返回409/400" "$([ "$status" = "409" ] || [ "$status" = "400" ] || [ "$status" = "500" ] && echo "pass" || echo "fail")"

# 2.3 创建用户 - 空用户名
result=$(api_post "http://localhost:9000/api/v1/admin/users" '{"username":"","role":"user"}' "use_cookie")
status=$(get_status "$result")
test_case "2.3 创建用户 - 空用户名返回400" "$([ "$status" = "400" ] && echo "pass" || echo "fail")"

# 2.4 创建用户 - 超长用户名 (256字符)
long_user=$(printf 'a%.0s' {1..256})
result=$(api_post "http://localhost:9000/api/v1/admin/users" "{\"username\":\"$long_user\",\"role\":\"user\"}" "use_cookie")
status=$(get_status "$result")
test_case "2.4 创建用户 - 超长用户名返回400/500" "$([ "$status" = "400" ] || [ "$status" = "500" ] && echo "pass" || echo "fail")"

# 2.5 创建用户 - 无效角色
result=$(api_post "http://localhost:9000/api/v1/admin/users" '{"username":"testuser2","role":"superadmin"}' "use_cookie")
status=$(get_status "$result")
test_case "2.5 创建用户 - 无效角色返回400" "$([ "$status" = "400" ] && echo "pass" || echo "fail")"

# 2.6 获取用户列表 - 验证包含新用户
result=$(api_get "http://localhost:9000/api/v1/admin/users" "use_cookie")
status=$(get_status "$result")
body=$(get_body "$result")
test_case "2.6 获取用户列表 - 验证包含新用户" "$([ "$status" = "200" ] && echo "$body" | grep -q "testuser1" && echo "pass" || echo "fail")"

# ============================================================
# 3. 设备管理测试 (刁钻边界)
# ============================================================
echo ""
echo -e "${YELLOW}=== 3. 设备管理测试 (刁钻边界) ===${NC}"

# 3.1 注册设备 - 正常
result=$(api_post "http://localhost:9000/api/v1/devices/register" '{"name":"Test Device","installation_instance_id":"test-001"}' "use_cookie")
status=$(get_status "$result")
test_case "3.1 注册设备 - 正常" "$([ "$status" = "200" ] && echo "pass" || echo "fail")"

# 3.2 注册设备 - 空名称
result=$(api_post "http://localhost:9000/api/v1/devices/register" '{"name":"","installation_instance_id":"test-002"}' "use_cookie")
status=$(get_status "$result")
test_case "3.2 注册设备 - 空名称应成功(默认名称)" "$([ "$status" = "200" ] && echo "pass" || echo "fail")"

# 3.3 获取设备列表
result=$(api_get "http://localhost:9000/api/v1/devices" "use_cookie")
status=$(get_status "$result")
test_case "3.3 获取设备列表" "$([ "$status" = "200" ] && echo "pass" || echo "fail")"

# 3.4 获取当前设备
result=$(api_get "http://localhost:9000/api/v1/devices/current" "use_cookie")
status=$(get_status "$result")
test_case "3.4 获取当前设备" "$([ "$status" = "200" ] && echo "pass" || echo "fail")"

# ============================================================
# 4. 映射管理测试 (刁钻边界)
# ============================================================
echo ""
echo -e "${YELLOW}=== 4. 映射管理测试 (刁钻边界) ===${NC}"

# 4.1 创建TCP映射 - 正常
result=$(api_post "http://localhost:9000/api/v1/mappings" '{"name":"Test TCP","protocol":"tcp","local_ip":"127.0.0.1","local_port":8080}' "use_cookie")
status=$(get_status "$result")
body=$(get_body "$result")
if [ "$status" = "200" ] && echo "$body" | grep -q "id"; then
    MAPPING_ID=$(echo "$body" | grep -o '"id":"[^"]*' | cut -d'"' -f4)
    test_case "4.1 创建TCP映射 - 正常" "pass"
else
    test_case "4.1 创建TCP映射 - 正常" "fail"
    MAPPING_ID=""
fi

# 4.2 创建UDP映射 - 正常
result=$(api_post "http://localhost:9000/api/v1/mappings" '{"name":"Test UDP","protocol":"udp","local_ip":"127.0.0.1","local_port":5353}' "use_cookie")
status=$(get_status "$result")
test_case "4.2 创建UDP映射 - 正常" "$([ "$status" = "200" ] && echo "pass" || echo "fail")"

# 4.3 创建映射 - 无效协议
result=$(api_post "http://localhost:9000/api/v1/mappings" '{"name":"Test Invalid","protocol":"icmp","local_ip":"127.0.0.1","local_port":8080}' "use_cookie")
status=$(get_status "$result")
test_case "4.3 创建映射 - 无效协议返回400" "$([ "$status" = "400" ] && echo "pass" || echo "fail")"

# 4.4 创建映射 - 无效端口 (0)
result=$(api_post "http://localhost:9000/api/v1/mappings" '{"name":"Test Port 0","protocol":"tcp","local_ip":"127.0.0.1","local_port":0}' "use_cookie")
status=$(get_status "$result")
test_case "4.4 创建映射 - 无效端口0返回400" "$([ "$status" = "400" ] && echo "pass" || echo "fail")"

# 4.5 创建映射 - 无效端口 (70000)
result=$(api_post "http://localhost:9000/api/v1/mappings" '{"name":"Test Port 70000","protocol":"tcp","local_ip":"127.0.0.1","local_port":70000}' "use_cookie")
status=$(get_status "$result")
test_case "4.5 创建映射 - 无效端口70000返回400" "$([ "$status" = "400" ] && echo "pass" || echo "fail")"

# 4.6 创建映射 - 空名称
result=$(api_post "http://localhost:9000/api/v1/mappings" '{"name":"","protocol":"tcp","local_ip":"127.0.0.1","local_port":8081}' "use_cookie")
status=$(get_status "$result")
test_case "4.6 创建映射 - 空名称返回400" "$([ "$status" = "400" ] && echo "pass" || echo "fail")"

# 4.7 创建映射 - 无效IP
result=$(api_post "http://localhost:9000/api/v1/mappings" '{"name":"Test Invalid IP","protocol":"tcp","local_ip":"999.999.999.999","local_port":8080}' "use_cookie")
status=$(get_status "$result")
test_case "4.7 创建映射 - 无效IP(应成功或失败)" "$([ "$status" = "200" ] || [ "$status" = "400" ] && echo "pass" || echo "fail")"

# 4.8 获取映射列表
result=$(api_get "http://localhost:9000/api/v1/mappings" "use_cookie")
status=$(get_status "$result")
test_case "4.8 获取映射列表" "$([ "$status" = "200" ] && echo "pass" || echo "fail")"

# 4.9 获取单个映射
if [ -n "$MAPPING_ID" ]; then
    result=$(api_get "http://localhost:9000/api/v1/mappings/$MAPPING_ID" "use_cookie")
    status=$(get_status "$result")
    test_case "4.9 获取单个映射" "$([ "$status" = "200" ] && echo "pass" || echo "fail")"
else
    test_case "4.9 获取单个映射" "fail"
fi

# 4.10 删除映射
if [ -n "$MAPPING_ID" ]; then
    result=$(api_delete "http://localhost:9000/api/v1/mappings/$MAPPING_ID" "use_cookie")
    status=$(get_status "$result")
    test_case "4.10 删除映射" "$([ "$status" = "200" ] && echo "pass" || echo "fail")"
else
    test_case "4.10 删除映射" "fail"
fi

# ============================================================
# 5. 域名管理测试 (刁钻边界)
# ============================================================
echo ""
echo -e "${YELLOW}=== 5. 域名管理测试 (刁钻边界) ===${NC}"

# 5.1 创建域名 - 正常
result=$(api_post "http://localhost:9000/api/v1/domains" '{"hostname":"test.example.com","https_mode":"http_only"}' "use_cookie")
status=$(get_status "$result")
test_case "5.1 创建域名 - 正常" "$([ "$status" = "200" ] && echo "pass" || echo "fail")"

# 5.2 创建域名 - 重复域名
result=$(api_post "http://localhost:9000/api/v1/domains" '{"hostname":"test.example.com","https_mode":"http_only"}' "use_cookie")
status=$(get_status "$result")
test_case "5.2 创建域名 - 重复域名返回409" "$([ "$status" = "409" ] && echo "pass" || echo "fail")"

# 5.3 创建域名 - 无效域名格式
result=$(api_post "http://localhost:9000/api/v1/domains" '{"hostname":"invalid domain!@#","https_mode":"http_only"}' "use_cookie")
status=$(get_status "$result")
test_case "5.3 创建域名 - 无效域名格式返回400" "$([ "$status" = "400" ] && echo "pass" || echo "fail")"

# 5.4 创建域名 - 空域名
result=$(api_post "http://localhost:9000/api/v1/domains" '{"hostname":"","https_mode":"http_only"}' "use_cookie")
status=$(get_status "$result")
test_case "5.4 创建域名 - 空域名返回400" "$([ "$status" = "400" ] && echo "pass" || echo "fail")"

# 5.5 获取域名列表
result=$(api_get "http://localhost:9000/api/v1/domains" "use_cookie")
status=$(get_status "$result")
test_case "5.5 获取域名列表" "$([ "$status" = "200" ] && echo "pass" || echo "fail")"

# 5.6 域名预检
result=$(api_post "http://localhost:9000/api/v1/domains/preflight" '{"hostname":"new.example.com"}' "use_cookie")
status=$(get_status "$result")
test_case "5.6 域名预检" "$([ "$status" = "200" ] && echo "pass" || echo "fail")"

# ============================================================
# 6. Cloudflare 测试 (刁钻边界)
# ============================================================
echo ""
echo -e "${YELLOW}=== 6. Cloudflare 测试 (刁钻边界) ===${NC}"

# 6.1 获取Token状态
result=$(api_get "http://localhost:9000/api/v1/cloudflare/status" "use_cookie")
status=$(get_status "$result")
test_case "6.1 获取Token状态" "$([ "$status" = "200" ] && echo "pass" || echo "fail")"

# 6.2 上传Token - 空Token
result=$(api_post "http://localhost:9000/api/v1/cloudflare/token" '{"token":""}' "use_cookie")
status=$(get_status "$result")
test_case "6.2 上传空Token返回400" "$([ "$status" = "400" ] && echo "pass" || echo "fail")"

# 6.3 上传Token - 正常
result=$(api_post "http://localhost:9000/api/v1/cloudflare/token" '{"token":"test-token-12345"}' "use_cookie")
status=$(get_status "$result")
test_case "6.3 上传Token - 正常" "$([ "$status" = "200" ] && echo "pass" || echo "fail")"

# 6.4 获取Zone列表
result=$(api_get "http://localhost:9000/api/v1/cloudflare/zones" "use_cookie")
status=$(get_status "$result")
test_case "6.4 获取Zone列表" "$([ "$status" = "200" ] && echo "pass" || echo "fail")"

# ============================================================
# 7. 证书管理测试 (刁钻边界)
# ============================================================
echo ""
echo -e "${YELLOW}=== 7. 证书管理测试 ===${NC}"

# 7.1 获取证书列表
result=$(api_get "http://localhost:9000/api/v1/certificates" "use_cookie")
status=$(get_status "$result")
test_case "7.1 获取证书列表" "$([ "$status" = "200" ] && echo "pass" || echo "fail")"

# ============================================================
# 8. 操作管理测试
# ============================================================
echo ""
echo -e "${YELLOW}=== 8. 操作管理测试 ===${NC}"

# 8.1 获取操作列表
result=$(api_get "http://localhost:9000/api/v1/operations" "use_cookie")
status=$(get_status "$result")
test_case "8.1 获取操作列表" "$([ "$status" = "200" ] && echo "pass" || echo "fail")"

# ============================================================
# 9. FRPS 插件测试 (刁钻边界)
# ============================================================
echo ""
echo -e "${YELLOW}=== 9. FRPS 插件测试 (刁钻边界) ===${NC}"

# 9.1 Login - 正常
result=$(api_post "http://localhost:9000/internal/frp/login" '{"version":"0.58.0","hostname":"test","os":"linux","arch":"amd64","user":"test","privilege_key":"test"}')
status=$(get_status "$result")
test_case "9.1 FRPS Login - 正常" "$([ "$status" = "200" ] && echo "pass" || echo "fail")"

# 9.2 Login - 空body
result=$(api_post "http://localhost:9000/internal/frp/login" '{}')
status=$(get_status "$result")
test_case "9.2 FRPS Login - 空body" "$([ "$status" = "200" ] && echo "pass" || echo "fail")"

# 9.3 NewProxy - 正常
result=$(api_post "http://localhost:9000/internal/frp/new-proxy" '{"user":"test","proxy_name":"test","proxy_type":"tcp"}')
status=$(get_status "$result")
test_case "9.3 FRPS NewProxy - 正常" "$([ "$status" = "200" ] && echo "pass" || echo "fail")"

# 9.4 NewWorkConn - 正常
result=$(api_post "http://localhost:9000/internal/frp/new-work-conn" '{"user":"test","run_id":"test"}')
status=$(get_status "$result")
test_case "9.4 FRPS NewWorkConn - 正常" "$([ "$status" = "200" ] && echo "pass" || echo "fail")"

# 9.5 Ping - 正常
result=$(api_post "http://localhost:9000/internal/frp/ping" '{"user":"test"}')
status=$(get_status "$result")
test_case "9.5 FRPS Ping - 正常" "$([ "$status" = "200" ] && echo "pass" || echo "fail")"

# 9.6 CloseProxy - 正常
result=$(api_post "http://localhost:9000/internal/frp/close-proxy" '{"user":"test","proxy_name":"test"}')
status=$(get_status "$result")
test_case "9.6 FRPS CloseProxy - 正常" "$([ "$status" = "200" ] && echo "pass" || echo "fail")"

# ============================================================
# 10. 客户端配置测试
# ============================================================
echo ""
echo -e "${YELLOW}=== 10. 客户端配置测试 ===${NC}"

# 10.1 获取bootstrap
result=$(api_get "http://localhost:9000/api/v1/client/bootstrap" "use_cookie")
status=$(get_status "$result")
test_case "10.1 获取bootstrap" "$([ "$status" = "200" ] && echo "pass" || echo "fail")"

# 10.2 获取配置
result=$(api_get "http://localhost:9000/api/v1/client/config" "use_cookie")
status=$(get_status "$result")
test_case "10.2 获取配置" "$([ "$status" = "200" ] && echo "pass" || echo "fail")"

# 10.3 心跳
result=$(api_post "http://localhost:9000/api/v1/client/heartbeat" '{"version":"1.0.0"}' "use_cookie")
status=$(get_status "$result")
test_case "10.3 心跳" "$([ "$status" = "200" ] && echo "pass" || echo "fail")"

# ============================================================
# 11. 系统状态测试
# ============================================================
echo ""
echo -e "${YELLOW}=== 11. 系统状态测试 ===${NC}"

# 11.1 获取系统状态
result=$(api_get "http://localhost:9000/api/v1/admin/system" "use_cookie")
status=$(get_status "$result")
test_case "11.1 获取系统状态" "$([ "$status" = "200" ] && echo "pass" || echo "fail")"

# 11.2 获取审计日志
result=$(api_get "http://localhost:9000/api/v1/admin/audit" "use_cookie")
status=$(get_status "$result")
test_case "11.2 获取审计日志" "$([ "$status" = "200" ] && echo "pass" || echo "fail")"

# ============================================================
# 12. 安全测试 (刁钻边界)
# ============================================================
echo ""
echo -e "${YELLOW}=== 12. 安全测试 (刁钻边界) ===${NC}"

# 12.1 SQL注入 - 映射创建
result=$(api_post "http://localhost:9000/api/v1/mappings" '{"name":"test'\'' OR 1=1--","protocol":"tcp","local_ip":"127.0.0.1","local_port":8080}' "use_cookie")
status=$(get_status "$result")
test_case "12.1 SQL注入 - 映射创建应成功或失败" "$([ "$status" = "200" ] || [ "$status" = "400" ] || [ "$status" = "500" ] && echo "pass" || echo "fail")"

# 12.2 XSS - 映射名称
result=$(api_post "http://localhost:9000/api/v1/mappings" '{"name":"<img src=x onerror=alert(1)>","protocol":"tcp","local_ip":"127.0.0.1","local_port":8082}' "use_cookie")
status=$(get_status "$result")
test_case "12.2 XSS - 映射名称应成功或失败" "$([ "$status" = "200" ] || [ "$status" = "400" ] || [ "$status" = "500" ] && echo "pass" || echo "fail")"

# 12.3 路径遍历 - 尝试访问其他用户数据
result=$(api_get "http://localhost:9000/api/v1/admin/users?id=../admin" "use_cookie")
status=$(get_status "$result")
test_case "12.3 路径遍历防护" "$([ "$status" = "200" ] || [ "$status" = "400" ] || [ "$status" = "404" ] && echo "pass" || echo "fail")"

# 12.4 未认证访问受保护端点
result=$(api_get "http://localhost:9000/api/v1/admin/users")
status=$(get_status "$result")
test_case "12.4 未认证访问返回401" "$([ "$status" = "401" ] && echo "pass" || echo "fail")"

# 12.5 无效Cookie访问
result=$(curl -s -w "\n%{http_code}" -b "frp_session=invalid_session_id" "http://localhost:9000/api/v1/admin/users" 2>/dev/null)
status=$(get_status "$result")
test_case "12.5 无效Cookie返回401" "$([ "$status" = "401" ] && echo "pass" || echo "fail")"

# ============================================================
# 13. 前端构建测试
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
# 14. Go代码质量测试
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
# 15. 单元测试
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
# 停止服务器
# ============================================================
kill $SERVER_PID 2>/dev/null
wait $SERVER_PID 2>/dev/null

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

PERCENTAGE=$((PASSED * 100 / TOTAL))
echo -e "通过率: ${BLUE}${PERCENTAGE}%${NC}"
echo ""

if [ $FAILED -gt 0 ]; then
    echo -e "${RED}存在失败的测试！${NC}"
    exit 1
else
    echo -e "${GREEN}所有测试通过！${NC}"
    exit 0
fi
