.PHONY: all build build-go build-frontend clean dev

# Default output binary
BINARY := aidg-lite

all: build

# Build everything
build: build-frontend build-go

# Build Go backend
build-go:
	@echo "==> Building Go backend..."
	cd .. && cd aidg-lite && go build -o $(BINARY) .
	@echo "==> Binary: $(BINARY)"

# Build frontend
build-frontend:
	@echo "==> Building frontend..."
	cd frontend && npm install --prefer-offline && npx vite build
	@echo "==> Frontend built to frontend/dist/"

# Development mode: start Vite dev server + Go backend
dev:
	@echo "Starting AIDG Lite in development mode..."
	@echo "Frontend dev server: http://localhost:5173"
	@echo "Backend: http://127.0.0.1:8000"
	cd frontend && npx vite &
	PORT=8000 FRONTEND_DIST_DIR=./frontend/dist go run .

# Run the built binary
run: build
	./$(BINARY)

# Clean build artifacts
clean:
	rm -f $(BINARY)
	rm -rf frontend/dist
	rm -rf frontend/node_modules

# TypeScript type check only
check:
	cd frontend && npx tsc --noEmit
