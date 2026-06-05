# Skill: SFTP 部署～APP 上传

> 通过 SFTP 将构建产物（APK、APP 安装包等）上传到远程服务器。

---

## 概述

本 skill 用于给项目配置 **SFTP 自动上传**，适用于：

| 场景 | 说明 |
|------|------|
| **Android APK** | 构建后自动上传 APK 安装包 |
| **iOS IPA** | 构建后自动上传 IPA 安装包 |
| **桌面应用** | 上传 exe/dmg/AppImage 等安装文件 |
| **二进制文件** | 上传任意构建产物 |

**核心功能**：
- 支持密码和 SSH 私钥两种认证方式
- 自动清理旧文件，保留最新 N 个版本
- 单文件或整个目录上传

---

## 前置条件

- 拥有 SFTP 服务器（SSH 支持文件传输即可）
- 已安装 `gh` CLI 并登录

---

## 步骤一：复制 Workflow 模板

将本目录下的 [`templates/build-upload.yml`](./templates/build-upload.yml) 复制到目标项目的 `.github/workflows/`。

---

## 步骤二：配置 GitHub Secrets

在仓库中设置 SFTP 所需的 secret。

### Windows 命令（CMD）

```cmd
set PROJ=【项目名】
set USERNAME=weijia
set ROOT=%USERNAME%/%PROJ%

set SFTP_HOST=【SFTP 服务器地址】
set SFTP_PORT=22
set SFTP_USERNAME=【SFTP 用户名】
set SFTP_PRIVATE_KEY=【SSH 私钥内容】

gh secret set SFTP_HOST --repo %ROOT% --body %SFTP_HOST%
gh secret set SFTP_PORT --repo %ROOT% --body %SFTP_PORT%
gh secret set SFTP_USERNAME --repo %ROOT% --body %SFTP_USERNAME%
gh secret set SFTP_PRIVATE_KEY --repo %ROOT% --body "%SFTP_PRIVATE_KEY%"
```

### Linux / macOS

```bash
PROJ="【项目名】"
USERNAME="weijia"
ROOT="${USERNAME}/${PROJ}"

gh secret set SFTP_HOST --repo "$ROOT" --body "【SFTP 服务器地址】"
gh secret set SFTP_PORT --repo "$ROOT" --body "22"
gh secret set SFTP_USERNAME --repo "$ROOT" --body "【SFTP 用户名】"
gh secret set SFTP_PRIVATE_KEY --repo "$ROOT" --body "【SSH 私钥内容】"
```

### Secrets 清单

| Secret 名称 | 值 | 说明 |
|-------------|-----|------|
| `SFTP_HOST` | 服务器 IP 或域名 | SFTP 服务器地址 |
| `SFTP_PORT` | `22` | SFTP 端口（默认 22） |
| `SFTP_USERNAME` | 用户名 | SFTP 认证用户名 |
| `SFTP_PRIVATE_KEY` | SSH 私钥完整内容 | 推荐：使用 SSH 私钥认证 |
| `SFTP_PASSWORD` | 密码 | 备选：使用密码认证（与私钥二选一） |

---

## 步骤三：触发部署

```bash
# 推送代码即可触发构建和上传
git push origin main
```

---

## 上传结果示例

```
/var/www/apps/
├── my-app-v1.0.apk      ← 最新版本
├── my-app-v0.9.apk
├── my-app-v0.8.apk
├── my-app-v0.7.apk
└── my-app-v0.6.apk
```

超过 `max-keep-files` 数量时，自动删除最旧的文件。

---

## 新项目快速配置清单

1. **复制 workflow** → 将 `templates/build-upload.yml` 放入 `.github/workflows/`
2. **设置 secrets** → 运行步骤二的命令
3. **推送代码** → `git push origin main`

---

## 注意事项

- 推荐使用 SSH 私钥认证，比密码更安全
- `source-path` 支持单个文件或整个目录
- `cleanup-old-files` 仅在 `source-path` 为目录时生效
- 确保 SFTP 用户对目标目录有写入权限
