# HTTPS Configuration Guide

This guide explains how to configure the AIDG server to use HTTPS instead of HTTP.

## Quick Start

### 1. Generate Self-Signed Certificate (Development)

For development and testing, you can generate a self-signed certificate:

```bash
./scripts/generate-self-signed-cert.sh
```

This will create:
- `./certs/server.crt` - TLS certificate
- `./certs/server.key` - Private key

### 2. Configure Environment Variables

Set the following environment variables:

```bash
export SERVER_PROTOCOL=https
export TLS_CERT_FILE=./certs/server.crt
export TLS_KEY_FILE=./certs/server.key
```

Or add them to your `.env` file:

```env
SERVER_PROTOCOL=https
TLS_CERT_FILE=./certs/server.crt
TLS_KEY_FILE=./certs/server.key
```

### 3. Start the Server

```bash
./bin/server
```

The server will now run on HTTPS. Access it at:
- `https://localhost:8000` (or your configured port)

## Configuration Options

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `SERVER_PROTOCOL` | `http` | Server protocol: `http` or `https` |
| `TLS_CERT_FILE` | `""` | Path to TLS certificate file (required for HTTPS) |
| `TLS_KEY_FILE` | `""` | Path to TLS private key file (required for HTTPS) |
| `PORT` | `8000` | Server port |

## Production Deployment

⚠️ **Important:** Self-signed certificates should NOT be used in production.

For production, use certificates from a trusted Certificate Authority (CA):

### Option 1: Let's Encrypt (Free)

1. Install Certbot:
   ```bash
   # Ubuntu/Debian
   sudo apt install certbot
   
   # macOS
   brew install certbot
   ```

2. Generate certificate:
   ```bash
   sudo certbot certonly --standalone -d yourdomain.com
   ```

3. Configure server:
   ```bash
   export SERVER_PROTOCOL=https
   export TLS_CERT_FILE=/etc/letsencrypt/live/yourdomain.com/fullchain.pem
   export TLS_KEY_FILE=/etc/letsencrypt/live/yourdomain.com/privkey.pem
   ```

### Option 2: Commercial CA

Purchase a certificate from a commercial CA (DigiCert, Comodo, etc.) and configure:

```bash
export SERVER_PROTOCOL=https
export TLS_CERT_FILE=/path/to/certificate.crt
export TLS_KEY_FILE=/path/to/private.key
```

## Reverse Proxy Configuration

### nginx

If using nginx as a reverse proxy, handle TLS termination at nginx level:

```nginx
server {
    listen 443 ssl http2;
    server_name yourdomain.com;
    
    ssl_certificate /path/to/certificate.crt;
    ssl_certificate_key /path/to/private.key;
    
    # Modern SSL configuration
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;
    ssl_prefer_server_ciphers on;
    
    location / {
        proxy_pass http://localhost:8000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

In this case, run the backend server in HTTP mode:
```bash
export SERVER_PROTOCOL=http
```

## Troubleshooting

### Browser Shows Security Warning

This is normal for self-signed certificates. Options:

1. **Accept the warning** (development only)
2. **Add certificate to system trust store** (macOS):
   ```bash
   sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain ./certs/server.crt
   ```
3. Use a trusted certificate for production

### Certificate Error: "cannot validate certificate"

Ensure:
- Certificate file path is correct
- Private key file path is correct
- Files have proper permissions (key: 600, cert: 644)
- Certificate is not expired

### Port Already in Use

If port 443 is already in use:
```bash
export PORT=8443  # Use alternative HTTPS port
```

## Security Best Practices

1. **Never commit private keys** to version control
   - Add `*.key` to `.gitignore`
   - Add `certs/` directory to `.gitignore`

2. **Use strong encryption**
   - Modern TLS versions (1.2+)
   - Strong cipher suites
   - 2048-bit or 4096-bit RSA keys

3. **Rotate certificates regularly**
   - Set up automatic renewal (e.g., certbot cron job)
   - Monitor certificate expiration

4. **Restrict file permissions**
   - Private keys: `chmod 600`
   - Certificates: `chmod 644`

5. **Use HTTPS everywhere**
   - Redirect HTTP to HTTPS
   - Set HSTS headers
   - Use secure cookies

## Examples

### Development Setup
```bash
# Generate self-signed certificate
./scripts/generate-self-signed-cert.sh

# Run with HTTPS
SERVER_PROTOCOL=https \
TLS_CERT_FILE=./certs/server.crt \
TLS_KEY_FILE=./certs/server.key \
./bin/server
```

### Production Setup (Let's Encrypt)
```bash
# After obtaining Let's Encrypt certificate
SERVER_PROTOCOL=https \
TLS_CERT_FILE=/etc/letsencrypt/live/yourdomain.com/fullchain.pem \
TLS_KEY_FILE=/etc/letsencrypt/live/yourdomain.com/privkey.pem \
PORT=443 \
./bin/server
```

### Docker Deployment
```dockerfile
# Mount certificates as volumes
docker run -d \
  -p 443:443 \
  -e SERVER_PROTOCOL=https \
  -e TLS_CERT_FILE=/certs/server.crt \
  -e TLS_KEY_FILE=/certs/server.key \
  -e PORT=443 \
  -v /path/to/certs:/certs:ro \
  aidg-server
```

## References

- [OpenSSL Documentation](https://www.openssl.org/docs/)
- [Let's Encrypt](https://letsencrypt.org/)
- [Mozilla SSL Configuration Generator](https://ssl-config.mozilla.org/)
