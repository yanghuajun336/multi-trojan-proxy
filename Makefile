.PHONY: build test clean install lint run help

# 变量定义
BINARY_NAME=proxy-server
GO=go
GOFLAGS=-v
BUILD_DIR=build
INSTALL_DIR=/usr/local/bin

# 默认目标
all: build

## build: 编译项目
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/proxy
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

## install: 安装到系统
install: build
	@echo "Installing $(BINARY_NAME) to $(INSTALL_DIR)..."
	@sudo cp $(BUILD_DIR)/$(BINARY_NAME) $(INSTALL_DIR)/
	@echo "Installed successfully"

## clean: 清理构建文件
clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	@$(GO) clean
	@echo "Clean complete"

## test: 运行测试
test:
	@echo "Running tests..."
	$(GO) test -v ./...

## lint: 代码检查
lint:
	@echo "Running linters..."
	@if command -v golangci-lint > /dev/null; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed, skipping..."; \
		$(GO) vet ./...; \
	fi

## run: 运行程序(开发模式)
run: build
	@echo "Running $(BINARY_NAME)..."
	@$(BUILD_DIR)/$(BINARY_NAME) -config config.yaml

## fmt: 格式化代码
fmt:
	@echo "Formatting code..."
	@$(GO) fmt ./...

## deps: 下载依赖
deps:
	@echo "Downloading dependencies..."
	@$(GO) mod download
	@$(GO) mod tidy

## help: 显示帮助信息
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^## ' Makefile | sed 's/## /  /'
