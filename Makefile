# MieruPilot Makefile

# 版本信息
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ' 2>/dev/null || echo "unknown")

# 构建标志
LDFLAGS := -ldflags "-s -w \
	-X github.com/HlONGlin/MieruPilot/internal/buildinfo.Version=$(VERSION) \
	-X github.com/HlONGlin/MieruPilot/internal/buildinfo.Commit=$(COMMIT) \
	-X github.com/HlONGlin/MieruPilot/internal/buildinfo.BuildTime=$(BUILD_TIME)"

# 输出目录
DIST_DIR := dist

# 颜色输出
GREEN := \033[0;32m
NC := \033[0m # No Color

.PHONY: all clean build-manager build-agent build-all test fmt vet release help

all: build-all ## 构建所有组件 (默认)

help: ## 显示帮助信息
	@echo "MieruPilot 构建工具"
	@echo ""
	@echo "使用方法: make [目标]"
	@echo ""
	@echo "可用目标:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(GREEN)%-15s$(NC) %s\n", $$1, $$2}'

clean: ## 清理构建产物
	@echo "清理构建目录..."
	@rm -rf $(DIST_DIR)
	@echo "$(GREEN)✓ 清理完成$(NC)"

build-manager: ## 构建 Manager (当前平台)
	@echo "构建 Manager..."
	@mkdir -p $(DIST_DIR)
	@go build $(LDFLAGS) -o $(DIST_DIR)/merit-manager ./cmd/manager
	@echo "$(GREEN)✓ Manager 构建完成: $(DIST_DIR)/merit-manager$(NC)"

build-agent: ## 构建 Agent (当前平台)
	@echo "构建 Agent..."
	@mkdir -p $(DIST_DIR)
	@go build $(LDFLAGS) -o $(DIST_DIR)/merit-agent ./cmd/agent
	@echo "$(GREEN)✓ Agent 构建完成: $(DIST_DIR)/merit-agent$(NC)"

build-all: build-manager build-agent ## 构建 Manager 和 Agent

# 交叉编译 - Manager
build-manager-linux-amd64: ## 构建 Manager (Linux AMD64)
	@echo "构建 Manager for Linux AMD64..."
	@mkdir -p $(DIST_DIR)
	@GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(DIST_DIR)/merit-manager-linux-amd64 ./cmd/manager
	@echo "$(GREEN)✓ 构建完成: $(DIST_DIR)/merit-manager-linux-amd64$(NC)"

build-manager-linux-arm64: ## 构建 Manager (Linux ARM64)
	@echo "构建 Manager for Linux ARM64..."
	@mkdir -p $(DIST_DIR)
	@GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(DIST_DIR)/merit-manager-linux-arm64 ./cmd/manager
	@echo "$(GREEN)✓ 构建完成: $(DIST_DIR)/merit-manager-linux-arm64$(NC)"

build-manager-windows-amd64: ## 构建 Manager (Windows AMD64)
	@echo "构建 Manager for Windows AMD64..."
	@mkdir -p $(DIST_DIR)
	@GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(DIST_DIR)/merit-manager-windows-amd64.exe ./cmd/manager
	@echo "$(GREEN)✓ 构建完成: $(DIST_DIR)/merit-manager-windows-amd64.exe$(NC)"

build-manager-darwin-amd64: ## 构建 Manager (macOS Intel)
	@echo "构建 Manager for macOS Intel..."
	@mkdir -p $(DIST_DIR)
	@GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(DIST_DIR)/merit-manager-darwin-amd64 ./cmd/manager
	@echo "$(GREEN)✓ 构建完成: $(DIST_DIR)/merit-manager-darwin-amd64$(NC)"

build-manager-darwin-arm64: ## 构建 Manager (macOS Apple Silicon)
	@echo "构建 Manager for macOS Apple Silicon..."
	@mkdir -p $(DIST_DIR)
	@GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(DIST_DIR)/merit-manager-darwin-arm64 ./cmd/manager
	@echo "$(GREEN)✓ 构建完成: $(DIST_DIR)/merit-manager-darwin-arm64$(NC)"

# 交叉编译 - Agent
build-agent-linux-amd64: ## 构建 Agent (Linux AMD64)
	@echo "构建 Agent for Linux AMD64..."
	@mkdir -p $(DIST_DIR)
	@GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(DIST_DIR)/merit-agent-linux-amd64 ./cmd/agent
	@echo "$(GREEN)✓ 构建完成: $(DIST_DIR)/merit-agent-linux-amd64$(NC)"

build-agent-linux-arm64: ## 构建 Agent (Linux ARM64)
	@echo "构建 Agent for Linux ARM64..."
	@mkdir -p $(DIST_DIR)
	@GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(DIST_DIR)/merit-agent-linux-arm64 ./cmd/agent
	@echo "$(GREEN)✓ 构建完成: $(DIST_DIR)/merit-agent-linux-arm64$(NC)"

# 发布构建 - 所有平台
release-manager: build-manager-linux-amd64 build-manager-linux-arm64 build-manager-windows-amd64 build-manager-darwin-amd64 build-manager-darwin-arm64 ## 构建所有平台的 Manager

release-agent: build-agent-linux-amd64 build-agent-linux-arm64 ## 构建所有平台的 Agent

release: release-manager release-agent ## 构建所有平台的所有组件
	@echo "$(GREEN)✓ 发布构建完成！所有文件在 $(DIST_DIR) 目录$(NC)"
	@ls -lh $(DIST_DIR)

# 开发工具
test: ## 运行测试
	@echo "运行测试..."
	@go test -v -race -coverprofile=coverage.out ./...
	@echo "$(GREEN)✓ 测试完成$(NC)"

fmt: ## 格式化代码
	@echo "格式化代码..."
	@go fmt ./...
	@echo "$(GREEN)✓ 格式化完成$(NC)"

vet: ## 代码静态检查
	@echo "运行 go vet..."
	@go vet ./...
	@echo "$(GREEN)✓ 检查完成$(NC)"

tidy: ## 整理依赖
	@echo "整理依赖..."
	@go mod tidy
	@echo "$(GREEN)✓ 依赖整理完成$(NC)"

dev-manager: ## 以开发模式运行 Manager
	@echo "启动 Manager (开发模式)..."
	@go run ./cmd/manager --addr :3000 --data merit-dev.json

dev-agent: ## 以开发模式运行 Agent
	@echo "启动 Agent (开发模式)..."
	@go run ./cmd/agent --manager http://localhost:3000 --key dev-key

# 版本信息
version: ## 显示版本信息
	@echo "版本: $(VERSION)"
	@echo "提交: $(COMMIT)"
	@echo "构建时间: $(BUILD_TIME)"
