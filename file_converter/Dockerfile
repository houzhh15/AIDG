# Multi-stage build for File Converter Service with OCR support
FROM python:3.10-slim as builder

WORKDIR /app

# Install build dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    gcc \
    g++ \
    && rm -rf /var/lib/apt/lists/*

# Copy requirements and install dependencies
COPY requirements.txt .
RUN pip install --no-cache-dir --user -r requirements.txt

# Runtime stage
FROM python:3.10-slim

WORKDIR /app

# Install runtime dependencies for OCR and PDF processing
# tesseract-ocr: OCR engine (~30MB)
# poppler-utils: PDF to image conversion (pdf2image dependency) (~15MB)
# wget: for downloading language packs
RUN apt-get update && apt-get install -y --no-install-recommends \
    tesseract-ocr \
    poppler-utils \
    wget \
    && rm -rf /var/lib/apt/lists/*

# Create tessdata directory for language packs
RUN mkdir -p /app/tessdata

# Set Tesseract data path
ENV TESSDATA_PREFIX=/app/tessdata

# Download English language pack as base (~4MB)
RUN wget -q -O /app/tessdata/eng.traineddata \
    https://github.com/tesseract-ocr/tessdata/raw/main/eng.traineddata

# Copy Python dependencies from builder
COPY --from=builder /root/.local /root/.local

# Make sure scripts in .local are usable
ENV PATH=/root/.local/bin:$PATH

# Copy application code
COPY . .

# Expose port
EXPOSE 8001

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=10s --retries=3 \
    CMD python -c "import requests; requests.get('http://localhost:8001/health')" || exit 1

# Run application
CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8001", "--log-level", "info"]
