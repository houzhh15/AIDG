# React createContext 错误深度诊断指南

## 错误详情

```
Uncaught TypeError: Cannot read properties of undefined (reading 'createContext')
    at ui-vendor-BASzUKtj.js:1:1676
```

## 问题分析

这个错误表明 `ui-vendor` chunk 试图访问 React，但 React 还未被定义或加载。

### 可能的原因

1. **模块加载顺序问题** - `ui-vendor` 在 `react-vendor` 之前执行
2. **Vite 代码分割配置** - manualChunks 策略导致依赖关系错误
3. **浏览器缓存混乱** - 新旧版本文件混合

## 诊断步骤

### 步骤 1: 在浏览器中检查加载顺序

1. 打开浏览器开发者工具（F12）
2. 进入 **Network** 标签
3. 勾选 **"Disable cache"**
4. 刷新页面
5. 查看 JS 文件的加载顺序：

应该的正确顺序：
```
1. react-vendor-*.js      (React 和 ReactDOM)
2. ui-vendor-*.js          (UI 库，依赖 React)
3. vendor-*.js             (其他库)
4. index-*.js              (应用代码)
```

如果 `ui-vendor` 在 `react-vendor` **之前**加载，这就是问题所在。

### 步骤 2: 检查 modulepreload

在 Network 标签中，查看 `index.html` 的响应：

```html
<link rel="modulepreload" crossorigin href="/assets/markdown-vendor-DdmbsTbr.js">
<link rel="modulepreload" crossorigin href="/assets/ui-vendor-BASzUKtj.js">
<link rel="modulepreload" crossorigin href="/assets/vendor-REG-SvF_.js">
<link rel="modulepreload" crossorigin href="/assets/editor-vendor-RzsodmY4.js">
<link rel="modulepreload" crossorigin href="/assets/react-vendor-Ds3fg1fm.js">
```

**问题**：`react-vendor` 是**最后**一个 preload，但 `ui-vendor` 是**第二**个！

这导致浏览器可能先加载 `ui-vendor`，但它依赖 React。

### 步骤 3: 使用浏览器控制台检查

在 Console 中运行：

```javascript
// 检查 React 是否已加载
console.log(typeof React);        // 应该是 'object'
console.log(typeof ReactDOM);     // 应该是 'object'

// 如果是 'undefined'，说明 React 没有加载或者加载失败
```

## 解决方案

### 方案 1: 修复 Vite 代码分割配置（推荐）

**问题根源**：`vite.config.ts` 中的 `manualChunks` 配置导致依赖顺序错误。

**修复文件**：`frontend/vite.config.ts`

找到 `manualChunks` 部分并修改为：

```typescript
manualChunks: (id) => {
  // Split vendor chunks for better caching
  if (id.includes('node_modules')) {
    // React ecosystem - 最高优先级，最先加载
    if (id.includes('react') || id.includes('react-dom') || id.includes('react-router')) {
      return 'react-vendor';
    }
    // UI libraries (依赖 React)
    if (id.includes('@mui') || id.includes('antd') || id.includes('@ant-design')) {
      return 'ui-vendor';
    }
    // Editor libraries
    if (id.includes('monaco') || id.includes('codemirror') || id.includes('@codemirror')) {
      return 'editor-vendor';
    }
    // Markdown and syntax highlighting
    if (id.includes('marked') || id.includes('highlight.js') || id.includes('prism') || 
        id.includes('react-markdown') || id.includes('remark') || id.includes('rehype')) {
      return 'markdown-vendor';
    }
    // Other node_modules
    return 'vendor';
  }
},
```

**关键改动**：
- 确保 `react-vendor` 检查在**最前面**
- 添加 `react-markdown` 等到 `markdown-vendor`

### 方案 2: 使用 Rollup 插件控制加载顺序

在 `vite.config.ts` 中添加：

```typescript
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  build: {
    rollupOptions: {
      output: {
        // 确保 React 在最前面
        manualChunks: (id) => {
          if (id.includes('node_modules')) {
            if (id.includes('react') || id.includes('react-dom')) {
              return 'react-vendor';
            }
            if (id.includes('@ant-design') || id.includes('antd')) {
              return 'ui-vendor';
            }
            // ... 其他配置
          }
        },
        // 强制 chunk 顺序
        chunkFileNames: (chunkInfo) => {
          if (chunkInfo.name === 'react-vendor') {
            return 'assets/01-react-vendor-[hash].js';
          }
          if (chunkInfo.name === 'ui-vendor') {
            return 'assets/02-ui-vendor-[hash].js';
          }
          return 'assets/[name]-[hash].js';
        },
      },
    },
  },
});
```

### 方案 3: 禁用代码分割（快速临时方案）

如果上述方案不行，暂时禁用 manualChunks：

```typescript
build: {
  rollupOptions: {
    output: {
      // 注释掉 manualChunks
      // manualChunks: (id) => { ... },
    },
  },
},
```

这会导致所有代码打包成一个大文件，但能避免加载顺序问题。

### 方案 4: 强制清除所有缓存

如果是缓存问题：

1. **浏览器端**：
   ```
   - Chrome: 设置 → 隐私与安全 → 清除浏览数据 → 全部时间 → 缓存的图片和文件
   - 或使用隐私模式
   ```

2. **服务器端**：
   ```bash
   # 停止容器
   docker compose down
   
   # 删除所有镜像和缓存
   docker rmi aidg-aidg aidg
   docker system prune -a -f
   
   # 重新构建
   docker compose build --no-cache
   docker compose up -d
   ```

3. **开发环境测试**：
   ```bash
   cd frontend
   rm -rf node_modules/.vite dist
   npm run build
   npm run preview
   ```
   
   访问 `http://localhost:4173` 测试

## 临时绕过方案

如果需要立即使用系统，可以使用 Vite 开发服务器：

```bash
cd frontend
npm run dev
```

访问 `http://localhost:5173` 使用开发版本（不会有这个问题）。

## 验证修复

修复后，检查：

1. **Network 标签顺序**：
   ```
   react-vendor-*.js → 200 OK (最先加载)
   ui-vendor-*.js → 200 OK
   vendor-*.js → 200 OK
   index-*.js → 200 OK
   ```

2. **Console 无错误**：
   - ✅ 无 `Cannot read properties of undefined`
   - ✅ 无 `Unexpected token '<'`
   - ✅ 应用正常渲染

3. **检查 React**：
   ```javascript
   console.log(React);  // 应该输出 React 对象
   ```

## 下一步

1. 修改 `frontend/vite.config.ts`
2. 重新构建前端：
   ```bash
   cd frontend
   rm -rf dist
   npm run build
   ```
3. 重新构建 Docker 镜像
4. 清除浏览器缓存并测试

---

**文档创建时间**: 2025-10-08  
**相关错误**: `Cannot read properties of undefined (reading 'createContext')`  
**相关文件**: `frontend/vite.config.ts`, `ui-vendor-BASzUKtj.js`
