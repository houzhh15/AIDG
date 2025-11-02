#!/bin/bash
# Docker image build script for NLP Service

set -e

# Configuration
IMAGE_NAME="aidg-nlp-service"
IMAGE_TAG="latest"
DOCKERFILE_PATH="../Dockerfile"

echo "======================================"
echo "Building NLP Service Docker Image"
echo "======================================"
echo "Image: ${IMAGE_NAME}:${IMAGE_TAG}"
echo ""

# Navigate to project root
cd "$(dirname "$0")/.."

# Build image
echo "Building Docker image..."
docker build -t "${IMAGE_NAME}:${IMAGE_TAG}" -f Dockerfile .

# Check if build succeeded
if [ $? -eq 0 ]; then
    echo ""
    echo "✓ Docker image built successfully"
    echo ""
    echo "Image details:"
    docker images | grep "${IMAGE_NAME}"
    echo ""
    echo "To run the container:"
    echo "  docker run -d -p 5000:5000 --name nlp-service ${IMAGE_NAME}:${IMAGE_TAG}"
    echo ""
    echo "To test the service:"
    echo "  curl http://localhost:5000/health"
else
    echo ""
    echo "✗ Docker image build failed"
    exit 1
fi
