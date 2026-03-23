# Contributing

English | [中文](#中文)

Thank you for your interest in wechat-robot-go! Issues and Pull Requests are welcome.

## Development Environment

- Go 1.21 or later
- Git

## Getting Started

```bash
# Clone the repository
git clone https://github.com/SpellingDragon/wechat-robot-go.git
cd wechat-robot-go

# Install dependencies
go mod download

# Run tests
go test ./... -race

# Run examples
go run ./examples/echo
```

## Code Standards

### Formatting

Use `gofmt` for code formatting:

```bash
gofmt -w .
```

### Linting

Use `golangci-lint` for code analysis:

```bash
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
golangci-lint run
```

### Testing

- All new features must include unit tests
- Coverage target: 80%+
- Use table-driven test patterns
- Run tests with race detection: `go test ./... -race`

```bash
# Run all tests
go test ./... -v -race

# View coverage
go test ./... -cover

# Generate coverage report
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## Commit Convention

### Commit Messages

Use [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <description>

[optional body]
```

Types:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation
- `refactor`: Code refactoring
- `test`: Tests
- `chore`: Build/tooling

Examples:
```
feat(bot): add middleware support
fix(poller): resolve data race in Stop/Run
docs(readme): add bilingual documentation
```

### Pull Requests

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/my-feature`
3. Commit changes: `git commit -m "feat: add some feature"`
4. Push the branch: `git push origin feature/my-feature`
5. Create a Pull Request

PR Checklist:
- [ ] Code formatted with `gofmt`
- [ ] Passes `golangci-lint` checks
- [ ] Includes necessary tests
- [ ] All tests pass with `-race` flag
- [ ] Documentation updated if needed

## Project Structure

```
wechat/
├── auth.go                # Login authentication
├── bot.go                 # Bot core logic & facade API
├── client.go              # HTTP client
├── crypto.go              # AES encryption utilities
├── cdn.go                 # CDN upload/download
├── media_builder.go       # Media message item builders
├── message.go             # Message types
├── message_send.go        # Core message sending
├── message_send_media.go  # One-stop media sending
├── text_split.go          # Smart text splitting
├── middleware.go           # Middleware chain
├── poller.go              # Long-polling
├── context_token_store.go # Context token persistence
├── typing.go              # Typing status
├── options.go             # Configuration options
├── errors.go              # Error types
└── *_test.go              # Test files

examples/
├── echo/                  # Echo bot example
└── ai-agent/              # AI Agent example
```

## Code of Conduct

- Respect all contributors
- Maintain professional and friendly communication
- Accept constructive criticism

## License

This project is licensed under the MIT License. Contributed code will be licensed under the same terms.

---

<a name="中文"></a>

# 贡献指南

感谢您对 wechat-robot-go 的关注！欢迎提交 Issue 和 Pull Request。

## 开发环境

- Go 1.21 或更高版本
- Git

## 快速开始

```bash
# 克隆仓库
git clone https://github.com/SpellingDragon/wechat-robot-go.git
cd wechat-robot-go

# 安装依赖
go mod download

# 运行测试
go test ./... -race

# 运行示例
go run ./examples/echo
```

## 代码规范

### 格式化

使用 `gofmt` 格式化代码：

```bash
gofmt -w .
```

### Lint

使用 `golangci-lint` 进行代码检查：

```bash
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
golangci-lint run
```

### 测试

- 所有新功能必须添加单元测试
- 测试覆盖率目标：80%+
- 使用 table-driven 测试模式
- 使用 `-race` 标志运行测试

```bash
# 运行所有测试
go test ./... -v -race

# 查看覆盖率
go test ./... -cover

# 生成覆盖率报告
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## 提交规范

### Commit Message

使用 Conventional Commits 格式：

```
<type>(<scope>): <description>

[optional body]
```

类型：
- `feat`: 新功能
- `fix`: Bug 修复
- `docs`: 文档更新
- `refactor`: 代码重构
- `test`: 测试相关
- `chore`: 构建/工具链相关

示例：
```
feat(bot): add middleware support
fix(poller): resolve data race in Stop/Run
docs(readme): add bilingual documentation
```

### Pull Request

1. Fork 本仓库
2. 创建功能分支：`git checkout -b feature/my-feature`
3. 提交更改：`git commit -m "feat: add some feature"`
4. 推送分支：`git push origin feature/my-feature`
5. 创建 Pull Request

PR 检查清单：
- [ ] 代码通过 `gofmt` 格式化
- [ ] 代码通过 `golangci-lint` 检查
- [ ] 添加了必要的测试
- [ ] 所有测试通过（含 `-race` 标志）
- [ ] 更新了相关文档

## 项目结构

```
wechat/
├── auth.go                # 登录认证
├── bot.go                 # Bot 核心逻辑和门面 API
├── client.go              # HTTP 客户端
├── crypto.go              # AES 加密工具
├── cdn.go                 # CDN 上传/下载
├── media_builder.go       # 媒体消息构建器
├── message.go             # 消息结构
├── message_send.go        # 核心消息发送
├── message_send_media.go  # 一站式媒体发送
├── text_split.go          # 智能文本分片
├── middleware.go           # 中间件链
├── poller.go              # 长轮询
├── context_token_store.go # Context Token 持久化
├── typing.go              # Typing 状态
├── options.go             # 配置选项
├── errors.go              # 错误类型
└── *_test.go              # 测试文件

examples/
├── echo/                  # Echo 示例
└── ai-agent/              # AI Agent 示例
```

## 行为准则

- 尊重所有贡献者
- 保持专业和友好的交流
- 接受建设性批评

## 问题反馈

- 使用 GitHub Issues 报告 Bug
- 提供详细的复现步骤和环境信息
- 标注合适的标签（bug, enhancement, question 等）

## 许可证

本项目采用 MIT 许可证。贡献的代码将以相同许可证授权。
