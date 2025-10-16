#!/bin/bash

echo "================================"
echo "ASR 诊断脚本"
echo "================================"
echo ""

# 1. 检查容器状态
echo "1. 检查容器状态"
echo "--------------------------------"
docker ps --filter "name=aidg" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"
echo ""

# 2. 检查 Whisper 服务健康
echo "2. 检查 Whisper 服务健康"
echo "--------------------------------"
curl -s http://localhost:8082/api/v1/models | jq '.' 2>/dev/null || echo "❌ Whisper 服务无法访问"
echo ""

# 3. 检查服务状态 API
echo "3. 检查服务状态 API"
echo "--------------------------------"
curl -s http://localhost:8000/api/v1/services/status | jq '.'
echo ""

# 4. 检查环境变量
echo "4. 检查关键环境变量"
echo "--------------------------------"
docker exec aidg-unified env | grep -E "WHISPER|ENABLE_AUDIO|ENABLE_DEGRADATION" | sort
echo ""

# 5. 查看最近的日志
echo "5. 查看 aidg-unified 最近日志（最后 50 行）"
echo "--------------------------------"
docker logs aidg-unified --tail 50 | grep -E "ASR|Whisper|HealthChecker|Orchestrator"
echo ""

# 6. 检查是否有会议任务
echo "6. 检查会议任务列表"
echo "--------------------------------"
curl -s http://localhost:8000/api/v1/meetings | jq '.data | length' 2>/dev/null || echo "无法获取会议列表"
echo ""

# 7. 检查数据目录
echo "7. 检查数据目录结构"
echo "--------------------------------"
echo "会议数据目录:"
ls -la ./data/meetings/ 2>/dev/null | head -10 || echo "目录不存在或为空"
echo ""

echo "================================"
echo "诊断完成"
echo "================================"
