#!/bin/bash
# AIDG Docker Compose 快速诊断脚本

set -e

echo "================================================================================"
echo "AIDG Docker Compose 诊断工具"
echo "================================================================================"
echo ""

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 1. 检查容器状态
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "1. 容器状态检查"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
if docker compose ps 2>/dev/null; then
    echo -e "${GREEN}✓ 容器状态获取成功${NC}"
else
    echo -e "${RED}✗ 无法获取容器状态${NC}"
    exit 1
fi
echo ""

# 2. 检查端口占用
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "2. 端口占用检查"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "检查端口 8000..."
if lsof -i :8000 >/dev/null 2>&1; then
    echo -e "${YELLOW}⚠ 端口 8000 已被占用：${NC}"
    lsof -i :8000
else
    echo -e "${GREEN}✓ 端口 8000 可用${NC}"
fi

echo ""
echo "检查端口 8081..."
if lsof -i :8081 >/dev/null 2>&1; then
    echo -e "${YELLOW}⚠ 端口 8081 已被占用：${NC}"
    lsof -i :8081
else
    echo -e "${GREEN}✓ 端口 8081 可用${NC}"
fi
echo ""

# 3. 检查容器日志
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "3. 容器日志（最后 30 行）"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
docker compose logs --tail=30 aidg 2>/dev/null || echo -e "${RED}✗ 无法获取日志${NC}"
echo ""

# 4. 检查健康状态
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "4. 容器健康检查"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
HEALTH=$(docker inspect aidg-unified --format='{{.State.Health.Status}}' 2>/dev/null || echo "no-healthcheck")
if [ "$HEALTH" = "healthy" ]; then
    echo -e "${GREEN}✓ 容器健康状态: $HEALTH${NC}"
elif [ "$HEALTH" = "no-healthcheck" ]; then
    echo -e "${YELLOW}⚠ 未配置健康检查${NC}"
else
    echo -e "${RED}✗ 容器健康状态: $HEALTH${NC}"
fi
echo ""

# 5. 检查 Supervisor 状态
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "5. Supervisor 进程状态"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
if docker compose exec -T aidg supervisorctl status 2>/dev/null; then
    echo -e "${GREEN}✓ Supervisor 状态获取成功${NC}"
else
    echo -e "${RED}✗ 无法获取 Supervisor 状态（容器可能未运行）${NC}"
fi
echo ""

# 6. 测试端口连接
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "6. 端口连接测试"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "测试 Web Server (8000)..."
if nc -zv localhost 8000 2>&1 | grep -q succeeded; then
    echo -e "${GREEN}✓ 端口 8000 可连接${NC}"
    
    # 测试 HTTP 响应
    if curl -s -o /dev/null -w "%{http_code}" http://localhost:8000/health | grep -q 200; then
        echo -e "${GREEN}✓ Web Server 健康检查通过${NC}"
    else
        echo -e "${YELLOW}⚠ Web Server 健康检查失败${NC}"
    fi
else
    echo -e "${RED}✗ 无法连接到端口 8000${NC}"
fi

echo ""
echo "测试 MCP Server (8081)..."
if nc -zv localhost 8081 2>&1 | grep -q succeeded; then
    echo -e "${GREEN}✓ 端口 8081 可连接${NC}"
    
    # 测试 HTTP 响应
    if curl -s -o /dev/null -w "%{http_code}" http://localhost:8081/health | grep -q 200; then
        echo -e "${GREEN}✓ MCP Server 健康检查通过${NC}"
    else
        echo -e "${YELLOW}⚠ MCP Server 健康检查失败${NC}"
    fi
else
    echo -e "${RED}✗ 无法连接到端口 8081${NC}"
fi
echo ""

# 7. 检查监听端口（容器内）
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "7. 容器内监听端口检查"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
if docker compose exec -T aidg netstat -tlnp 2>/dev/null | grep -E "8000|8081"; then
    echo -e "${GREEN}✓ 服务正在监听端口${NC}"
else
    echo -e "${RED}✗ 服务未监听 8000 或 8081 端口${NC}"
fi
echo ""

# 总结
echo "================================================================================"
echo "诊断完成"
echo "================================================================================"
echo ""
echo "快速修复建议："
echo ""
echo "如果容器未运行："
echo "  docker compose up -d"
echo ""
echo "如果容器运行但服务异常："
echo "  docker compose restart"
echo ""
echo "如果需要重建："
echo "  docker compose down && docker compose up -d --build"
echo ""
echo "查看实时日志："
echo "  docker compose logs -f aidg"
echo ""
echo "进入容器调试："
echo "  docker compose exec aidg sh"
echo ""
echo "================================================================================"
