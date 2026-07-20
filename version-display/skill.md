# Skill: 版本号显示～自动同步

> 在 Web 应用中显示 Git tag 版本号和发布时间，随每次部署自动更新。支持 **CI 构建注入** 和 **本地构建自动生成** 两种方案。

---

## 概述

本 skill 实现**构建时自动注入版本信息**，让前端应用能够显示：

| 信息 | 来源 | 示例 |
|------|------|------|
| **版本号** | Git tag (`v*`) 或 `branch-sha` | `v1.2.3` 或 `main-a1b2c3d` |
| **发布时间** | 构建时的北京时间 | `2024-01-15 08:30:00` |
| **Commit SHA** | 当前 commit 短哈希 | `a1b2c3d` |

**两种方案对比**：

| 方案 | 适用场景 | 版本来源 | 构建时间来源 |
|------|---------|---------|-------------|
| **方案一：CI 环境变量注入** | GitHub Actions 自动部署 | Git tag | Tag 创建时间 |
| **方案二：本地构建时生成** | 本地 `npm run build` 后手动部署 | Git branch/tag + sha | 构建时的北京时间 |

---

## 前置条件

- 项目使用 Vite / Vue CLI / React 等支持环境变量的构建工具
- 使用 Git 管理版本
- 已配置自动部署（可配合 `auto-deploy` skill 使用，方案一）

---

## 方案一：CI 构建时注入（GitHub Actions）

适用于通过 GitHub Actions 自动部署的场景。

### 步骤 1：复制 Workflow 模板

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

### 步骤 2：前端代码读取版本信息

```typescript
// src/version.ts
export const VERSION = import.meta.env.VITE_APP_VERSION || 'dev'
export const BUILD_TIME = import.meta.env.VITE_APP_BUILD_TIME || new Date().toISOString()
export const COMMIT_SHA = import.meta.env.VITE_APP_COMMIT_SHA || 'unknown'

// 格式化显示
export const versionDisplay = `${VERSION} (${COMMIT_SHA})`
export const buildTimeDisplay = new Date(BUILD_TIME).toLocaleString('zh-CN')
```

---

## 方案二：本地构建时自动生成（推荐）

适用于本地 `npm run build` 后手动部署到任意服务器（如 WebDAV、SFTP、静态托管等）。**不依赖 CI 环境变量**。

### 工作原理

```
npm run build
    ↓
scripts/generate-version.js 自动执行
    ↓
读取 git branch / tag / sha，生成北京时间构建时间
    ↓
写入 src/version.json
    ↓
前端代码 import version.json（不是 import.meta.env）
    ↓
页面显示版本信息
```

### 步骤 1：复制生成脚本

将 [`templates/generate-version.js`](./templates/generate-version.js) 复制到项目的 `scripts/` 目录。

### 步骤 2：修改 package.json

在 `build` 脚本前添加生成步骤：

```json
{
  "scripts": {
    "build": "node scripts/generate-version.js && tsc -b && vite build"
  }
}
```

### 步骤 3：创建 version.ts

```typescript
// src/version.ts
import versionInfo from './version.json'

export const VERSION = versionInfo.version
export const BUILD_TIME = versionInfo.buildTime
export const COMMIT_SHA = versionInfo.sha

export const versionDisplay = `${VERSION} (${COMMIT_SHA})`
export const buildTimeDisplay = BUILD_TIME
```

### 步骤 4：TypeScript 配置

确保 `tsconfig.json` 或 `tsconfig.app.json` 中启用了 JSON 导入：

```json
{
  "compilerOptions": {
    "resolveJsonModule": true
  }
}
```

### 生成文件示例

执行 `npm run build` 后，`src/version.json` 内容如下：

```json
{
  "version": "main-a1b2c3d",
  "buildTime": "2024/01/15 08:30:00",
  "sha": "a1b2c3d",
  "branch": "main",
  "tag": ""
}
```

如果有 git tag，则 `version` 为 tag 名称（如 `v1.2.3`），`tag` 字段也有值。

---

## 完整 Workflow 示例（方案一）

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
          fetch-depth: 0

      - name: Get version info
        id: version
        run: |
          TAG=${GITHUB_REF#refs/tags/}
          echo "tag=$TAG" >> $GITHUB_OUTPUT
          TAG_DATE=$(git log -1 --format=%ci $TAG 2>/dev/null || date -u +"%Y-%m-%d %H:%M:%S")
          echo "build_time=$TAG_DATE" >> $GITHUB_OUTPUT
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

      - name: Build the project
        run: npm run build
        env:
          VITE_APP_VERSION: ${{ steps.version.outputs.tag }}
          VITE_APP_BUILD_TIME: ${{ steps.version.outputs.build_time }}
          VITE_APP_COMMIT_SHA: ${{ steps.version.outputs.sha }}

      - name: Upload to WebDAV
        uses: weijia/action-upload-webdav@master
        with:
          webdav-url: ${{ secrets.WEBDAV_URL }}
          webdav-username: ${{ secrets.WEBDAV_USERNAME }}
          webdav-password: ${{ secrets.WEBDAV_PASSWORD }}
          webdav-root: online/${{ github.event.repository.name }}
          source-directory: ./dist

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

## 不同框架的环境变量对照（方案一）

| 框架 | 环境变量前缀 | 示例 |
|------|-------------|------|
| **Vite** | `VITE_` | `VITE_APP_VERSION` |
| **Vue CLI** | `VUE_APP_` | `VUE_APP_VERSION` |
| **React (CRA)** | `REACT_APP_` | `REACT_APP_VERSION` |
| **Next.js** | `NEXT_PUBLIC_` | `NEXT_PUBLIC_APP_VERSION` |

---

## 新项目快速配置清单

### 方案一（CI 注入）

1. **复制 workflow** → 将 `templates/version.yml` 合并到 `.github/workflows/deploy.yml`
2. **创建 version.ts** → 使用 `import.meta.env.VITE_APP_*` 读取
3. **修改组件** → 在 Footer 或 About 页面引入版本信息
4. **推送 tag** → `git tag v1.0.0 && git push origin v1.0.0`
5. **验证** → 检查页面底部是否正确显示版本号

### 方案二（本地生成）

1. **复制脚本** → 将 `templates/generate-version.js` 复制到 `scripts/`
2. **修改 package.json** → `build` 脚本前加 `node scripts/generate-version.js &&`
3. **创建 version.ts** → 使用 `import versionInfo from './version.json'`
4. **TS 配置** → 确保 `resolveJsonModule: true`
5. **构建** → `npm run build`，检查生成的 `version.json`
6. **部署** → 将 `dist/` 上传到任意服务器

---

## 注意事项

- **方案一** 中 `fetch-depth: 0` 是必需的，否则无法获取 tag 的创建时间
- **方案二** 中 `version.json` 会被 git 忽略（建议添加到 `.gitignore`），因为它每次构建都会变化
- 如果使用 `package.json` 版本而非 git tag，可将版本获取改为 `$(node -p "require('./package.json').version")`
- 时间格式可根据需要调整，`git log -1 --format=%ci` 返回的是 ISO 8601 格式
