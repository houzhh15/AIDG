# HTTPS 快速开始指南

## 开发环境使用 HTTPS

### 1. 生成自签名证书

```bash
./scripts/generate-self-signed-cert.sh
```

这将在 `./certs/` 目录下生成：
- `server.crt` - TLS 证书
- `server.key` - 私钥

### 2. 启动 HTTPS 服务器

```bash
# 方法 1: 使用环境变量
SERVER_PROTOCOL=https \
TLS_CERT_FILE=./certs/server.crt \
TLS_KEY_FILE=./certs/server.key \
./bin/server

# 方法 2: 使用 .env 文件
cat > .env << EOF
SERVER_PROTOCOL=https
TLS_CERT_FILE=./certs/server.crt
TLS_KEY_FILE=./certs/server.key
EOF
./bin/server
```

### 3. 访问服务器

在浏览器中访问：
```
https://localhost:8000
```

⚠️ **注意**：浏览器会显示安全警告（因为是自签名证书），这是正常的。在开发环境中点击"继续访问"即可。

## HTTP 模式（默认）

如果不设置 `SERVER_PROTOCOL` 或设置为 `http`，服务器将使用 HTTP 模式：

```bash
# 默认 HTTP 模式
./bin/server

# 或明确指定 HTTP
SERVER_PROTOCOL=http ./bin/server
```

访问地址：
```
http://localhost:8000
```

## 环境变量说明

| 变量名 | 默认值 | 说明 |
|--------|--------|------|
| `SERVER_PROTOCOL` | `http` | 服务器协议：`http` 或 `https` |
| `TLS_CERT_FILE` | `""` | TLS 证书文件路径（HTTPS 模式必需） |
| `TLS_KEY_FILE` | `""` | TLS 私钥文件路径（HTTPS 模式必需） |
| `PORT` | `8000` | 服务器端口 |

## 生产环境部署

⚠️ **重要**：生产环境不要使用自签名证书！

推荐使用 Let's Encrypt 免费证书：

```bash
# 1. 安装 certbot
sudo apt install certbot  # Ubuntu/Debian
brew install certbot       # macOS

# 2. 获取证书
sudo certbot certonly --standalone -d yourdomain.com

# 3. 配置服务器
SERVER_PROTOCOL=https \
TLS_CERT_FILE=/etc/letsencrypt/live/yourdomain.com/fullchain.pem \
TLS_KEY_FILE=/etc/letsencrypt/live/yourdomain.com/privkey.pem \
PORT=443 \
./bin/server
```

## 故障排查

### 浏览器显示证书错误

**开发环境**：点击"高级" → "继续访问"

**生产环境**：确保使用的是受信任的 CA 证书

### 证书文件找不到

检查文件路径和权限：
```bash
ls -la ./certs/server.crt
ls -la ./certs/server.key
```

确保私钥文件权限正确：
```bash
chmod 600 ./certs/server.key
chmod 644 ./certs/server.crt
```

### 端口被占用

更改端口：
```bash
PORT=8443 \
SERVER_PROTOCOL=https \
TLS_CERT_FILE=./certs/server.crt \
TLS_KEY_FILE=./certs/server.key \
./bin/server
```

## 更多信息

详细配置说明请参阅：[docs/HTTPS_CONFIGURATION.md](../HTTPS_CONFIGURATION.md)
