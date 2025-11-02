#!/bin/bash
# Local deployment script for NLP Service (without Docker)

set -e

# Configuration
VENV_DIR="venv"
PORT=5001

echo "======================================"
echo "NLP Service Local Deployment"
echo "======================================"
echo ""

# Navigate to project root
cd "$(dirname "$0")/.."

# Check if Python 3 is available
if ! command -v python3 &> /dev/null; then
    echo "✗ Python 3 is not installed"
    exit 1
fi

echo "Python version:"
python3 --version
echo ""

# Create virtual environment if not exists
if [ ! -d "${VENV_DIR}" ]; then
    echo "Creating virtual environment..."
    python3 -m venv "${VENV_DIR}"
    echo "✓ Virtual environment created"
else
    echo "✓ Virtual environment already exists"
fi
echo ""

# Activate virtual environment
echo "Activating virtual environment..."
source "${VENV_DIR}/bin/activate"
echo ""

# Install dependencies
echo "Installing dependencies..."
pip install --upgrade pip
pip install -r requirements.txt
echo "✓ Dependencies installed"
echo ""

# Download model if not cached
echo "Checking/downloading text2vec-base-chinese model..."
python -c "from sentence_transformers import SentenceTransformer; SentenceTransformer('shibing624/text2vec-base-chinese')"
echo "✓ Model ready"
echo ""

# Run service
echo "======================================"
echo "Starting NLP Service on port ${PORT}..."
echo "======================================"
echo ""
echo "Service will be available at: http://localhost:${PORT}"
echo "API docs: http://localhost:${PORT}/docs"
echo "Health check: http://localhost:${PORT}/health"
echo ""
echo "Press Ctrl+C to stop the service"
echo ""

uvicorn app:app --host 0.0.0.0 --port "${PORT}" --reload
