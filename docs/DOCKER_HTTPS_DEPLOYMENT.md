# Docker HTTPS 部署指南

本指南说明如何使用 Docker Compose 部署支持 HTTPS 的 AIDG 服务。

## 快速开始

### 方法 1: 使用环境变量（推荐）

```bash
# 1. 生成自签名证书（开发环境）
./scripts/generate-self-signed-cert.sh

# 2. 创建 .env 文件
cp .env.https.example .env
# 编辑 .env 文件，设置你的配置

# 3. 启动服务
docker-compose -f docker-compose.ghcr.yml up -d

# 4. 访问服务
# https://localhost:443
```

### 方法 2: 使用 docker-compose.https.yml 覆盖文件

```bash
# 1. 生成自签名证书
./scripts/generate-self-signed-cert.sh

# 2. 启动 HTTPS 服务
docker-compose -f docker-compose.ghcr.yml -f docker-compose.https.yml up -d

# 3. 访问服务
# https://localhost:443
```

## 部署场景

### 场景 1: 开发环境（自签名证书）

```bash
# 生成自签名证书
./scripts/generate-self-signed-cert.sh

# 启动开发环境
SERVER_PROTOCOL=https \
SERVER_PORT=8443 \
TLS_CERT_FILE=/app/certs/server.crt \
TLS_KEY_FILE=/app/certs/server.key \
docker-compose -f docker-compose.ghcr.yml up -d

# 访问：https://localhost:8443
```

### 场景 2: 生产环境（Let's Encrypt 证书）

```bash
# 1. 获取 Let's Encrypt 证书
sudo certbot certonly --standalone -d yourdomain.com

# 2. 创建证书符号链接或复制到 certs 目录
mkdir -p certs
sudo cp /etc/letsencrypt/live/yourdomain.com/fullchain.pem ./certs/server.crt
sudo cp /etc/letsencrypt/live/yourdomain.com/privkey.pem ./certs/server.key
sudo chown $(id -u):$(id -g) ./certs/server.*

# 3. 配置环境变量
cat > .env << EOF
SERVER_PROTOCOL=https
SERVER_PORT=443
TLS_CERT_FILE=/app/certs/server.crt
TLS_KEY_FILE=/app/certs/server.key
IMAGE_TAG=v0.1.14
ENV=production
LOG_LEVEL=info
LOG_FORMAT=json
USER_JWT_SECRET=$(openssl rand -base64 32)
ADMIN_DEFAULT_PASSWORD=$(openssl rand -base64 16)
MCP_PASSWORD=$(openssl rand -base64 16)
CORS_ALLOWED_ORIGINS=https://yourdomain.com
EOF

# 4. 启动生产服务
docker-compose -f docker-compose.ghcr.yml up -d

# 5. 设置证书自动续期
sudo crontab -e
# 添加: 0 0 * * * certbot renew --quiet && docker-compose -f /path/to/docker-compose.ghcr.yml restart
```

### 场景 3: 使用 Nginx 反向代理（推荐生产环境）

Nginx 处理 HTTPS，后端使用 HTTP：

```nginx
# /etc/nginx/sites-available/aidg
server {
    listen 443 ssl http2;
    server_name yourdomain.com;
    
    # SSL 配置
    ssl_certificate /etc/letsencrypt/live/yourdomain.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/yourdomain.com/privkey.pem;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;
    ssl_prefer_server_ciphers on;
    
    # 反向代理到后端
    location / {
        proxy_pass http://localhost:8000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}

# HTTP 重定向到 HTTPS
server {
    listen 80;
    server_name yourdomain.com;
    return 301 https://$server_name$request_uri;
}
```

后端使用 HTTP 模式：

```bash
# .env 文件
SERVER_PROTOCOL=http
SERVER_PORT=8000
# 其他配置...
```

## 端口说明

| 端口 | 用途 | 说明 |
|------|------|------|
| 443 | HTTPS 标准端口 | 需要 root 权限或 CAP_NET_BIND_SERVICE |
| 8443 | HTTPS 备用端口 | 非特权端口，开发推荐 |
| 8000 | HTTP 默认端口 | 或用于反向代理后端 |
| 8081 | MCP 服务端口 | AI 接口 |

## Docker Compose 文件说明

### docker-compose.ghcr.yml
基础配置文件，支持通过环境变量配置 HTTP/HTTPS。

### docker-compose.https.yml
HTTPS 覆盖配置，预配置了 HTTPS 相关设置。

使用方式：
```bash
docker-compose -f docker-compose.ghcr.yml -f docker-compose.https.yml up -d
```

## 证书管理

### 使用自签名证书

```bash
# 生成证书
./scripts/generate-self-signed-cert.sh

# 证书位置
ls -la ./certs/
# server.crt - 公钥证书
# server.key - 私钥
```

### 使用 Let's Encrypt

```bash
# 安装 certbot
sudo apt install certbot  # Ubuntu/Debian
brew install certbot       # macOS

# 获取证书
sudo certbot certonly --standalone -d yourdomain.com

# 证书位置
# /etc/letsencrypt/live/yourdomain.com/fullchain.pem
# /etc/letsencrypt/live/yourdomain.com/privkey.pem

# 自动续期（90天有效期）
sudo certbot renew --quiet
```

### 证书权限

```bash
# 确保 Docker 可以读取证书
chmod 644 ./certs/server.crt
chmod 600 ./certs/server.key
```

## 常用命令

```bash
# 查看日志
docker-compose -f docker-compose.ghcr.yml logs -f

# 重启服务
docker-compose -f docker-compose.ghcr.yml restart

# 停止服务
docker-compose -f docker-compose.ghcr.yml down

# 更新到最新版本
docker-compose -f docker-compose.ghcr.yml pull
docker-compose -f docker-compose.ghcr.yml up -d

# 查看服务状态
docker-compose -f docker-compose.ghcr.yml ps

# 进入容器
docker exec -it aidg-unified /bin/sh
```

## 健康检查

```bash
# HTTP 模式
curl http://localhost:8000/health

# HTTPS 模式（忽略证书验证）
curl -k https://localhost:443/health

# HTTPS 模式（验证证书）
curl https://localhost:443/health
```

## 故障排查

### 证书错误

```bash
# 检查证书文件是否存在
docker exec aidg-unified ls -la /app/certs/

# 检查证书有效期
openssl x509 -in ./certs/server.crt -noout -dates

# 查看服务器日志
docker-compose -f docker-compose.ghcr.yml logs aidg
```

### 端口冲突

```bash
# 检查端口占用
sudo lsof -i :443
sudo lsof -i :8443

# 使用其他端口
SERVER_PORT=8443 docker-compose -f docker-compose.ghcr.yml up -d
```

### 权限问题

```bash
# 443 端口需要特权，使用 8443 端口
SERVER_PORT=8443 docker-compose -f docker-compose.ghcr.yml up -d

# 或使用 root 运行
sudo docker-compose -f docker-compose.ghcr.yml up -d
```

## 安全建议

1. **生产环境使用可信证书**：Let's Encrypt 或商业 CA
2. **定期更新证书**：设置自动续期
3. **保护私钥**：严格的文件权限（600）
4. **更改默认密码**：USER_JWT_SECRET, ADMIN_DEFAULT_PASSWORD, MCP_PASSWORD
5. **配置防火墙**：只开放必要端口
6. **使用反向代理**：Nginx 或 Traefik 处理 TLS
7. **启用 HSTS**：强制 HTTPS
8. **监控证书过期**：提前续期

## 更多信息

- [HTTPS 配置详细文档](./HTTPS_CONFIGURATION.md)
- [HTTPS 快速开始](./HTTPS_QUICK_START.md)
- [Docker 部署指南](./DEPLOYMENT_GUIDE_FRIENDLY.md)
