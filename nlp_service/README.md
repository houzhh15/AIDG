# NLP Service for AI Dev Gov

基于 FastAPI 的文本向量化服务，使用 `text2vec-base-chinese` 模型生成中文文本的语义向量。

## 功能特性

- **文本向量化**：将中文文本转换为 768 维语义向量
- **批处理支持**：单次请求最多处理 100 条文本
- **CORS 支持**：允许跨域请求
- **健康检查**：提供服务健康状态监控
- **自动文档**：FastAPI 自动生成交互式 API 文档

## 技术栈

- **框架**：FastAPI 0.104+
- **模型**：sentence-transformers (text2vec-base-chinese)
- **服务器**：Uvicorn
- **Python**：3.10+

## 快速开始

### 方式一：本地部署（推荐用于开发）

```bash
cd nlp_service
chmod +x scripts/local-deploy.sh
./scripts/local-deploy.sh
```

脚本会自动完成以下操作：
1. 创建 Python 虚拟环境
2. 安装依赖
3. 下载 text2vec-base-chinese 模型
4. 启动服务（默认端口 5000）

备注：需要先执行 huggingface-cli login

### 方式二：Docker 部署（推荐用于生产）

```bash
cd nlp_service
chmod +x scripts/build-docker.sh
./scripts/build-docker.sh

# 运行容器
docker run -d -p 5000:5000 --name nlp-service aidg-nlp-service:latest

# 查看日志
docker logs -f nlp-service
```

### 方式三：手动部署

```bash
cd nlp_service

# 创建虚拟环境
python3 -m venv venv
source venv/bin/activate

# 安装依赖
pip install -r requirements.txt

# 运行服务
python app.py
# 或使用 uvicorn
uvicorn app:app --host 0.0.0.0 --port 5000 --reload
```

## API 使用

### 健康检查

```bash
curl http://localhost:5000/health
```

**响应示例**：
```json
{
  "status": "healthy",
  "model_loaded": true,
  "model_name": "text2vec-base-chinese"
}
```

### 文本向量化

**接口**：`POST /nlp/embed`

**请求示例**：
```bash
curl -X POST http://localhost:5000/nlp/embed \
  -H "Content-Type: application/json" \
  -d '{
    "texts": ["这是第一段文本", "这是第二段文本"],
    "model": "text2vec-base-chinese"
  }'
```

**响应示例**：
```json
{
  "embeddings": [
    [0.123, -0.456, 0.789, ...],
    [0.234, -0.567, 0.890, ...]
  ],
  "model": "text2vec-base-chinese",
  "dim": 768
}
```

**参数说明**：
- `texts`（必填）：文本列表，最少 1 条，最多 100 条
- `model`（可选）：模型名称，默认 `text2vec-base-chinese`

**错误处理**：
- `400 Bad Request`：请求参数错误（如批次大小超过 100）
- `500 Internal Server Error`：向量化失败
- `503 Service Unavailable`：模型未加载

## 交互式 API 文档

服务启动后，访问以下 URL 查看自动生成的 API 文档：

- **Swagger UI**：http://localhost:5000/docs
- **ReDoc**：http://localhost:5000/redoc

## 性能优化

- **批处理大小**：32（在内存和速度之间平衡）
- **模型加载**：启动时预加载，避免首次请求延迟
- **Docker 多阶段构建**：减小镜像大小
- **健康检查**：30 秒间隔，支持 Kubernetes/Docker Compose

## 目录结构

```
nlp_service/
├── app.py                 # FastAPI 应用主文件
├── requirements.txt       # Python 依赖
├── Dockerfile            # Docker 容器配置
├── README.md             # 本文档
└── scripts/
    ├── build-docker.sh   # Docker 镜像构建脚本
    └── local-deploy.sh   # 本地部署脚本
```

## 集成到 AIDG 系统

NLP Service 作为 AIDG 系统的独立微服务，通过 HTTP API 与 Go 后端集成：

```go
// Go 客户端调用示例
type NLPClient struct {
    baseURL string
    client  *http.Client
}

func (c *NLPClient) Embed(texts []string) ([][]float64, error) {
    req := map[string]interface{}{
        "texts": texts,
        "model": "text2vec-base-chinese",
    }
    // ... HTTP POST 请求逻辑
}
```

## 依赖说明

- **sentence-transformers**：提供预训练模型加载和推理
- **transformers**：Hugging Face Transformers 库
- **torch**：PyTorch 深度学习框架
- **fastapi**：现代 Web 框架
- **uvicorn**：ASGI 服务器

## 常见问题

**Q: 首次启动很慢？**  
A: 首次运行会下载约 400MB 的模型文件，后续启动会使用缓存。

**Q: 内存占用多大？**  
A: 模型加载后约占用 1.5GB 内存，建议预留 2GB。

**Q: 支持 GPU 加速吗？**  
A: 支持，安装 `torch` GPU 版本即可自动启用。

**Q: 如何修改端口？**  
A: 编辑 `scripts/local-deploy.sh` 中的 `PORT` 变量，或在 Docker 运行时映射不同端口。

## 监控和日志

- **日志级别**：INFO（可在 `app.py` 中调整）
- **日志格式**：包含时间戳、日志名称、级别和消息
- **健康检查**：`/health` 端点返回模型加载状态

## 版本历史

- **v1.0.0**（2025-11-02）：初始版本
  - 实现 POST /nlp/embed 接口
  - 支持批处理和 CORS
  - 提供 Docker 和本地部署脚本

## 许可证

本项目是 AIDG 系统的一部分，遵循相同的许可证。
