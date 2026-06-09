# Skill: Flutter 多平台构建 + 版本号 + SFTP 上传

> Flutter 项目一键配置 GitHub Actions：自动生成版本号、多平台构建、SFTP 上传安装包。

---

## 概述

本 skill 将以下三件事整合到一个完整的 GitHub Actions 工作流中：

| 功能 | 说明 |
|------|------|
| **版本号自动生成** | 有 git tag 用 tag + 时间戳，无 tag 用时间戳，每次构建唯一 |
| **多平台构建** | Android / iOS / macOS / Linux / Windows，可按需选择 |
| **SFTP 上传** | 构建产物自动上传到 SFTP 服务器，保留最近 N 个版本 |

**版本号规则**：

| 场景 | 版本号格式 | 示例 |
|------|-----------|------|
| 正好在 tag 上 | `x.y.z` | `1.1.0` |
| tag 后有新提交 | `x.y.z-YYYYMMDD.HHMMSSCST` | `1.1.0-20260609.143000CST` |
| 无 tag | `0.0.0-YYYYMMDD.HHMMSSCST` | `0.0.0-20260609.143000CST` |

**注意事项**：
- 版本号中**不含 `+` 号**（Android 不允许 `+`，会导致安装时报 "package info is null"）
- 用 `CST` 表示东八区，语义清晰且 Android 兼容
- 每次构建时间不同，SFTP 文件名唯一，不会覆盖旧版本

---

## 前置条件

- Flutter 项目（任意版本）
- GitHub 仓库
- SFTP 服务器（可选，不上传可跳过）

---

## 步骤一：复制文件到项目

将 `templates/` 中的文件复制到目标项目：

```
templates/generate_version.sh       → scripts/generate_version.sh
templates/build-all-platforms.yml   → .github/workflows/build-all-platforms.yml
templates/version.dart              → lib/version.dart（可选）
```

**替换模板中的占位符**：

| 占位符 | 替换为 |
|--------|--------|
| `【项目名】` | 你的项目名（如 `my-app`） |
| `3.35.6` | 你需要的 Flutter 版本 |

---

## 步骤二：启用 SFTP 上传（可选）

在 `build-all-platforms.yml` 中找到 Android job 的 SFTP 步骤，取消注释：

```yaml
      - name: Upload APK to SFTP
        uses: weijia/action-upload-sftp@master
        with:
          sftp-host: ${{ secrets.SFTP_HOST }}
          sftp-port: ${{ secrets.SFTP_PORT }}
          sftp-username: ${{ secrets.SFTP_USERNAME }}
          sftp-password: ${{ secrets.SFTP_PASSWORD }}
          source-path: build/app/outputs/flutter-apk/app-release.apk
          remote-path: /home/github/apps/【项目名】/app-release-${{ needs.generate-version.outputs.version_name }}.apk
          cleanup-old-files: 'true'
          max-keep-files: '5'
```

将 `remote-path` 中的 `【项目名】` 替换为实际项目名。

---

## 步骤三：配置 GitHub Secrets

### 必需（SFTP 上传时）

| Secret | 说明 |
|--------|------|
| `SFTP_HOST` | SFTP 服务器地址 |
| `SFTP_PORT` | SFTP 端口（默认 22） |
| `SFTP_USERNAME` | SFTP 用户名 |
| `SFTP_PASSWORD` | SFTP 密码（与私钥二选一） |

### 可选（Android 正式签名）

| Secret | 说明 |
|--------|------|
| `KEYSTORE_BASE64` | Base64 编码的 keystore 文件 |
| `KEYSTORE_PASSWORD` | Keystore 密码 |
| `KEY_ALIAS` | Key 别名 |
| `KEY_PASSWORD` | Key 密码 |

### 配置命令

```bash
PROJ="你的项目名"
USERNAME="你的GitHub用户名"
ROOT="${USERNAME}/${PROJ}"

gh secret set SFTP_HOST --repo "$ROOT" --body "你的SFTP地址"
gh secret set SFTP_PORT --repo "$ROOT" --body "22"
gh secret set SFTP_USERNAME --repo "$ROOT" --body "用户名"
gh secret set SFTP_PASSWORD --repo "$ROOT" --body "密码"
```

---

## 步骤四：在 Dart 代码中读取版本号（可选）

将 `templates/version.dart` 复制到 `lib/version.dart`，然后在代码中使用：

```dart
import 'version.dart';

// 在 About 页面或设置页面显示
Text('版本: ${AppVersion.display}')
```

---

## 触发方式

### 自动触发

每次 push 代码到任意分支自动构建：

```bash
git push origin main
```

### 手动触发（可选指定平台）

在 GitHub Actions 页面手动触发，可选择要构建的平台：

```
platforms: android,windows    # 只构建 Android 和 Windows
flutter_version: 3.24.0        # 指定 Flutter 版本
```

### 打 tag

```bash
git tag v1.2.0
git push origin v1.2.0
```

打 tag 后的第一次构建版本号为 `1.2.0`，后续提交版本号为 `1.2.0-20260610.090000CST`。

---

## SFTP 上传结果示例

```
/home/github/apps/my-app/
├── app-release-1.1.0-20260609.143000CST.apk    ← 最新
├── app-release-1.1.0-20260608.120000CST.apk
├── app-release-1.1.0-20260607.100000CST.apk
├── app-release-1.1.0-20260606.080000CST.apk
└── app-release-1.1.0-20260605.060000CST.apk    ← 最旧（下次构建时自动删除）
```

超过 `max-keep-files` 数量时，自动删除最旧的文件。

---

## 版本号生成脚本详解

`generate_version.sh` 的工作原理：

```
git describe --tags → 获取最近的 v* tag
        ↓
    有 tag？
   /      \
  是       否
  ↓        ↓
tag 上有   0.0.0-时间戳CST
新提交？   （用时间戳做 build number）
 / \
是   否
↓    ↓
tag-时间戳CST   tag
```

**关键设计决策**：
- 不用 commit 数量做后缀（无语义，用户不关心）
- 用构建时间戳做后缀（用户能看出构建时间）
- 不用 `+` 号（Android 安装器不兼容）
- 用 `CST` 表示时区（简洁、无特殊字符）

---

## 常见问题

### Q: 安装 APK 报 "package info is null"

**原因**：版本号包含 `+` 号（如 `1.0.0+5`），Android 无法解析。

**解决**：确保 `generate_version.sh` 中版本号不含 `+`。用 `-` 替代。

### Q: SFTP 文件名一直不变，旧版本被覆盖

**原因**：版本号固定（如一直用 `1.0.0`），没有时间戳后缀。

**解决**：确保 tag 后有新提交时会追加时间戳，或删除 tag 让版本号自动生成。

### Q: 只想构建 Android，不想构建其他平台

**解决**：手动触发时指定 `platforms: android`，或修改 workflow 的 `default` 值。

### Q: 如何使用正式签名（非 debug）

**解决**：配置 `KEYSTORE_BASE64` 等 4 个 secrets。生成方式：
```bash
base64 -w 0 your-keystore.jks > keystore_base64.txt
# 将 keystore_base64.txt 的内容设为 KEYSTORE_BASE64 secret
```

---

## 配合其他 Skill 使用

| Skill | 配合方式 |
|-------|---------|
| [sftp-deploy](../sftp-deploy/skill.md) | SFTP 上传的详细配置参考 |
| [appflowy-editor](../appflowy-editor/skill.md) | Flutter 编辑器集成 |
| [auto-deploy](../auto-deploy/skill.md) | Web 项目自动部署 |
