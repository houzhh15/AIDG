# Docker 构建故障排查指南

## 问题 1: Go embed 文件未找到

### 错误信息
```
cmd/mcp-se## 问题 3: npm 平台依赖错误

### 错误信息
```
npm error code EBADPLATFORM
npm error notsup Unsupported platform for @rollup/rollup-darwin-arm64@4.50.2
npm error notsup wanted {"os":"darwin","cpu":"arm64"} (current: {"os":"linux","cpu":"arm64"})
```

### 原因
`@rollup/rollup-darwin-arm64` 被错误地添加到 `frontend/package.json` 的 `devDependencies` 中。这是平台特定的包，不应该显式声明，应该由 Rollup 根据当前平台自动选择。

当在 macOS 上运行 `npm install` 时，某些包管理器可能会错误地将平台特定的可选依赖添加到 package.json 中。te.go:5:12: pattern task.prompt.md: no matching files found
```

### 原因
Go 代码中使用 `//go:embed task.prompt.md` 引用了不存在的文件。这通常是：
- 重构时遗留的代码
- 文件被移动或删除但 embed 引用未更新
- Go 1.22+ 对 embed 检查更严格

### 解决方案
✅ 已修复：删除未使用的 `template.go` 文件：
```bash
rm cmd/mcp-server/template.go
```

**替代方案**：
- 如果需要这个模板，创建对应的 `.md` 文件
- 如果只是未使用的代码，直接删除

---

## 问题 2: Go 版本不匹配

### 错误信息
```
go: go.mod requires go >= 1.22 (running go 1.21.13; GOTOOLCHAIN=local)
```

### 原因
`go.mod` 文件要求 Go 1.22+，但 Dockerfile 使用的是 `golang:1.21-alpine` 镜像。

### 解决方案
✅ 已修复：更新 Dockerfile 中的 Go 版本：
```dockerfile
# 修改前
FROM golang:1.21-alpine AS backend-builder

# 修改后
FROM golang:1.22-alpine AS backend-builder
```

---

## 问题 3: supervisord.conf 文件未找到

### 错误信息
```
ERROR [stage-2 9/9] COPY deployments/docker/supervisord.conf /etc/supervisord.conf
```

### 原因
`.dockerignore` 文件中排除了整个 `deployments/` 目录。

### 解决方案
✅ 已修复：在 `.dockerignore` 中添加例外规则：
```
!deployments/docker/supervisord.conf
```

---

## 问题 4: npm 平台依赖错误

### 错误信息
```
npm error code EBADPLATFORM
npm error notsup Unsupported platform for @rollup/rollup-darwin-arm64@4.50.2
npm error notsup wanted {"os":"darwin","cpu":"arm64"} (current: {"os":"linux","cpu":"arm64"})
```

### 原因
`npm ci` 严格检查 `package-lock.json` 中的平台特定依赖。当在 macOS 上生成 lock 文件，然后在 Linux Docker 容器中构建时，会出现平台不匹配错误。

### 解决方案

#### 方案 1: 从 package.json 中删除平台特定包 (已采用) ✅

从 `frontend/package.json` 的 `devDependencies` 中删除 `@rollup/rollup-darwin-arm64`：

```diff
 "devDependencies": {
-  "@rollup/rollup-darwin-arm64": "^4.50.1",
   "@types/node": "^20.14.2",
```

**原理**:
- Rollup 和 Vite 会根据当前平台自动安装正确的原生模块
- 在 macOS 上会安装 `@rollup/rollup-darwin-arm64`
- 在 Linux 上会安装 `@rollup/rollup-linux-arm64`
- 不需要在 package.json 中显式声明

**优点**:
- ✅ 彻底解决跨平台构建问题
- ✅ package.json 更干净，平台无关
- ✅ 符合最佳实践
- ✅ 开发和生产环境都能正常工作

#### 方案 2: 删除 lock 文件 (备用方案)

#### 方案 2: 排除 package-lock.json (额外保护)

在 `.dockerignore` 中排除 lock 文件：

```
**/package-lock.json
```

**说明**: 虽然 Dockerfile 中已经删除了 lock 文件，在 .dockerignore 中排除可以作为额外保护并减小构建上下文

#### 方案 3: 在容器中重新生成 lock 文件 (等同于方案 1)

在 Docker 构建中重新生成 lock 文件：

```dockerfile
RUN rm -f package-lock.json && npm install
```

**优点**:
- ✅ 确保平台兼容

**缺点**:
- ❌ 构建时间长
- ❌ 失去 lock 文件的版本锁定优势

#### 方案 4: 使用 --force 或 --legacy-peer-deps (不推荐) ❌

```dockerfile
RUN npm ci --force
# 或
RUN npm install --force
```

**问题**: 
- ❌ 强制安装可能不兼容的依赖
- ❌ 隐藏真实的兼容性问题
- ❌ 可能导致运行时错误

---

## 当前配置 (推荐方案)

### frontend/package.json

**关键点**: 不要在 `devDependencies` 中添加平台特定的包

```json
{
  "devDependencies": {
    // ❌ 不要这样做
    // "@rollup/rollup-darwin-arm64": "^4.50.1",
    
    // ✅ 只声明主包，让工具自动选择平台版本
    "@types/node": "^20.14.2",
    "@types/react": "^18.2.22",
    "vite": "^5.2.0"
  }
}
```

### Dockerfile (frontend-builder stage)

```dockerfile
# Stage 2: Build frontend
FROM node:18-alpine AS frontend-builder

WORKDIR /app/frontend

# Copy package files
COPY frontend/package*.json ./

# Install dependencies
# Note: Platform-specific packages like @rollup/rollup-linux-arm64 are auto-selected by npm
RUN npm install --no-fund --no-audit

# Copy frontend source
COPY frontend/ ./

# Build frontend (production mode)
RUN npm run build
```

**工作原理**:
1. npm 读取 `package.json` 和 `package-lock.json`
2. Vite/Rollup 作为依赖被安装时，会自动检测当前平台
3. 自动下载并安装适合当前平台的原生模块（如 `@rollup/rollup-linux-arm64`）
4. 无需手动指定或删除 lock 文件

### .dockerignore

```ignore
# Frontend development
frontend/node_modules/
frontend/dist/
frontend/.vite/
frontend/tsconfig.tsbuildinfo
**/package-lock.json  # Exclude to avoid platform-specific lock issues in Docker
```

---

## 验证构建

### 使用测试脚本

```bash
./test-docker-build.sh
```

### 手动构建测试

```bash
# 清理之前的构建缓存
docker builder prune -a

# 重新构建
docker build -t aidg:test .

# 或使用 Makefile
make docker-build VERSION=test
```

---

## 最佳实践

### 跨平台开发建议

1. **使用 npm install 在 Docker 中**
   - Docker 构建使用 `npm install`
   - 本地开发使用 `npm ci` 或 `npm install`

2. **定期更新依赖**
   ```bash
   npm update
   npm audit fix
   ```

3. **测试 Docker 构建**
   - 在推送前本地测试 Docker 构建
   - 使用 CI/CD 自动化测试

4. **文档化平台差异**
   - 记录已知的平台特定包
   - 在 README 中说明构建要求

### 避免常见错误

❌ **不要这样做**:
```dockerfile
RUN npm ci  # 在跨平台开发中会因为平台不匹配失败
RUN npm install --production=false  # 如果 lock 文件存在仍会检查平台
```

✅ **应该这样做**:
```dockerfile
RUN rm -f package-lock.json && npm install --production=false  # 删除 lock 文件，让 npm 重新解析
```

### 本地开发与 Docker 构建的差异

| 环境 | 使用命令 | Lock 文件 | 依赖版本 |
|------|---------|----------|----------|
| **本地开发** (macOS) | `npm install` 或 `npm ci` | 保留并使用 | macOS 特定包 (darwin-arm64) |
| **Docker 构建** (Linux) | `rm -f package-lock.json && npm install` | 删除后重新生成 | Linux 特定包 (linux-arm64) |

这种差异是**正常且预期的**，因为不同操作系统需要不同的原生模块。

---

## 问题 6: TypeScript 类型错误 - import.meta.env

### 错误信息
```
src/config/env.ts(24,58): error TS2339: Property 'env' does not exist on type 'ImportMeta'.
```

### 原因
TypeScript 不知道 `import.meta.env` 的类型定义。Vite 项目需要 `vite-env.d.ts` 类型声明文件来定义环境变量的类型。

### 解决方案
✅ 已修复：创建 `frontend/src/vite-env.d.ts` 文件：

```typescript
/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_API_BASE_URL?: string;
  readonly VITE_APP_TITLE?: string;
  readonly VITE_APP_VERSION?: string;
  readonly VITE_LOG_LEVEL?: string;
  // 更多环境变量...
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
```

**工作原理**：
1. `/// <reference types="vite/client" />` 引入 Vite 的基础类型
2. `ImportMetaEnv` 接口定义所有 Vite 环境变量（VITE_ 前缀）
3. `ImportMeta` 接口扩展了 `import.meta` 的类型，添加 `env` 属性

**注意事项**：
- 这个文件应该在 `tsconfig.json` 的 `include` 范围内（如 `src/`）
- 每次添加新的环境变量时，需要更新此接口
- TypeScript 会在编译时检查环境变量的使用是否正确

---

## 问题 7: Vite 缺少 terser 依赖

### 错误信息
```
error during build:
[vite:terser] terser not found. Since Vite v3, terser has become an optional dependency. You need to install it.
```

### 原因
从 Vite 3 开始，`terser` 变成了可选依赖。如果在生产构建时使用 terser 进行代码压缩（默认行为），则需要显式安装。

### 解决方案
✅ 已修复：在 `frontend/package.json` 的 `devDependencies` 中添加 terser：

```json
{
  "devDependencies": {
    "terser": "^5.36.0",
    // ... 其他依赖
  }
}
```

**替代方案**：
如果不想使用 terser，可以在 `vite.config.ts` 中配置使用 esbuild 压缩：

```typescript
export default defineConfig({
  build: {
    minify: 'esbuild',  // 使用 esbuild 而不是 terser
  }
})
```

**对比**：
- **terser**: 压缩效果更好，但速度较慢
- **esbuild**: 速度更快，但压缩效果稍差

---

## 问题 8: docker-compose 引用错误的 Dockerfile

### 错误信息
```
failed to solve: failed to read dockerfile: open Dockerfile.unified: no such file or directory
```

### 原因
`docker-compose.yml` 中仍然引用旧的 `Dockerfile.unified`，但该文件已被重命名为 `Dockerfile`。

另外，Docker Compose v2 不再需要 `version` 字段，该字段已过时。

### 解决方案
✅ 已修复：更新 `docker-compose.yml` 和 `docker-compose.prod.yml`：

```diff
-version: '3.8'
-
 services:
   aidg:
     build:
       context: .
-      dockerfile: Dockerfile.unified
+      dockerfile: Dockerfile
```

**注意**：
- Docker Compose v2（`docker compose`）不需要 `version` 字段
- Docker Compose v1（`docker-compose`）需要 `version` 字段，但已被弃用
- 推荐使用 Docker Compose v2（内置在 Docker Desktop 中）

---

## 相关资源

- [npm ci 文档](https://docs.npmjs.com/cli/v8/commands/npm-ci)
- [Docker .dockerignore 文档](https://docs.docker.com/engine/reference/builder/#dockerignore-file)
- [多阶段构建最佳实践](https://docs.docker.com/develop/develop-images/multistage-build/)
- [Vite 环境变量文档](https://vitejs.dev/guide/env-and-mode.html)
- [TypeScript 类型声明](https://www.typescriptlang.org/docs/handbook/declaration-files/introduction.html)
- [Vite 构建优化](https://vitejs.dev/guide/build.html)

---

## 更新日志

- **2025-10-08**: 修复 supervisord.conf 路径问题
- **2025-10-08**: 修复 Go 版本不匹配问题（1.21 → 1.22）
- **2025-10-08**: 删除未使用的 template.go 文件
- **2025-10-08**: 修复 npm 平台依赖问题
- **2025-10-08**: 创建 vite-env.d.ts 类型声明文件
- **2025-10-08**: 添加 terser 依赖到 package.json
- **2025-10-08**: 修复 docker-compose.yml 引用错误的 Dockerfile
