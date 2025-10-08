#!/bin/bash
# 开发环境启动脚本

set -e

# 加载环境变量
if [ -f .env ]; then
    export $(cat .env | grep -v '^#' | xargs)
fi

# 启动 server
echo "Starting Web Server on :${PORT:-8000}..."
go run ./cmd/server &
SERVER_PID=$!

# 启动 mcp-server
echo "Starting MCP Server on :${MCP_PORT:-8081}..."
go run ./cmd/mcp-server &
MCP_PID=$!

# 启动前端
echo "Starting Frontend on :5173..."
cd frontend && npm run dev &
FRONTEND_PID=$!

# 捕获退出信号
trap 'kill $SERVER_PID $MCP_PID $FRONTEND_PID' EXIT

echo ""
echo "All services started!"
echo "  - Web Server: http://localhost:${PORT:-8000}"
echo "  - MCP Server: http://localhost:${MCP_PORT:-8081}"
echo "  - Frontend: http://localhost:5173"
echo ""
echo "Press Ctrl+C to stop all services"

wait
