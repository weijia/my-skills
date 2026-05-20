# Skill: 自动部署～无密码

> 将网页应用通过 WebDAV + GitHub Pages 自动发布，git push tag 即触发部署。

---

## 概述

本 skill 用于给前端项目配置**三通道自动部署**：

| 通道 | 目标 | 说明 |
|------|------|------|
| **WebDAV (版本目录)** | `online/{项目名}` | 按 tag 版本部署，保留历史版本 |
| **WebDAV (latest)** | `online/latest` | 始终指向最新版本，每次先清空再部署 |
| **GitHub Pages** | `gh-pages` 分支 | 部署到 GitHub Pages |

**触发条件**：推送 `v*` 格式的 tag（如 `v1.0.0`），或手动触发。

**关键约束**：项目部署在服务器子目录（非根目录），构建时必须设置正确的 `base path`，否则静态资源引用会 404。

---

## 前置条件

- 项目为前端静态站点（Vite / React / Vue 等）
- 已安装 `gh` CLI 并登录（`gh auth login`）
- 拥有 WebDAV 服务器账号

---

## 步骤一：复制 Workflow 模板

将本目录下的 [`templates/deploy.yml`](./templates/deploy.yml) 复制到目标项目的 `.github/workflows/deploy.yml`。

### 关于 base path 的说明

由于页面部署在子目录 `online/{项目名}/` 下（而非服务器根目录），构建产物中所有静态资源引用（JS、CSS、图片等）必须带上子目录前缀，否则浏览器会从根目录查找资源导致 404。

**不同框架的配置方式**：

| 框架 | 构建命令中的环境变量 | 或配置文件 |
|------|---------------------|-----------|
| **Vite** | `VITE_BASE=/项目名/ npm run build` | `vite.config.ts` → `base: '/项目名/'` |
| **Vue CLI** | `VUE_APP_BASE=/项目名/ npm run build` | `vue.config.js` → `publicPath: '/项目名/'` |
| **React (CRA)** | `PUBLIC_URL=/项目名/ npm run build` | `homepage: '/项目名/'` in package.json |
| **Nuxt 3** | `NUXT_APP_BASE_PATH=/项目名/ npm run build` | `nuxt.config.ts` → `app.baseURL: '/项目名/'` |
| **Next.js** | `NEXT_PUBLIC_BASE_PATH=/项目名/ npm run build` | `next.config.js` → `basePath: '/项目名'` |

---

## 步骤二：配置 GitHub Secrets

在仓库中设置 WebDAV 所需的 3 个 secret。

### Windows 命令（CMD）

```cmd
set PROJ=【项目名】
set USERNAME=weijia
set ROOT=%USERNAME%/%PROJ%
set WEBDAV_PASSWORD=【你的密码】
set WEBDAV_USERNAME=【你的用户名】
set WEBDAV_URL=https://miya.teracloud.jp/dav/

gh secret set WEBDAV_URL --repo %ROOT% --body %WEBDAV_URL%
gh secret set WEBDAV_USERNAME --repo %ROOT% --body %WEBDAV_USERNAME%
gh secret set WEBDAV_PASSWORD --repo %ROOT% --body %WEBDAV_PASSWORD%
```

### PowerShell

```powershell
$PROJ = "【项目名】"
$USERNAME = "weijia"
$ROOT = "$USERNAME/$PROJ"
$WEBDAV_PASSWORD = "【你的密码】"
$WEBDAV_USERNAME = "【你的用户名】"
$WEBDAV_URL = "https://miya.teracloud.jp/dav/"

gh secret set WEBDAV_URL --repo $ROOT --body $WEBDAV_URL
gh secret set WEBDAV_USERNAME --repo $ROOT --body $WEBDAV_USERNAME
gh secret set WEBDAV_PASSWORD --repo $ROOT --body $WEBDAV_PASSWORD
```

### Linux / macOS

```bash
PROJ="【项目名】"
USERNAME="weijia"
ROOT="${USERNAME}/${PROJ}"
WEBDAV_PASSWORD="【你的密码】"
WEBDAV_USERNAME="【你的用户名】"
WEBDAV_URL="https://miya.teracloud.jp/dav/"

gh secret set WEBDAV_URL --repo "$ROOT" --body "$WEBDAV_URL"
gh secret set WEBDAV_USERNAME --repo "$ROOT" --body "$WEBDAV_USERNAME"
gh secret set WEBDAV_PASSWORD --repo "$ROOT" --body "$WEBDAV_PASSWORD"
```

### Secrets 清单

| Secret 名称 | 值 | 说明 |
|-------------|-----|------|
| `WEBDAV_URL` | `https://miya.teracloud.jp/dav/` | WebDAV 服务器地址 |
| `WEBDAV_USERNAME` | 你的 WebDAV 用户名 | WebDAV 认证用户名 |
| `WEBDAV_PASSWORD` | 你的 WebDAV 密码 | WebDAV 认证密码 |

---

## 步骤三：启用 GitHub Pages

1. 进入仓库 → **Settings** → **Pages**
2. **Source** 选择 **GitHub Actions**
3. 保存

---

## 步骤四：触发部署

```bash
# 创建并推送 tag 即可触发自动部署
git tag v1.0.0
git push origin v1.0.0
```

或在 GitHub 仓库的 **Actions** 页面手动点击 **Run workflow**。

---

## 部署结果

部署成功后，站点可通过以下地址访问：

| 通道 | 地址 | 用途 |
|------|------|------|
| **WebDAV (版本)** | `https://miya.teracloud.jp/dav/online/{项目名}/` | 特定版本访问 |
| **WebDAV (latest)** | `https://miya.teracloud.jp/dav/online/latest/` | 始终访问最新版本 |
| **GitHub Pages** | `https://{用户名}.github.io/{项目名}/` | GitHub 托管 |

### WebDAV 目录结构

```
online/
├── my-app/           ← 版本目录（webdav-root: online/my-app）
│   ├── index.html
│   └── assets/
└── latest/           ← 最新版本（与项目目录同级）
    ├── index.html
    └── assets/
```

### latest 目录说明

通过 `weijia/action-upload-webdav` 默认开启的 `copy-to-latest` 功能实现。Action 会自动：
1. 先清空 `online/latest/` 目录（如果存在）
2. 将最新构建的文件上传到 `latest/` 目录

这样 `latest/` 始终指向最新发布的版本，方便用户访问最新版而无需知道具体 tag。

### 版本自动清理

Action 会在每次上传前自动检查版本目录数量，**超过 5 个版本时自动删除最旧的版本**，始终只保留最新的 5 个版本。

- 排序依据：目录的创建时间（`creationdate`），无时间信息的排在最前（优先删除）
- `latest` 目录不会被纳入清理范围
- 清理失败不会阻止上传流程

---

## 新项目快速配置清单

给新项目 `x` 配置自动部署的完整流程：

1. **复制 workflow** → 将 `templates/deploy.yml` 放入 `.github/workflows/`
2. **配置 base path** → 在构建配置中设置 `base: '/x/'`
3. **设置 secrets** → 运行步骤二的命令（替换 `PROJ=x`）
4. **启用 GitHub Pages** → Settings → Pages → Source: GitHub Actions
5. **推送 tag** → `git tag v1.0.0 && git push origin v1.0.0`

---

## 注意事项

- WebDAV action 使用的是 `weijia/action-upload-webdav@master`，会将 `source-directory` 下的所有文件上传到 `webdav-root` 指定的路径
- `copy-to-latest` 默认开启，会自动将文件复制到与项目目录同级的 `latest/` 目录
- 版本自动清理默认保留最新 10 个版本，超出部分按创建时间从旧到新删除
- 如果项目使用 yarn 而非 npm，将 `npm ci` 改为 `yarn install --frozen-lockfile`，`npm run build` 改为 `yarn build`
- WebDAV 密码通过 GitHub Secrets 存储，不会暴露在代码或日志中
- 首次部署前请确保 WebDAV 服务器上 `online/` 目录已存在，否则可能需要手动创建
