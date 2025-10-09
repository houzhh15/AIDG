# Unified Dockerfile for AIDG (Web Server + MCP Server + Frontend)
# Single image contains both human interface (8000) and AI interface (8081)

# Stage 1: Build Go backends
FROM golang:1.22-alpine AS backend-builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git make

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY cmd/ ./cmd/
COPY pkg/ ./pkg/

# Build both server binaries
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo \
    -ldflags="-w -s" \
    -o /app/bin/server \
    ./cmd/server

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo \
    -ldflags="-w -s" \
    -o /app/bin/mcp-server \
    ./cmd/mcp-server

# Stage 2: Build frontend
FROM node:18-alpine AS frontend-builder

WORKDIR /app/frontend

# Copy package files
COPY frontend/package*.json ./

# Install dependencies
# Note: Platform-specific packages like @rollup/rollup-linux-arm64 are auto-selected by npm
RUN npm install --no-fund --no-audit

# Copy frontend source
COPY frontend/ ./

# Build frontend (production mode)
RUN npm run build

# Stage 3: Runtime image (contains both servers)
FROM alpine:3.18

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata supervisor

# Create non-root user
RUN addgroup -g 1000 aidg && \
    adduser -D -u 1000 -G aidg aidg

WORKDIR /app

# Copy both backend binaries
COPY --from=backend-builder /app/bin/server /app/server
COPY --from=backend-builder /app/bin/mcp-server /app/mcp-server

# Copy MCP server prompts directory
COPY --from=backend-builder /app/cmd/mcp-server/prompts /app/prompts

# Copy frontend dist
COPY --from=frontend-builder /app/frontend/dist /app/frontend/dist

# Create data directories
RUN mkdir -p /app/data/projects \
             /app/data/users \
             /app/data/meetings \
             /app/data/audit_logs && \
    chown -R aidg:aidg /app

# Copy supervisor config
COPY deployments/docker/supervisord.conf /etc/supervisord.conf

# Switch to non-root user
USER aidg

# Expose both ports
EXPOSE 8000 8081

# Set environment variables
ENV ENV=production \
    PORT=8000 \
    MCP_HTTP_PORT=8081 \
    LOG_LEVEL=info \
    LOG_FORMAT=json

# Health check (check both services)
HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8000/health && \
        wget --no-verbose --tries=1 --spider http://localhost:8081/health || exit 1

# Run both servers with supervisor
CMD ["/usr/bin/supervisord", "-c", "/etc/supervisord.conf"]
