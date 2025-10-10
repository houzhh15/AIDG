#!/bin/bash
# Whisper 容器诊断脚本

echo "=== Whisper 容器诊断 ==="
echo

echo "1. 检查容器状态..."
docker ps -a --filter name=aidg-whisper --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"
echo

echo "2. 检查容器日志..."
docker logs aidg-whisper 2>&1 | tail -30
echo

echo "3. 检查 Whisper 模型目录..."
ls -lh ./models/whisper/ 2>/dev/null || echo "模型目录不存在"
echo

echo "4. 检查 Whisper 镜像信息..."
docker images ghcr.io/mutablelogic/go-whisper:latest
echo

echo "5. 尝试手动启动容器（测试）..."
echo "如果上面的步骤都失败，可以尝试："
echo "  docker run --rm -p 8082:8082 -v ./models/whisper:/models:ro ghcr.io/mutablelogic/go-whisper:latest serve --host 0.0.0.0 --port 8082 --models-dir /models"
echo

echo "=== 诊断完成 ==="
