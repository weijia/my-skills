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
- 已安装 `gh` CLI 并登录（或用 GitHub Web 界面设置 Secrets）

---

## 步骤一：复制 Workflow 模板

将本目录下的 `templates/build-upload.yml` 复制到目标项目的 `.github/workflows/`。

**根据项目类型修改构建步骤**：

### Flutter Android 示例

```yaml
      - name: Setup Flutter
        uses: subosito/flutter-action@v2
        with:
          flutter-version: '3.x'

      - name: Build APK
        run: flutter build apk --release

      - name: Upload APK to SFTP
        uses: weijia/action-upload-sftp@master
        with:
          sftp-host: ${{ secrets.SFTP_HOST }}
          sftp-port: ${{ secrets.SFTP_PORT }}
          sftp-username: ${{ secrets.SFTP_USERNAME }}
          sftp-password: ${{ secrets.SFTP_PASSWORD }}
          source-path: build/app/outputs/flutter-apk/app-release.apk
          remote-path: /home/github/apps/【项目名】/app-release.apk
          cleanup-old-files: 'true'
          max-keep-files: 5
```

### 通用 Node.js 项目示例

```yaml
      - name: Setup Node.js
        uses: actions/setup-node@v4
        with:
          node-version: '20'

      - name: Build project
        run: npm ci && npm run build

      - name: Upload to SFTP
        uses: weijia/action-upload-sftp@master
        with:
          sftp-host: ${{ secrets.SFTP_HOST }}
          sftp-port: ${{ secrets.SFTP_PORT }}
          sftp-username: ${{ secrets.SFTP_USERNAME }}
          sftp-private-key: ${{ secrets.SFTP_PRIVATE_KEY }}
          source-path: ./dist/
          remote-path: /var/www/apps/【项目名】/
          cleanup-old-files: 'true'
          max-keep-files: 5
```

---

## 步骤二：配置 GitHub Secrets

### 方式一：使用 GitHub CLI（推荐）

#### Windows 命令（CMD）

```cmd
set PROJ=【项目名】
set USERNAME=【GitHub用户名】
set ROOT=%USERNAME%/%PROJ%

set SFTP_HOST=【SFTP 服务器地址】
set SFTP_PORT=22
set SFTP_USERNAME=【SFTP 用户名】

:: 密码认证（二选一）
set SFTP_PASSWORD=【SFTP 密码】
gh secret set SFTP_PASSWORD --repo %ROOT% --body %SFTP_PASSWORD%

:: 或 SSH 私钥认证（二选一，推荐）
set SFTP_PRIVATE_KEY=【SSH 私钥完整内容】
gh secret set SFTP_PRIVATE_KEY --repo %ROOT% --body "%SFTP_PRIVATE_KEY%"

:: 通用 secrets
gh secret set SFTP_HOST --repo %ROOT% --body %SFTP_HOST%
gh secret set SFTP_PORT --repo %ROOT% --body %SFTP_PORT%
gh secret set SFTP_USERNAME --repo %ROOT% --body %SFTP_USERNAME%
```

#### Linux / macOS

```bash
PROJ="【项目名】"
USERNAME="【GitHub用户名】"
ROOT="${USERNAME}/${PROJ}"

SFTP_HOST="【SFTP 服务器地址】"
SFTP_PORT="22"
SFTP_USERNAME="【SFTP 用户名】"

# 密码认证（二选一）
SFTP_PASSWORD="【SFTP 密码】"
gh secret set SFTP_PASSWORD --repo "$ROOT" --body "$SFTP_PASSWORD"

# 或 SSH 私钥认证（二选一，推荐）
# SFTP_PRIVATE_KEY="$(cat ~/.ssh/id_rsa)"
# gh secret set SFTP_PRIVATE_KEY --repo "$ROOT" --body "$SFTP_PRIVATE_KEY"

gh secret set SFTP_HOST --repo "$ROOT" --body "$SFTP_HOST"
gh secret set SFTP_PORT --repo "$ROOT" --body "$SFTP_PORT"
gh secret set SFTP_USERNAME --repo "$ROOT" --body "$SFTP_USERNAME"
```

### 方式二：使用 GitHub Web 界面

1. 打开仓库页面 → **Settings** → **Secrets and variables** → **Actions**
2. 点击 **New repository secret**
3. 逐个添加以下 secrets

### Secrets 清单

| Secret 名称 | 必填 | 值 | 说明 |
|-------------|------|-----|------|
| `SFTP_HOST` | ✅ | 服务器 IP 或域名 | SFTP 服务器地址 |
| `SFTP_PORT` | ✅ | `22` | SFTP 端口（默认 22） |
| `SFTP_USERNAME` | ✅ | 用户名 | SFTP 认证用户名 |
| `SFTP_PASSWORD` | ⚪ | 密码 | 密码认证（与私钥二选一） |
| `SFTP_PRIVATE_KEY` | ⚪ | SSH 私钥完整内容 | 私钥认证（与密码二选一，推荐） |

> ⚠️ **密码和私钥二选一**，不能同时为空

---

## 步骤三：触发部署

```bash
git push origin main
```

---

## Action 参数详解

| 参数 | 必填 | 默认值 | 说明 |
|------|------|--------|------|
| `sftp-host` | ✅ | - | SFTP 服务器地址 |
| `sftp-port` | ⚪ | `22` | SFTP 端口 |
| `sftp-username` | ✅ | - | 用户名 |
| `sftp-password` | ⚪ | - | 密码（与私钥二选一） |
| `sftp-private-key` | ⚪ | - | SSH 私钥内容（与密码二选一） |
| `source-path` | ✅ | - | 本地文件或目录路径 |
| `remote-path` | ✅ | - | 远程目标路径 |
| `cleanup-old-files` | ⚪ | `true` | 是否清理旧文件 |
| `max-keep-files` | ⚪ | `5` | 保留最新 N 个文件 |
| `debug` | ⚪ | `false` | 调试模式 |

---

## 上传结果示例

```
/home/github/apps/my-app/
├── app-release-v1.0.0.apk
├── app-release-v0.9.0.apk
├── app-release-v0.8.0.apk
├── app-release-v0.7.0.apk
└── app-release-v0.6.0.apk
```

超过 `max-keep-files` 数量时，自动删除最旧的文件。

---

## 常见问题

### Q: 上传失败，提示 "File not found"

**原因**：`source-path` 指定的文件不存在，或构建步骤未正确执行。

**解决**：
1. 检查构建是否成功
2. 确认 `source-path` 路径正确（可用 `ls -la` 在 workflow 中调试）

### Q: 认证失败

**原因**：密码或私钥错误。

**解决**：
1. 确认 secrets 已正确设置
2. 测试手动连接：`sftp -P 22 user@host`
3. 私钥认证时，确保私钥格式正确（包含 `-----BEGIN...` 头尾）

### Q: 清理旧文件不生效

**原因**：`cleanup-old-files` 仅在 `source-path` 为目录时生效。

**解决**：如需按版本保留，使用带版本号的文件名：
```yaml
remote-path: /home/github/apps/my-app/app-release-${{ github.run_number }}.apk
```

---

## 注意事项

- **推荐使用 SSH 私钥认证**，比密码更安全
- `source-path` 支持单个文件或整个目录
- `cleanup-old-files` 仅在 `source-path` 为目录时生效
- 确保 SFTP 用户对目标目录有写入权限
- Action 引用必须使用 `@master` 或完整 commit SHA，不能用 `@v1.0.0`（release 未打包）
