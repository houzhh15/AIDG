#!/bin/bash

# Generate self-signed certificate for HTTPS development
# Usage: ./scripts/generate-self-signed-cert.sh [output_dir]

set -e

# Default output directory
OUTPUT_DIR="${1:-./certs}"
CERT_FILE="${OUTPUT_DIR}/server.crt"
KEY_FILE="${OUTPUT_DIR}/server.key"
DAYS=365

# Create output directory if it doesn't exist
mkdir -p "${OUTPUT_DIR}"

echo "========================================="
echo "Generating self-signed TLS certificate"
echo "========================================="
echo "Output directory: ${OUTPUT_DIR}"
echo "Certificate file: ${CERT_FILE}"
echo "Private key file: ${KEY_FILE}"
echo "Valid for: ${DAYS} days"
echo ""

# Generate private key and self-signed certificate
openssl req -x509 -newkey rsa:4096 -keyout "${KEY_FILE}" -out "${CERT_FILE}" \
    -days ${DAYS} -nodes \
    -subj "/C=US/ST=State/L=City/O=Organization/OU=Department/CN=localhost" \
    -addext "subjectAltName=DNS:localhost,DNS:*.localhost,IP:127.0.0.1,IP:0.0.0.0"

echo ""
echo "========================================="
echo "✅ Certificate generated successfully!"
echo "========================================="
echo ""
echo "To use HTTPS mode, set the following environment variables:"
echo ""
echo "  export SERVER_PROTOCOL=https"
echo "  export TLS_CERT_FILE=${CERT_FILE}"
echo "  export TLS_KEY_FILE=${KEY_FILE}"
echo ""
echo "Or add to your .env file:"
echo ""
echo "  SERVER_PROTOCOL=https"
echo "  TLS_CERT_FILE=${CERT_FILE}"
echo "  TLS_KEY_FILE=${KEY_FILE}"
echo ""
echo "⚠️  Note: This is a self-signed certificate for development only."
echo "    Browsers will show a security warning. For production, use a"
echo "    certificate from a trusted Certificate Authority (CA)."
echo ""

# Set appropriate permissions
chmod 600 "${KEY_FILE}"
chmod 644 "${CERT_FILE}"

echo "File permissions set:"
echo "  ${KEY_FILE} (600 - private key)"
echo "  ${CERT_FILE} (644 - public certificate)"
echo ""

# Display certificate information
echo "Certificate information:"
openssl x509 -in "${CERT_FILE}" -text -noout | grep -A 2 "Subject:"
openssl x509 -in "${CERT_FILE}" -text -noout | grep -A 1 "Validity"
echo ""
