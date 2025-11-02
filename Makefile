# AIDG Makefile

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date +%Y%m%d_%H%M%S)
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS := -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GitCommit=$(GIT_COMMIT)

.PHONY: all install build build-prod test dev clean docker-build

all: build

# 安装依赖
install:
	@echo "Installing Go dependencies..."
	go mod download
	go mod verify
	@echo "Installing frontend dependencies..."
	cd frontend && npm ci

# 开发构建
build:
	@echo "Building for development..."
	go build -o bin/server ./cmd/server
	go build -o bin/mcp-server ./cmd/mcp-server
	go build -o bin/merge-segments ./cmd/merge-segments
	@echo "Build complete: bin/server, bin/mcp-server, bin/merge-segments"

# 生产构建
build-prod:
	@echo "Building for production..."
	go build -ldflags "$(LDFLAGS)" -o bin/server ./cmd/server
	go build -ldflags "$(LDFLAGS)" -o bin/mcp-server ./cmd/mcp-server
	@echo "Building frontend..."
	cd frontend && npm run build
	@echo "Production build complete"

# 运行测试
test:
	@echo "Running tests..."
	go test -v -race -cover ./...

# 启动开发环境
dev:
	@echo "Starting development environment..."
	@bash -c '\
		if [ -f .env ]; then \
			echo "Loading .env file..."; \
			set -a; source .env; set +a; \
		fi; \
		echo "Starting server on :8000..."; \
		ENV=development \
		JWT_SECRET=dev-secret-change-me-in-production-at-least-32-chars \
		USER_JWT_SECRET=dev-user-jwt-secret-at-least-32-chars \
		ADMIN_DEFAULT_PASSWORD=ChangeMe2024SecurePassword! \
		go run ./cmd/server & \
		echo "Starting MCP server on :8081..."; \
		ENV=development go run ./cmd/mcp-server & \
		echo "Starting frontend on :5173..."; \
		cd frontend && npm run dev'

# 清理构建产物
clean:
	@echo "Cleaning build artifacts..."
	rm -rf bin/
	rm -rf frontend/dist/
	go clean

# 构建 Docker 镜像
docker-build:
	@echo "Building Docker image..."
	docker build -t aidg:$(VERSION) .
	@echo "Docker image built"
