#!/bin/sh
# Dynamic healthcheck script that supports both HTTP and HTTPS

set -e

# Determine protocol from SERVER_PROTOCOL environment variable
PROTOCOL="${SERVER_PROTOCOL:-http}"
PORT="${PORT:-8000}"
MCP_PORT="${MCP_HTTP_PORT:-8081}"

# Additional wget options for HTTPS
WGET_OPTS="--no-verbose --tries=1 --spider"
if [ "$PROTOCOL" = "https" ]; then
    WGET_OPTS="$WGET_OPTS --no-check-certificate"
fi

# Check main server
wget $WGET_OPTS "${PROTOCOL}://localhost:${PORT}/health" || exit 1

# Check MCP server (always HTTP on internal port)
wget --no-verbose --tries=1 --spider "http://localhost:${MCP_PORT}/health" || exit 1

exit 0
