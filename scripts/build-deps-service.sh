#!/bin/bash
#
# AIDG Deps-Service Docker 镜像构建脚本
# 使用静态的 Dockerfile.deps 进行构建
#

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Variables
IMAGE_NAME="aidg-deps-service"
IMAGE_TAG="latest"
DOCKERFILE="Dockerfile.deps"

# Parse command-line arguments
while [[ $# -gt 0 ]]; do
  case $1 in
    --image-name)
      IMAGE_NAME="$2"
      shift 2
      ;;
    --image-tag)
      IMAGE_TAG="$2"
      shift 2
      ;;
    --dockerfile)
      DOCKERFILE="$2"
      shift 2
      ;;
    -h|--help)
      echo "Usage: $0 [OPTIONS]"
      echo ""
      echo "Options:"
      echo "  --image-name NAME    Docker image name (default: aidg-deps-service)"
      echo "  --image-tag TAG      Docker image tag (default: latest)"
      echo "  --dockerfile FILE    Dockerfile to use (default: Dockerfile.deps)"
      echo "  -h, --help           Show this help message"
      echo ""
      echo "Examples:"
      echo "  $0"
      echo "  $0 --image-tag v1.0.0"
      echo "  $0 --image-name my-deps-service --image-tag latest"
      exit 0
      ;;
    *)
      echo -e "${RED}Unknown option: $1${NC}"
      echo "Use --help for usage information"
      exit 1
      ;;
  esac
done

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}  AIDG Deps-Service Build${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# Validate Dockerfile exists
if [ ! -f "$DOCKERFILE" ]; then
  echo -e "${RED}❌ Dockerfile not found: $DOCKERFILE${NC}"
  exit 1
fi

echo -e "${GREEN}📋 Configuration:${NC}"
echo "  Dockerfile: $DOCKERFILE"
echo "  Image Name: $IMAGE_NAME"
echo "  Image Tag: $IMAGE_TAG"
echo ""

# Build Docker image
echo -e "${YELLOW}🔨 Building Docker image...${NC}"
/usr/local/bin/docker build -f "$DOCKERFILE" -t "${IMAGE_NAME}:${IMAGE_TAG}" .

if [ $? -ne 0 ]; then
  echo ""
  echo -e "${RED}========================================${NC}"
  echo -e "${RED}  ❌ Docker build failed${NC}"
  echo -e "${RED}========================================${NC}"
  exit 1
fi

echo ""
echo -e "${YELLOW}🔍 Verifying image...${NC}"

# Verify image exists
if ! /usr/local/bin/docker image inspect "${IMAGE_NAME}:${IMAGE_TAG}" > /dev/null 2>&1; then
  echo -e "${RED}❌ Image verification failed: image not found${NC}"
  exit 1
fi

# Get image details
IMAGE_SIZE=$(/usr/local/bin/docker image inspect "${IMAGE_NAME}:${IMAGE_TAG}" --format='{{.Size}}' | awk '{printf "%.2f MB", $1/1024/1024}')
IMAGE_ID=$(/usr/local/bin/docker image inspect "${IMAGE_NAME}:${IMAGE_TAG}" --format='{{.Id}}' | cut -c8-19)

echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}  ✅ Build Successful!${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo -e "${GREEN}📦 Image Details:${NC}"
echo "  Name: ${IMAGE_NAME}:${IMAGE_TAG}"
echo "  ID: ${IMAGE_ID}"
echo "  Size: ${IMAGE_SIZE}"
echo ""
echo -e "${BLUE}🚀 Run container:${NC}"
echo "  docker run -d -p 8080:8080 \\"
echo "    -v \$(pwd)/data:/data \\"
echo "    -v \$(pwd)/config:/app/config:ro \\"
echo "    -e HUGGINGFACE_TOKEN=your_token \\"
echo "    ${IMAGE_NAME}:${IMAGE_TAG}"
echo ""
echo -e "${BLUE}🔍 Check logs:${NC}"
echo "  docker logs -f <container_id>"
echo ""
