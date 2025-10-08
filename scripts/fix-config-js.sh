#!/bin/bash

# 修复 config.js Content-Type 问题的快速部署脚本

set -e

echo "🔧 修复 config.js Content-Type 问题"
echo "=================================="
echo ""

# 颜色定义
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# 1. 停止现有容器
echo -e "${YELLOW}1. 停止现有容器...${NC}"
docker compose down

# 2. 重新构建镜像（仅后端部分需要重新编译）
echo -e "${YELLOW}2. 重新构建 Docker 镜像...${NC}"
echo "   注意：前端不需要重新构建，只需要重新编译 Go 代码"
docker compose build --no-cache

# 3. 启动服务
echo -e "${YELLOW}3. 启动服务...${NC}"
docker compose up -d

# 4. 等待服务就绪
echo -e "${YELLOW}4. 等待服务启动...${NC}"
sleep 5

# 5. 验证修复
echo ""
echo -e "${YELLOW}5. 验证修复...${NC}"
echo ""

# 检查 config.js 的 Content-Type
echo "检查 config.js 响应头："
CONTENT_TYPE=$(curl -s -I http://localhost:8000/config.js | grep -i "content-type" | tr -d '\r')
echo "  $CONTENT_TYPE"

if echo "$CONTENT_TYPE" | grep -q "application/javascript"; then
    echo -e "  ${GREEN}✅ Content-Type 正确${NC}"
else
    echo -e "  ${RED}❌ Content-Type 仍然错误${NC}"
    echo "  预期: Content-Type: application/javascript"
    exit 1
fi

# 检查 Cache-Control
CACHE_CONTROL=$(curl -s -I http://localhost:8000/config.js | grep -i "cache-control" | tr -d '\r')
echo "  $CACHE_CONTROL"

if echo "$CACHE_CONTROL" | grep -q "no-cache"; then
    echo -e "  ${GREEN}✅ Cache-Control 正确${NC}"
else
    echo -e "  ${YELLOW}⚠️  Cache-Control 可能不正确${NC}"
fi

echo ""
echo "检查服务状态："
docker compose ps

echo ""
echo "查看最近日志："
docker compose logs --tail=20

echo ""
echo -e "${GREEN}✅ 修复完成！${NC}"
echo ""
echo "📝 下一步："
echo "   1. 打开浏览器访问 http://localhost:8000"
echo "   2. 按 Cmd+Shift+R (Mac) 或 Ctrl+Shift+R (Windows) 强制刷新"
echo "   3. 或使用隐私/无痕模式访问"
echo ""
echo "🔍 如果仍有问题："
echo "   - 检查浏览器控制台（F12 → Console）"
echo "   - 检查网络请求（F12 → Network）"
echo "   - 参考文档：docs/FRONTEND_JS_ERROR_FIX.md"
echo ""
