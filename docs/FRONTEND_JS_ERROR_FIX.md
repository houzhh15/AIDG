# 前端 JavaScript 错误修复指南

## 问题描述

访问 `http://localhost:8000` 时出现以下错误：

```
Uncaught SyntaxError: Unexpected token '<' (at config.js:1:1)
ui-vendor-BASzUKtj.js:1 Uncaught TypeError: Cannot read properties of undefined (reading 'createContext')
```

## 问题分析

### ✅ 已确认正常的部分

1. **容器运行正常** - `docker compose ps` 显示健康状态
2. **前端文件存在** - `/app/frontend/dist/` 目录包含所有构建文件
3. **config.js 存在且内容正确** - 日志显示 `GET /config.js status=200`
4. **所有 JS/CSS 资源返回 200** - 服务器正确提供静态文件

### 🔍 可能的原因

#### 1. **浏览器缓存问题**（最可能）

当 Docker 镜像更新后，浏览器可能仍在使用旧版本的缓存文件，导致：
- 请求新的 `config.js`，但浏览器使用旧的缓存
- 新旧版本的 JS chunk 不兼容
- React 依赖未正确加载

#### 2. **服务器端口混淆**

- Web Server (8000) 和 MCP Server (8081) 可能混淆
- 确保访问的是 8000 端口（前端）

#### 3. **CORS 或代理配置**

- 如果通过代理访问，可能导致请求错误
- 直接访问 `http://localhost:8000` 而非其他域名

## 解决方案

### 方案 1：清除浏览器缓存（推荐首选）

#### Chrome/Edge

1. 打开开发者工具（F12）
2. 右键点击刷新按钮
3. 选择 **"清空缓存并硬性重新加载"**（Empty Cache and Hard Reload）

或者：

1. 按 `Cmd+Shift+Delete` (Mac) / `Ctrl+Shift+Delete` (Windows)
2. 选择 "缓存的图片和文件"
3. 时间范围选择 "全部"
4. 点击 "清除数据"

#### Firefox

1. 按 `Cmd+Shift+R` (Mac) / `Ctrl+Shift+R` (Windows) 强制刷新
2. 或在开发者工具中，Network 标签右键选择 "Clear Cache"

#### Safari

1. 按 `Cmd+Option+E` 清空缓存
2. 然后按 `Cmd+R` 刷新页面

### 方案 2：使用隐私/无痕模式

```bash
# 在新的隐私窗口中打开
# Chrome: Cmd+Shift+N (Mac) / Ctrl+Shift+N (Windows)
# Firefox: Cmd+Shift+P (Mac) / Ctrl+Shift+P (Windows)
# Safari: Cmd+Shift+N
```

然后访问 `http://localhost:8000`

### 方案 3：重新构建并启动容器

如果上述方法无效，完全重新构建镜像：

```bash
# 1. 停止并删除容器和旧镜像
docker compose down
docker rmi aidg-aidg aidg

# 2. 清理构建缓存
docker builder prune -f

# 3. 重新构建镜像（不使用缓存）
docker compose build --no-cache

# 4. 启动服务
docker compose up -d

# 5. 验证构建
docker compose exec aidg ls -la /app/frontend/dist/
docker compose exec aidg cat /app/frontend/dist/config.js
```

### 方案 4：添加 Cache-Control 头（长期解决）

修改服务器配置，为静态文件添加合适的缓存策略。

检查 `cmd/server/main.go` 中的静态文件服务配置：

```go
// 示例：添加 Cache-Control 头
router.Use(func(c *gin.Context) {
    if strings.HasPrefix(c.Request.URL.Path, "/assets/") {
        // 资源文件：长期缓存
        c.Header("Cache-Control", "public, max-age=31536000, immutable")
    } else if c.Request.URL.Path == "/config.js" || 
              c.Request.URL.Path == "/" || 
              c.Request.URL.Path == "/index.html" {
        // HTML 和配置文件：不缓存
        c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
        c.Header("Pragma", "no-cache")
        c.Header("Expires", "0")
    }
    c.Next()
})
```

## 验证修复

### 1. 检查网络请求

在浏览器开发者工具中（F12）：

1. 打开 **Network** 标签
2. 勾选 **"Disable cache"**
3. 刷新页面
4. 确认所有请求返回 **200 状态码**：
   - `/config.js` → 200
   - `/assets/react-vendor-*.js` → 200
   - `/assets/ui-vendor-*.js` → 200
   - `/assets/index-*.js` → 200

### 2. 检查控制台

在 **Console** 标签中：
- ✅ **无红色错误信息**
- ✅ **无 `Unexpected token '<'` 错误**
- ✅ **无 `Cannot read properties of undefined` 错误**
- ✅ 应用正常加载并显示登录界面

### 3. 验证 config.js 加载

在 Console 中运行：

```javascript
console.log(window.CONFIG);
```

应该输出：

```javascript
{
  // 配置对象，即使字段被注释也应该存在
}
```

### 4. 验证 React 加载

在 Console 中运行：

```javascript
console.log(typeof React);
```

应该输出：`object` 或 `undefined`（如果 React 未全局暴露，这是正常的）

但**不应该**出现错误。

## 常见问题

### Q1: 清除缓存后仍然报错？

**A:** 尝试：
1. 完全关闭浏览器，重新打开
2. 使用不同的浏览器测试
3. 检查是否有浏览器插件干扰（禁用所有插件后测试）

### Q2: 隐私模式下正常，普通模式下报错？

**A:** 这确认了是缓存问题。解决方法：
1. 在普通模式下，手动清除 `localhost:8000` 的所有数据
2. Chrome: 地址栏左侧锁图标 → "网站设置" → "清除数据"
3. 刷新页面

### Q3: 所有浏览器都报同样的错误？

**A:** 可能是服务器端问题：
1. 检查服务器日志：`docker compose logs aidg --tail=100`
2. 验证静态文件服务配置
3. 确认 `/app/frontend/dist/` 目录权限正确

### Q4: 能否通过配置避免此问题？

**A:** 是的，在 `vite.config.ts` 中配置输出文件名带哈希：

```typescript
export default defineConfig({
  build: {
    rollupOptions: {
      output: {
        entryFileNames: 'assets/[name]-[hash].js',
        chunkFileNames: 'assets/[name]-[hash].js',
        assetFileNames: 'assets/[name]-[hash].[ext]'
      }
    }
  }
});
```

这样每次构建都会生成新的文件名，浏览器不会使用旧缓存。

## 预防措施

### 开发环境

1. **始终开启 "Disable cache"**
   - Chrome DevTools → Network → ✅ Disable cache

2. **使用 Vite 开发服务器**
   ```bash
   cd frontend
   npm run dev
   ```
   访问 `http://localhost:5173`（Vite 开发服务器自动处理缓存）

### 生产环境

1. **使用版本化的静态资源**
   - Vite 默认在文件名中包含哈希值
   - 确保构建配置正确

2. **配置合适的 Cache-Control 头**
   - HTML/config.js: `no-cache`
   - JS/CSS assets: `max-age=31536000, immutable`

3. **使用 CDN 或反向代理**
   - 配置缓存策略
   - 支持缓存清除（purge）

## 调试命令

```bash
# 检查容器内文件
docker compose exec aidg ls -lah /app/frontend/dist/
docker compose exec aidg cat /app/frontend/dist/config.js
docker compose exec aidg cat /app/frontend/dist/index.html

# 检查服务器日志
docker compose logs aidg --tail=100 -f

# 测试静态文件访问
curl -I http://localhost:8000/config.js
curl -I http://localhost:8000/assets/index-BgU87EmW.js

# 检查 Content-Type
curl -I http://localhost:8000/config.js | grep -i content-type
# 应该返回: content-type: application/javascript 或 text/javascript
```

## 总结

**最常见原因**：浏览器缓存了旧版本的 JS 文件

**最快解决方案**：
1. 清空浏览器缓存并硬性重新加载
2. 或使用隐私/无痕模式访问

**长期解决方案**：
1. 配置适当的 HTTP 缓存头
2. 确保 Vite 构建使用哈希文件名
3. 开发时始终开启 "Disable cache"

---

**最后更新**：2025-10-08  
**相关文档**：
- [Docker 构建故障排除](./DOCKER_BUILD_TROUBLESHOOTING.md)
- [Docker Compose 故障排除](./DOCKER_COMPOSE_TROUBLESHOOTING.md)
