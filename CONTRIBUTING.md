# 贡献指南

感谢您对 FRP 多用户云隧道管理平台项目的关注！

## 开发环境设置

### 前置要求

- Go 1.21+
- Node.js 20+
- npm 或 pnpm
- Git

### 克隆项目

```bash
git clone https://github.com/sshiong/frp-cloudflare-platform.git
cd frp-cloudflare-platform
```

### 安装依赖

```bash
# 安装 Go 依赖
cd server-panel && go mod download
cd ../client-panel && go mod download

# 安装前端依赖
cd ../server-panel/web-admin && npm install
cd ../../client-panel/web-client && npm install
```

### 开发模式运行

```bash
# 启动 Server Panel 开发模式
make dev-server

# 启动 Client Panel 开发模式
make dev-client
```

## 代码规范

### Go 代码

- 使用 `gofmt` 格式化代码
- 使用 `golangci-lint` 进行代码检查
- 遵循 [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- 使用结构化日志 (slog)
- 所有敏感数据必须加密

### TypeScript/Vue 代码

- 使用 ESLint 进行代码检查
- 使用 Prettier 格式化代码
- 遵循 Vue 3 组合式 API 风格
- 使用 TypeScript 严格模式

### 提交规范

使用 [Conventional Commits](https://www.conventionalcommits.org/) 规范：

```
<type>(<scope>): <subject>

[optional body]

[optional footer(s)]
```

类型 (type):
- `feat`: 新功能
- `fix`: 修复 bug
- `docs`: 文档更新
- `style`: 代码格式调整
- `refactor`: 重构
- `test`: 测试相关
- `chore`: 构建/工具相关

示例：
```
feat(server): add user management API
fix(client): fix FRPC restart issue
docs(readme): update installation guide
```

## 分支策略

- `main`: 生产分支
- `develop`: 开发分支
- `feature/*`: 功能分支
- `fix/*`: 修复分支
- `release/*`: 发布分支

## Pull Request 流程

1. Fork 项目
2. 创建功能分支 (`git checkout -b feature/amazing-feature`)
3. 提交更改 (`git commit -m 'feat: add amazing feature'`)
4. 推送到分支 (`git push origin feature/amazing-feature`)
5. 创建 Pull Request

### PR 要求

- 描述清楚更改内容
- 包含相关测试
- 通过所有 CI 检查
- 代码审查通过

## 测试

### 运行测试

```bash
# 运行所有测试
make test

# 运行 Server Panel 测试
make test-server

# 运行 Client Panel 测试
make test-client

# 运行集成测试
make test-integration
```

### 测试覆盖率

```bash
# 生成覆盖率报告
cd server-panel && go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## 安全

### 报告安全漏洞

如果您发现安全漏洞，请**不要**在公开 Issue 中报告。

请发送邮件至 security@example.com，我们会尽快回复。

### 安全编码实践

- 所有用户输入必须验证和清理
- 使用参数化 SQL 查询
- 敏感数据必须加密存储
- 日志中不得包含敏感信息
- 使用安全的密码哈希算法 (Argon2id)
- 实施适当的访问控制

## 文档

### 更新文档

- 代码变更需要更新相关文档
- API 变更需要更新 OpenAPI 规范
- 新功能需要添加使用说明

### 生成文档

```bash
# 生成 API 文档
go install github.com/swaggo/swag/cmd/swag@latest
cd server-panel && swag init
```

## 发布

### 版本号

使用 [Semantic Versioning](https://semver.org/):

- MAJOR.MINOR.PATCH
- 例如: 1.0.0, 1.1.0, 1.1.1

### 发布流程

1. 创建 release 分支
2. 更新版本号
3. 更新 CHANGELOG
4. 创建 Git tag
5. GitHub Actions 自动构建和发布

## 联系方式

- Issues: GitHub Issues
- Discussions: GitHub Discussions
- Email: support@example.com

## 许可证

本项目使用 MIT 许可证。贡献代码即表示您同意将代码置于 MIT 许可证下。
