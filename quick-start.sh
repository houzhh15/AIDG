#!/bin/bash
# Quick start script for AIDG using GHCR image

set -e

VERSION="${1:-0.1.0-alpha}"

echo "🚀 Starting AIDG (version: ${VERSION})"
echo ""

# Create data directories
mkdir -p data/{projects,users,meetings,audit_logs}

# Start with docker-compose
IMAGE_TAG=${VERSION} docker-compose -f docker-compose.ghcr.yml up -d

echo ""
echo "✅ AIDG is starting!"
echo ""
echo "📍 Access points:"
echo "   Web UI:  http://localhost:8000"
echo "   MCP API: http://localhost:8081"
echo ""
echo "📋 Useful commands:"
echo "   Logs:    docker-compose -f docker-compose.ghcr.yml logs -f"
echo "   Stop:    docker-compose -f docker-compose.ghcr.yml down"
echo "   Status:  docker-compose -f docker-compose.ghcr.yml ps"
echo ""
