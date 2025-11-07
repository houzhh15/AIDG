# AIDG 快速开始 - 5分钟上手 ⚡

> 最简单的开始方式！跟着步骤走，5分钟就能运行起来。

---

## 🚀 三步开始

### 第一步：确认 Docker 已安装

打开终端，输入：
```bash
docker --version
```

✅ 看到版本号？继续下一步！  
❌ 没有？先安装 [Docker Desktop](https://www.docker.com/products/docker-desktop)

### 第二步：下载并启动

```bash
# 克隆项目
git clone https://github.com/houzhh15/AIDG.git
cd AIDG

# 创建数据目录
mkdir -p data/{projects,users,meetings,audit_logs}

# 启动服务（基础版）
docker-compose up -d
```

### 第三步：打开浏览器

访问 http://localhost:8000

- 用户名：`admin`
- 密码：`admin123`

**搞定！** 🎉

---

## 📱 我想要会议录音功能？

需要额外3步：

```bash
# 1. 设置 HuggingFace Token（免费注册 huggingface.co）
export HUGGINGFACE_TOKEN=hf_你的token

# 2. 构建 deps-service
./scripts/build-deps-service.sh

# 3. 使用完整配置启动
docker-compose -f docker-compose.deps.yml up -d
```

---

## 🆘 遇到问题？

### 端口被占用？
```bash
# 修改 docker-compose.yml 中的端口
ports:
  - "9000:8000"  # 改成其他端口
```

### 查看日志
```bash
docker-compose logs -f
```

### 停止服务
```bash
docker-compose down
```

---

## 📚 更多信息

- 📖 **完整部署指南**: [DEPLOYMENT_GUIDE_FRIENDLY.md](DEPLOYMENT_GUIDE_FRIENDLY.md)
- 💬 **遇到问题**: [GitHub Issues](https://github.com/houzhh15/AIDG/issues)

---

**就是这么简单！** 🎊
