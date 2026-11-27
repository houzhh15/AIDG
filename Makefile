# AIDG Makefile

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date +%Y%m%d_%H%M%S)
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS := -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GitCommit=$(GIT_COMMIT)

.PHONY: all install build build-prod test test-compat dev clean docker-build

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
	@echo "Build complete: bin/server, bin/mcp-server"

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

# 无 Docker 的阶段 0 兼容基线
# 仓库内样本均为 synthetic；真实脱敏生产样本需按 test/compat/README.md 导入。
test-compat:
	go test ./cmd/server/internal/users ./cmd/server/internal/api ./cmd/server/internal/domain/meetings -run Compat -count=1

# 启动开发环境
dev:
	@echo "Starting development environment..."
	@bash -c '\
		if [ -f .env ]; then \
			echo "Loading .env file..."; \
			set -a; source .env; set +a; \
		fi; \
		mkdir -p ./data/projects ./data/users ./data/meetings ./data/audit; \
		unset WHISPER_MODE WHISPER_PROGRAM_PATH WHISPER_API_URL DIARIZATION_SCRIPT_PATH EMBEDDING_SCRIPT_PATH PYTHON_PATH HF_HOME HUGGINGFACE_TOKEN DEPENDENCY_MODE; \
		DEV_SERVER_PORT=$${AIDG_SERVER_PORT:-$${SERVER_PORT:-8082}}; \
		export AIDG_SERVER_PORT=$$DEV_SERVER_PORT; \
		export AIDG_MEETING_ASR_BACKEND=external; \
		export AIDG_MEETING_V2_ENABLED=true; \
		export VITE_API_BASE_URL=$${VITE_API_BASE_URL:-http://localhost:$$DEV_SERVER_PORT}; \
		echo "Starting server on :$$DEV_SERVER_PORT (external ASR backend)..."; \
		ENV=development \
		AIDG_DATA_PROJECTS_DIR=./data/projects \
		AIDG_DATA_USERS_DIR=./data/users \
		AIDG_DATA_MEETINGS_DIR=./data/meetings \
		AIDG_DATA_AUDIT_LOGS_DIR=./data/audit \
		JWT_SECRET=dev-secret-change-me-in-production-at-least-32-chars \
		USER_JWT_SECRET=dev-user-jwt-secret-at-least-32-chars \
		ADMIN_DEFAULT_PASSWORD=ChangeMe2024SecurePassword \
		DEPENDENCY_SHARED_VOLUME=./data \
		go run ./cmd/server & \
		echo "Starting MCP server on :$${MCP_HTTP_PORT:-8081} (backend http://localhost:$$DEV_SERVER_PORT)..."; \
		MCP_SERVER_URL=http://localhost:$$DEV_SERVER_PORT MCP_HTTP_PORT=$${MCP_HTTP_PORT:-8081} ENV=development go run ./cmd/mcp-server & \
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
