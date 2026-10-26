# 贡献指南

感谢你考虑为 MieruPilot 做出贡献！

## 开发环境

### 系统要求

- Go 1.21 或更高版本
- Git
- Make (可选，用于构建)

### 克隆仓库

```bash
git clone https://github.com/HlONGlin/UnifiedProxy.git
cd MieruPilot
```

### 安装依赖

```bash
go mod download
```

### 运行开发版本

**Manager (Web 管理面板):**

```bash
go run ./cmd/manager --addr :3000 --data merit-dev.json
```

访问 `http://localhost:3000` 并按提示设置管理员账号。

**Agent (节点代理):**

```bash
sudo go run ./cmd/agent --manager http://localhost:3000 --key <你的密钥>
```

> Agent 需要 root 权限来管理 systemd 服务。

## 项目结构

```
MieruPilot/
├── cmd/
│   ├── agent/          # Agent 入口点
│   └── manager/        # Manager 入口点
├── internal/
│   ├── agent/          # Agent 核心逻辑
│   ├── manager/        # Manager HTTP 服务器
│   │   └── web/        # Web 资源 (HTML/CSS/JS)
│   ├── model/          # 数据模型
│   ├── traffic/        # 流量统计
│   ├── updater/        # 更新检查
│   └── buildinfo/      # 版本信息
├── one-click.sh        # 一键安装脚本
├── go.mod
└── README.md
```

## 开发流程

### 1. 创建分支

```bash
git checkout -b feature/your-feature-name
```

或

```bash
git checkout -b fix/your-bug-fix
```

### 2. 编写代码

- 遵循 Go 代码风格规范
- 添加必要的注释
- 确保代码通过 `go vet` 检查

### 3. 测试

```bash
# 运行所有测试
go test ./...

# 代码静态检查
go vet ./...

# 格式化代码
go fmt ./...
```

### 4. 提交更改

使用清晰的提交信息：

```bash
git commit -m "feat: 添加新功能描述"
# 或
git commit -m "fix: 修复具体问题"
```

**提交信息格式:**

- `feat:` - 新功能
- `fix:` - Bug 修复
- `docs:` - 文档更新
- `style:` - 代码格式调整
- `refactor:` - 代码重构
- `perf:` - 性能优化
- `test:` - 测试相关
- `chore:` - 构建/工具链相关

### 5. 推送并创建 Pull Request

```bash
git push origin feature/your-feature-name
```

然后在 GitHub 上创建 Pull Request，描述你的更改。

## 代码规范

### Go 代码

- 使用 `gofmt` 格式化代码
- 遵循 [Effective Go](https://go.dev/doc/effective_go) 指南
- 导出的函数/类型必须有文档注释
- 错误处理不能忽略
- 避免使用 `panic`，除非真正不可恢复

### 前端代码

- HTML/CSS/JS 在 `internal/manager/web/assets/` 中
- 使用现代 ES6+ 语法
- 保持样式与现有设计一致
- 确保在不同浏览器中测试

### 文档

- README 使用中文
- 代码注释使用中文或英文均可
- API 相关注释建议使用英文

## 功能开发建议

### 添加新的 API 端点

1. 在 `internal/model/model.go` 中定义数据模型（如需要）
2. 在 `internal/manager/handlers.go` 中实现处理函数
3. 在 `internal/manager/server.go` 中注册路由
4. 在 `internal/manager/web/assets/app.js` 中添加前端调用

### 添加新的流量功能

1. 在 `internal/traffic/traffic.go` 中实现统计逻辑
2. 在 `internal/model/model.go` 中添加相关字段
3. 在 Agent 的上报逻辑中添加数据采集

### 修改前端样式

主要样式定义在 `internal/manager/web/assets/app.css` 中。

## 报告问题

在提交 Issue 时，请包含：

- 操作系统和版本
- Go 版本
- MieruPilot 版本
- 详细的复现步骤
- 预期行为和实际行为
- 相关日志或错误信息

## 安全问题

如果你发现安全漏洞，**请不要公开提交 Issue**。请通过以下方式联系：

- 发送邮件到 [你的邮箱]
- 或通过 GitHub Security Advisories

## 许可证

通过贡献代码，你同意你的贡献将在与本项目相同的许可证下发布。

## 行为准则

- 尊重所有贡献者
- 接受建设性的批评
- 专注于对项目最有利的事情
- 对其他社区成员表现出同理心

## 获取帮助

- 查看 [README.md](README.md)
- 在 [GitHub Discussions](https://github.com/HlONGlin/UnifiedProxy/discussions) 中提问
- 查看现有的 [Issues](https://github.com/HlONGlin/UnifiedProxy/issues)

---

再次感谢你的贡献！🎉
