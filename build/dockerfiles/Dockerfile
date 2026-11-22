# Lightweight Dockerfile for AIDG - Go services and frontend only.

# Stage 1: Build Go backends
FROM golang:1.23-alpine AS backend-builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git make

# Copy go mod files first (for better caching)
COPY go.mod go.sum ./
RUN go mod download

# Copy source code (split by subdirectory for better cache invalidation)
# This ensures Docker can detect changes in individual subdirectories
COPY cmd/server/ ./cmd/server/
COPY cmd/mcp-server/ ./cmd/mcp-server/
COPY cmd/merge-segments/ ./cmd/merge-segments/
COPY pkg/logger/ ./pkg/logger/
COPY pkg/metrics/ ./pkg/metrics/
COPY pkg/similarity/ ./pkg/similarity/

# Build all binaries with maximum compression
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo \
    -ldflags="-w -s" \
    -o /app/bin/server \
    ./cmd/server && \
    CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo \
    -ldflags="-w -s" \
    -o /app/bin/mcp-server \
    ./cmd/mcp-server && \
    CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo \
    -ldflags="-w -s" \
    -o /app/bin/merge-segments \
    ./cmd/merge-segments

# Optional: Further compress binaries with UPX (uncomment if needed)
# RUN apk add --no-cache upx && \
#     upx --best --lzma /app/bin/server /app/bin/mcp-server /app/bin/merge-segments

# Stage 2: Build frontend
FROM node:20-alpine AS frontend-builder

WORKDIR /app/frontend

COPY frontend/package*.json ./
RUN npm config set fetch-timeout 600000 && \
    npm config set fetch-retries 5 && \
    npm config set fetch-retry-mintimeout 20000 && \
    npm config set fetch-retry-maxtimeout 120000 && \
    npm install --no-fund --no-audit --production=false

COPY frontend/ ./
RUN npm run build && \
    # Remove source maps to reduce size
    find dist -name "*.map" -type f -delete

# Stage 3: Lightweight runtime image
FROM alpine:3.19

RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    supervisor \
    wget

# Create non-root user and all necessary directories in one layer
RUN addgroup -g 1000 aidg && \
    adduser -D -u 1000 -G aidg aidg && \
    mkdir -p /app/data/projects \
             /app/data/users \
             /app/data/meetings \
             /app/data/audit_logs \
             /app/prompts \
             /app/frontend/dist \
             /data \
             /models \
             /external/scripts && \
    chown -R aidg:aidg /app /data /models /external

WORKDIR /app

# Copy all binaries in one layer
COPY --from=backend-builder --chown=aidg:aidg /app/bin/server /app/server
COPY --from=backend-builder --chown=aidg:aidg /app/bin/mcp-server /app/mcp-server
COPY --from=backend-builder --chown=aidg:aidg /app/bin/merge-segments /usr/local/bin/merge-segments
COPY --from=backend-builder --chown=aidg:aidg /app/cmd/mcp-server/prompts /app/prompts

# Copy frontend dist
COPY --from=frontend-builder --chown=aidg:aidg /app/frontend/dist /app/frontend/dist

# Copy supervisor config
COPY --chown=aidg:aidg deployments/docker/supervisord.conf /etc/supervisord.conf

# Switch to non-root user
USER aidg

# Expose web and MCP ports
EXPOSE 8000 8001

# Environment variables for external dependencies
ENV ENV=production \
    PORT=8000 \
    MCP_HTTP_PORT=8001 \
    LOG_LEVEL=info \
    LOG_FORMAT=json \
    ENABLE_AUDIO_CONVERSION=false \
    ENABLE_SPEAKER_DIARIZATION=false

HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8000/health && \
        wget --no-verbose --tries=1 --spider http://localhost:8001/health || exit 1

# Run with supervisor
CMD ["/usr/bin/supervisord", "-c", "/etc/supervisord.conf"]
