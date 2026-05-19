# Skill: 版本号显示～自动同步

> 在 Web 应用中显示 GitHub tag 版本号和发布时间，随每次部署自动更新。

---

## 概述

本 skill 实现**构建时自动注入版本信息**，让前端应用能够显示：

| 信息 | 来源 | 示例 |
|------|------|------|
| **版本号** | Git tag (`v*`) 或 `package.json` | `v1.2.3` |
| **发布时间** | Git tag 的创建时间 | `2024-01-15 08:30:00` |
| **Commit SHA** | 当前 commit 短哈希 | `a1b2c3d` |

**工作原理**：

```
GitHub Actions 构建时
    ↓
提取 tag 名称 + 发布时间 + commit SHA
    ↓
写入环境变量 VITE_APP_VERSION / VITE_APP_BUILD_TIME
    ↓
前端代码通过 import.meta.env 读取
    ↓
页面显示版本信息
```

---

## 前置条件

- 项目使用 Vite / Vue CLI / React 等支持环境变量的构建工具
- 使用 Git tag 管理版本（如 `v1.0.0`）
- 已配置自动部署（可配合 `auto-deploy` skill 使用）

---

## 步骤一：复制 Workflow 模板

将本目录下的 [`templates/version.yml`](./templates/version.yml) 内容合并到你的 `.github/workflows/deploy.yml` 中。

**关键步骤说明**：

```yaml
# 1. 获取版本信息步骤（添加到 build job 的最前面）
- name: Get version info
  id: version
  run: |
    # 获取当前 tag 名称（如 v1.0.0）
    echo "tag=${GITHUB_REF#refs/tags/}" >> $GITHUB_OUTPUT
    
    # 获取 tag 的创建时间（ISO 8601 格式）
    TAG_DATE=$(git log -1 --format=%ai ${GITHUB_REF#refs/tags/} 2>/dev/null || date -u +"%Y-%m-%d %H:%M:%S")
    echo "build_time=$TAG_DATE" >> $GITHUB_OUTPUT
    
    # 获取 commit SHA（短格式）
    echo "sha=${GITHUB_SHA::7}" >> $GITHUB_OUTPUT

# 2. 构建时注入环境变量（在 Build 步骤中）
- name: Build the project
  run: npm run build
  env:
    VITE_APP_VERSION: ${{ steps.version.outputs.tag }}
    VITE_APP_BUILD_TIME: ${{ steps.version.outputs.build_time }}
    VITE_APP_COMMIT_SHA: ${{ steps.version.outputs.sha }}
```

---

## 步骤二：前端代码读取版本信息

### Vite 项目

```typescript
// src/version.ts
export const VERSION = import.meta.env.VITE_APP_VERSION || 'dev'
export const BUILD_TIME = import.meta.env.VITE_APP_BUILD_TIME || new Date().toISOString()
export const COMMIT_SHA = import.meta.env.VITE_APP_COMMIT_SHA || 'unknown'

// 格式化显示
export const versionDisplay = `${VERSION} (${COMMIT_SHA})`
export const buildTimeDisplay = new Date(BUILD_TIME).toLocaleString('zh-CN')
```

```vue
<!-- 在 Footer 或 About 页面中使用 -->
<template>
  <footer class="version-footer">
    <span>版本: {{ versionDisplay }}</span>
    <span>发布时间: {{ buildTimeDisplay }}</span>
  </footer>
</template>

<script setup lang="ts">
import { versionDisplay, buildTimeDisplay } from '@/version'
</script>
```

### Vue CLI 项目

```javascript
// .env.production
VUE_APP_VERSION=${TAG}
VUE_APP_BUILD_TIME=${BUILD_TIME}
```

```javascript
// src/version.js
export const VERSION = process.env.VUE_APP_VERSION || 'dev'
export const BUILD_TIME = process.env.VUE_APP_BUILD_TIME || new Date().toISOString()
```

### React (CRA) 项目

```javascript
// src/version.js
export const VERSION = process.env.REACT_APP_VERSION || 'dev'
export const BUILD_TIME = process.env.REACT_APP_BUILD_TIME || new Date().toISOString()
```

**注意**：CRA 需要以 `REACT_APP_` 开头，所以 workflow 中需要相应修改环境变量名。

---

## 步骤三：TypeScript 类型支持（可选）

```typescript
// src/env.d.ts
/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_APP_VERSION: string
  readonly VITE_APP_BUILD_TIME: string
  readonly VITE_APP_COMMIT_SHA: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
```

---

## 完整 Workflow 示例

配合 `auto-deploy` skill 使用的完整 workflow：

```yaml
name: "Auto Deploy with Version"

on:
  workflow_dispatch:
  push:
    tags:
      - 'v*'

permissions:
  contents: read
  pages: write
  id-token: write

concurrency:
  group: "pages"
  cancel-in-progress: true

jobs:
  build-and-deploy:
    runs-on: ubuntu-latest

    steps:
      - name: Checkout code
        uses: actions/checkout@v4
        with:
          fetch-depth: 0  # 需要完整历史来获取 tag 信息

      # ========== 获取版本信息 ==========
      - name: Get version info
        id: version
        run: |
          TAG=${GITHUB_REF#refs/tags/}
          echo "tag=$TAG" >> $GITHUB_OUTPUT
          
          # 获取 tag 的创建时间
          TAG_DATE=$(git log -1 --format=%ci $TAG 2>/dev/null || date -u +"%Y-%m-%d %H:%M:%S")
          echo "build_time=$TAG_DATE" >> $GITHUB_OUTPUT
          
          # 获取 commit SHA
          echo "sha=${GITHUB_SHA::7}" >> $GITHUB_OUTPUT
          
          echo "📦 Version: $TAG"
          echo "🕐 Build Time: $TAG_DATE"
          echo "🔨 Commit: ${GITHUB_SHA::7}"

      - name: Setup Node.js
        uses: actions/setup-node@v4
        with:
          node-version: '20'

      - name: Install dependencies
        run: npm ci

      # ========== 构建（注入版本信息） ==========
      - name: Build the project
        run: npm run build
        env:
          VITE_APP_VERSION: ${{ steps.version.outputs.tag }}
          VITE_APP_BUILD_TIME: ${{ steps.version.outputs.build_time }}
          VITE_APP_COMMIT_SHA: ${{ steps.version.outputs.sha }}
          VITE_BASE: /${{ github.event.repository.name }}/

      # ========== 部署到 WebDAV ==========
      - name: Upload to WebDAV
        uses: weijia/action-upload-webdav@master
        with:
          webdav-url: ${{ secrets.WEBDAV_URL }}
          webdav-username: ${{ secrets.WEBDAV_USERNAME }}
          webdav-password: ${{ secrets.WEBDAV_PASSWORD }}
          webdav-root: online/${{ github.event.repository.name }}
          source-directory: ./dist

      # ========== 部署到 GitHub Pages ==========
      - name: Setup Pages
        uses: actions/configure-pages@v5

      - name: Upload artifact
        uses: actions/upload-pages-artifact@v3
        with:
          path: ./dist

  deploy-pages:
    needs: build-and-deploy
    runs-on: ubuntu-latest
    environment:
      name: github-pages
      url: ${{ steps.deployment.outputs.page_url }}
    steps:
      - name: Deploy to GitHub Pages
        id: deployment
        uses: actions/deploy-pages@v4
```

---

## 显示效果示例

```
┌─────────────────────────────────────────┐
│  My Web App                             │
│                                         │
│  [页面内容...]                          │
│                                         │
├─────────────────────────────────────────┤
│  版本: v1.2.3 (a1b2c3d)    发布时间: 2024-01-15 08:30:00  │
└─────────────────────────────────────────┘
```

---

## 不同框架的环境变量对照

| 框架 | 环境变量前缀 | 示例 |
|------|-------------|------|
| **Vite** | `VITE_` | `VITE_APP_VERSION` |
| **Vue CLI** | `VUE_APP_` | `VUE_APP_VERSION` |
| **React (CRA)** | `REACT_APP_` | `REACT_APP_VERSION` |
| **Next.js** | `NEXT_PUBLIC_` | `NEXT_PUBLIC_APP_VERSION` |

使用本 skill 时，请根据你的框架修改 workflow 中的环境变量名。

---

## 新项目快速配置清单

1. **复制 workflow** → 将 `templates/version.yml` 合并到 `.github/workflows/deploy.yml`
2. **创建 version.ts** → 复制上面的代码到 `src/version.ts`
3. **修改组件** → 在 Footer 或 About 页面引入版本信息
4. **推送 tag** → `git tag v1.0.0 && git push origin v1.0.0`
5. **验证** → 检查页面底部是否正确显示版本号

---

## 注意事项

- `fetch-depth: 0` 是必需的，否则无法获取 tag 的创建时间
- 如果使用 `package.json` 版本而非 git tag，可将 `steps.version.outputs.tag` 改为 `$(node -p "require('./package.json').version")`
- 时间格式可根据需要调整，`git log -1 --format=%ci` 返回的是 ISO 8601 格式
