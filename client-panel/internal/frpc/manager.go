package frpc

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Manager FRPC 进程管理器
// 管理单个 FRPC 实例的生命周期
// 包括：启动、停止、重启、状态查询、Admin API 调用
type Manager struct {
	mu           sync.Mutex
	binaryPath   string        // FRPC 二进制路径
	configDir    string        // 配置目录
	pid          int           // 当前进程 PID
	startTime    time.Time     // 进程启动时间
	binaryHash   string        // 二进制 SHA-256
	cmd          *exec.Cmd     // 当前进程
	adminAddr    string        // Admin API 地址
	adminUser    string        // Admin 用户名
	adminPass    string        // Admin 密码
	logger       *slog.Logger
	ctx          context.Context
	cancel       context.CancelFunc
	status       string        // stopped, starting, running, stopping, error
	lastError    string
}

// NewManager 创建 FRPC 管理器
func NewManager(binaryPath, configDir string, logger *slog.Logger) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		binaryPath: binaryPath,
		configDir:  configDir,
		logger:     logger,
		ctx:        ctx,
		cancel:     cancel,
		status:     "stopped",
	}
}

// Start 启动 FRPC 进程
// 流程：
// 1. 检查是否已有运行实例
// 2. 计算二进制哈希
// 3. 生成 Admin API 随机凭证
// 4. 启动进程（非 shell，独立进程组）
// 5. 等待 Admin API 可用
func (m *Manager) Start(ctx context.Context, configPath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.status == "running" {
		return fmt.Errorf("FRPC 已在运行")
	}

	// 验证二进制文件
	if err := m.verifyBinary(); err != nil {
		return fmt.Errorf("二进制验证失败: %w", err)
	}

	// 生成 Admin API 随机凭证
	adminPort, adminUser, adminPass, err := generateAdminCredentials()
	if err != nil {
		return fmt.Errorf("生成 Admin 凭证失败: %w", err)
	}

	m.adminAddr = fmt.Sprintf("127.0.0.1:%d", adminPort)
	m.adminUser = adminUser
	m.adminPass = adminPass

	// 构造命令参数
	args := []string{
		"-c", configPath,
		"--web_server.addr", "127.0.0.1",
		"--web_server.port", strconv.Itoa(adminPort),
		"--web_server.user", adminUser,
		"--web_server.password", adminPass,
	}

	// 创建命令（非 shell）
	cmd := exec.CommandContext(m.ctx, m.binaryPath, args...)
	cmd.Dir = m.configDir

	// 设置进程组（Unix）
	if runtime.GOOS != "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Setpgid: true,
		}
	}

	// 日志捕获
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("创建 stdout 管道失败: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("创建 stderr 管道失败: %w", err)
	}

	m.status = "starting"
	m.lastError = ""

	// 启动进程
	if err := cmd.Start(); err != nil {
		m.status = "error"
		m.lastError = fmt.Sprintf("启动失败: %s", err)
		return fmt.Errorf("启动 FRPC 失败: %w", err)
	}

	m.cmd = cmd
	m.pid = cmd.Process.Pid
	m.startTime = time.Now()

	m.logger.Info("FRPC 已启动", "pid", m.pid, "admin_addr", m.adminAddr)

	// 启动日志收集
	go m.collectLogs(stdout, "stdout")
	go m.collectLogs(stderr, "stderr")

	// 等待进程退出
	go m.waitForExit()

	// 等待 Admin API 可用
	go m.waitForAdminAPI(ctx)

	return nil
}

// Stop 停止 FRPC 进程
// 优雅停止：SIGTERM -> 等待 -> SIGKILL
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cmd == nil || m.cmd.Process == nil {
		return nil
	}

	m.status = "stopping"
	m.logger.Info("正在停止 FRPC", "pid", m.pid)

	// 发送 SIGTERM
	if runtime.GOOS != "windows" {
		m.cmd.Process.Signal(syscall.SIGTERM)
	} else {
		m.cmd.Process.Signal(os.Interrupt)
	}

	// 等待优雅退出（最多 10 秒）
	done := make(chan error, 1)
	go func() {
		done <- m.cmd.Wait()
	}()

	select {
	case <-done:
		m.logger.Info("FRPC 已优雅退出")
	case <-time.After(10 * time.Second):
		// 超时，强制杀进程
		m.logger.Warn("FRPC 未响应 SIGTERM，发送 SIGKILL")
		if runtime.GOOS != "windows" {
			// 杀死整个进程组
			syscall.Kill(-m.pid, syscall.SIGKILL)
		} else {
			m.cmd.Process.Kill()
		}
		<-done
	}

	m.cmd = nil
	m.pid = 0
	m.status = "stopped"
	m.lastError = ""

	return nil
}

// Restart 重启 FRPC
func (m *Manager) Restart(ctx context.Context, configPath string) error {
	if err := m.Stop(); err != nil {
		m.logger.Warn("停止旧进程失败", "error", err)
	}
	time.Sleep(500 * time.Millisecond) // 等待端口释放
	return m.Start(ctx, configPath)
}

// Reload 重载配置（不停止进程）
func (m *Manager) Reload() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.status != "running" {
		return fmt.Errorf("FRPC 未运行")
	}

	// 通过 Admin API 发送 reload 请求
	url := fmt.Sprintf("http://%s/api/reload", m.adminAddr)
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return fmt.Errorf("创建 reload 请求失败: %w", err)
	}
	req.SetBasicAuth(m.adminUser, m.adminPass)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("reload 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("reload 失败: HTTP %d: %s", resp.StatusCode, string(body))
	}

	m.logger.Info("FRPC reload 成功")
	return nil
}

// Status 通过 Admin API 获取状态
func (m *Manager) Status() (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.status != "running" {
		return m.status, nil
	}

	url := fmt.Sprintf("http://%s/api/status", m.adminAddr)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return m.status, nil
	}
	req.SetBasicAuth(m.adminUser, m.adminPass)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return m.status, nil
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return string(body), nil
}

// Verify 验证配置文件
func (m *Manager) Verify(configPath string) error {
	cmd := exec.Command(m.binaryPath, "verify", "-c", configPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("验证失败: %s: %w", string(output), err)
	}
	return nil
}

// GetProcessInfo 获取进程信息
func (m *Manager) GetProcessInfo() (pid int, startTime time.Time, binaryHash string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.pid, m.startTime, m.binaryHash
}

// GetStatus 获取当前状态
func (m *Manager) GetStatus() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.status
}

// GetLastError 获取最后错误
func (m *Manager) GetLastError() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastError
}

// GetAdminCredentials 获取 Admin API 凭证
func (m *Manager) GetAdminCredentials() (addr, user, pass string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.adminAddr, m.adminUser, m.adminPass
}

// verifyBinary 验证 FRPC 二进制文件
func (m *Manager) verifyBinary() error {
	// 检查文件存在
	info, err := os.Stat(m.binaryPath)
	if err != nil {
		return fmt.Errorf("二进制文件不存在: %w", err)
	}

	// 检查是否可执行
	if info.Mode()&0111 == 0 {
		return fmt.Errorf("二进制文件不可执行")
	}

	// 计算 SHA-256
	f, err := os.Open(m.binaryPath)
	if err != nil {
		return fmt.Errorf("打开二进制文件失败: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("计算哈希失败: %w", err)
	}
	m.binaryHash = hex.EncodeToString(h.Sum(nil))

	return nil
}

// collectLogs 收集进程日志
func (m *Manager) collectLogs(reader io.ReadCloser, source string) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 64*1024)

	for scanner.Scan() {
		line := scanner.Text()
		// 过滤敏感信息
		sanitized := sanitizeLogLine(line)
		m.logger.Debug("FRPC", "source", source, "line", sanitized)
	}
}

// waitForExit 等待进程退出
func (m *Manager) waitForExit() {
	if m.cmd == nil {
		return
	}

	err := m.cmd.Wait()

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.ctx.Err() != nil {
		// 主动停止
		return
	}

	if err != nil {
		m.status = "error"
		m.lastError = fmt.Sprintf("进程退出: %s", err)
		m.logger.Error("FRPC 异常退出", "pid", m.pid, "error", err)
	} else {
		m.status = "stopped"
		m.logger.Info("FRPC 已退出", "pid", m.pid)
	}

	m.cmd = nil
	m.pid = 0
}

// waitForAdminAPI 等待 Admin API 可用
func (m *Manager) waitForAdminAPI(ctx context.Context) {
	url := fmt.Sprintf("http://%s/api/status", m.adminAddr)
	client := &http.Client{Timeout: 2 * time.Second}

	for i := 0; i < 30; i++ {
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
		}

		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			continue
		}
		req.SetBasicAuth(m.adminUser, m.adminPass)

		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			m.mu.Lock()
			m.status = "running"
			m.mu.Unlock()
			m.logger.Info("FRPC Admin API 就绪")
			return
		}
	}

	m.logger.Warn("FRPC Admin API 未就绪")
}

// generateAdminCredentials 生成 Admin API 随机凭证
func generateAdminCredentials() (port int, user, pass string, err error) {
	// 随机端口（10000-60000）
	portBytes := make([]byte, 2)
	rand.Read(portBytes)
	port = 10000 + int(portBytes[0])<<8|int(portBytes[1])%50000

	// 随机用户名（16 字符）
	userBytes := make([]byte, 12)
	rand.Read(userBytes)
	user = "admin_" + hex.EncodeToString(userBytes)[:8]

	// 随机密码（32 字符）
	passBytes := make([]byte, 24)
	rand.Read(passBytes)
	pass = hex.EncodeToString(passBytes)

	return port, user, pass, nil
}

// sanitizeLogLine 清理日志行中的敏感信息
func sanitizeLogLine(line string) string {
	sensitive := []string{"token", "password", "secret", "key", "auth"}
	lower := strings.ToLower(line)
	for _, kw := range sensitive {
		if strings.Contains(lower, kw) {
			if len(line) > 80 {
				return line[:80] + "... [REDACTED]"
			}
			return "[REDACTED]"
		}
	}
	return line
}

// DetectLegacyProcess 检测遗留的 FRPC 进程
// 通过进程名和二进制路径匹配
func DetectLegacyProcess(binaryPath string) ([]int, error) {
	// 简化实现：检查是否有同名进程
	cmd := exec.Command("pgrep", "-f", filepath.Base(binaryPath))
	output, err := cmd.Output()
	if err != nil {
		// pgrep 返回 1 表示无匹配
		return nil, nil
	}

	var pids []int
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if pid, err := strconv.Atoi(strings.TrimSpace(line)); err == nil {
			pids = append(pids, pid)
		}
	}
	return pids, nil
}
