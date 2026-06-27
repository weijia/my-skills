# Skill: Android 原生项目构建 + 版本号 + 签名 + SFTP 上传
> 纯 Java Android 项目一键配置 GitHub Actions：自动版本号递增、Release 签名、SFTP 上传 APK。
---
## 概述
本 skill 适用于**原生 Android 项目（Groovy DSL，纯 Java，无 Flutter/Kotlin）**，将以下流程整合到 GitHub Actions：
| 功能 | 说明 |
|------|------|
| **版本号自动递增** | `versionCode` 使用 `github.run_number` 自动递增，`versionName` 保持手动管理或 Git tag 驱动 |
| **Release 签名** | 通过 GitHub Secrets 持久化 keystore，每次构建使用同一签名 |
| **SFTP 上传** | APK 自动上传到远程服务器，保留最近 N 个版本 |
| **原地更新** | 相同签名 + 递增 versionCode = 直接覆盖安装，不需要卸载 |

**原地更新的三个前提条件（Android 系统强制）**：
| 条件 | 说明 | 本 Skill 支持 |
|------|------|--------------|
| 相同包名 | `applicationId` 必须一致 | 由项目配置保证 |
| 相同签名 | 使用同一 keystore 签名 | 通过 GitHub Secrets 持久化 |
| versionCode 递增 | 新版本号必须大于旧版本 | 使用 `github.run_number` 自动递增 |

**版本号策略**：
| 场景 | versionCode | versionName |
|------|------------|-------------|
| GitHub Actions 构建 | `github.run_number`（自动递增） | `build.gradle` 中的固定值（如 `1.0.0`） |
| 本地构建 | `build.gradle` 中的固定值（如 `127`） | `build.gradle` 中的固定值 |
| Git tag 触发 | `github.run_number` | Git tag 值（如 `1.1.0`） |

> 版本号中**不含 `+` 号**（Android 不允许，会导致安装时报 "package info is null"）

---
## 前置条件
- 原生 Android 项目（Groovy DSL `build.gradle`）
- GitHub 仓库
- JDK 17（CI 推荐）
- SFTP 服务器（可选，不上传可跳过）
- Android SDK API Level 14+（兼容几乎所有设备）

---
## 步骤一：复制 Workflow 模板
将 `templates/build-release.yml` 复制到目标项目的 `.github/workflows/`。
根据项目修改以下占位符：
| 占位符 | 替换为 |
|--------|--------|
| `【项目名】` | 你的项目名（用于 APK 文件名） |

---
## 步骤二：配置 build.gradle 签名
确保 `app/build.gradle` 中的签名配置与模板匹配。标准配置如下：
```groovy
// 签名配置 - 在 android 块之后定义
if (System.getenv("KEYSTORE_PATH") != null && System.getenv("KEYSTORE_PASSWORD") != null) {
    android.signingConfigs {
        release {
            storeFile file(System.getenv("KEYSTORE_PATH"))
            storePassword System.getenv("KEYSTORE_PASSWORD")
            keyAlias System.getenv("KEY_ALIAS") ?: "release"
            keyPassword System.getenv("KEY_PASSWORD") ?: System.getenv("KEYSTORE_PASSWORD")
        }
    }
    android.buildTypes.release.signingConfig android.signingConfigs.release
}
```

**为什么在 `android {}` 块之后定义？**
在 Groovy DSL 中，如果在 `android {}` 块内直接使用 `System.getenv()` 并调用 `file()` 方法，会导致：
```
No signature of method: java.lang.String.call() is applicable for argument types: (String)
```
将签名配置放在 `android {}` 块之后，可以正确访问已初始化的 `android` 对象。

**本地构建不受影响**：本地不设置环境变量时，`System.getenv("KEYSTORE_PATH")` 为 null，整个 if 块跳过，使用 debug 签名正常构建。

---
## 步骤三：生成 Keystore 并配置 GitHub Secrets
### 1. 生成 keystore（只需一次）
```bash
keytool -genkeypair \
  -keystore release-keystore.jks \
  -keyalg RSA -keysize 2048 -validity 36500 \
  -alias release \
  -storepass your-password \
  -keypass your-password \
  -dname "CN=YourApp, OU=Dev, O=YourOrg, C=CN"
```

### 2. 转为 Base64
```bash
base64 -w 0 release-keystore.jks > keystore_base64.txt
```

### 3. 配置 GitHub Secrets
```bash
PROJ="【项目名】"
USERNAME="【GitHub用户名】"
ROOT="${USERNAME}/${PROJ}"

# Keystore 签名 Secrets
gh secret set KEYSTORE_BASE64 --repo "$ROOT" --body "$(cat keystore_base64.txt)"
gh secret set KEYSTORE_PASSWORD --repo "$ROOT" --body "your-password"
gh secret set KEY_ALIAS --repo "$ROOT" --body "release"
gh secret set KEY_PASSWORD --repo "$ROOT" --body "your-password"

# SFTP Secrets（可选）
gh secret set SFTP_HOST --repo "$ROOT" --body "your-sftp-host"
gh secret set SFTP_PORT --repo "$ROOT" --body "22"
gh secret set SFTP_USERNAME --repo "$ROOT" --body "sftp-username"
gh secret set SFTP_PASSWORD --repo "$ROOT" --body "sftp-password"
```

### Secrets 清单
| Secret 名称 | 必填 | 说明 |
|-------------|------|------|
| `KEYSTORE_BASE64` | 签名时 | Base64 编码的 keystore 文件 |
| `KEYSTORE_PASSWORD` | 签名时 | Keystore 密码 |
| `KEY_ALIAS` | 签名时 | Key 别名（默认 `release`） |
| `KEY_PASSWORD` | 签名时 | Key 密码 |
| `SFTP_HOST` | SFTP 时 | 服务器地址 |
| `SFTP_PORT` | SFTP 时 | 端口（默认 `22`） |
| `SFTP_USERNAME` | SFTP 时 | 用户名 |
| `SFTP_PASSWORD` | SFTP 时 | 密码 |

> 未配置 keystore secrets 时，workflow 自动回退到 debug 构建（无签名）。

---
## 步骤四：APP 内更新检查（可选）
将 `templates/AppUpdateChecker.java` 复制到项目中。
该工具类提供：
- 检查服务器上是否有新版本 APK
- 下载 APK 到本地
- 触发系统安装（兼容 Android 4.0 ~ Android 14+）

在 `MainActivity` 中使用：
```java
// 在 onCreate 或连接成功后检查更新
AppUpdateChecker.checkForUpdate(this,
    "https://your-server.com/apps/stock-app/version.json");
```

**`version.json` 格式**（放在 SFTP 同级目录）：
```json
{
  "versionCode": 128,
  "versionName": "1.0.0",
  "downloadUrl": "https://your-server.com/apps/stock-app/stock-app-v1.0.0-build128.apk",
  "changelog": "修复了若干问题"
}
```

**兼容性说明**：
| Android 版本 | 安装行为 | 需要权限 |
|-------------|---------|---------|
| 4.0 - 7.1 | 直接安装 | `INTERNET` |
| 8.0 - 13 | 需要安装权限 | `REQUEST_INSTALL_PACKAGES` + 文件 Provider |
| 14+ | 同上 + 用户确认 | 同上 |

`AppUpdateChecker.java` 内部已处理 FileProvider 和权限请求。

---
## 触发方式
### 自动触发
每次 push 到 main 分支自动构建并上传：
```bash
git push origin main
```

### 手动触发
在 GitHub Actions 页面手动触发 workflow。

### Tag 触发
```bash
git tag v1.1.0
git push origin v1.1.0
```
Tag 触发时会同时创建 GitHub Release。

---
## SFTP 上传结果示例
```
/home/github/apps/stock-android-app/
├── stock-app-v1.0.0-build128.apk    ← 最新
├── stock-app-v1.0.0-build127.apk
├── stock-app-v1.0.0-build126.apk
├── stock-app-v1.0.0-build125.apk
└── stock-app-v1.0.0-build124.apk    ← 最旧（下次构建时自动删除）
```
超过 `max-keep-files` 数量时，自动删除最旧的文件。

---
## 常见问题
### Q: 新 APK 无法覆盖安装，提示"未安装应用"
**原因**：签名不一致。每次构建使用了不同的 keystore。
**解决**：
1. 确保只生成一次 keystore
2. 确保 `KEYSTORE_BASE64` secret 始终是同一个文件
3. 不要删除 secret 后重新生成

### Q: 安装 APK 报 "package info is null"
**原因**：版本号包含 `+` 号（如 `1.0.0+5`），Android 无法解析。
**解决**：确保 versionName 不含 `+` 号。

### Q: versionCode 冲突
**原因**：GitHub Actions 的 `run_number` 全仓库共享。如果同时运行多个 workflow，可能产生相同的 run_number。
**解决**：workflow 模板中 `commit versionCode update back` 步骤会在构建后提交更新后的 versionCode 到仓库，确保一致性。

### Q: 本地构建不受 CI 签名影响
本地构建时没有环境变量，`if` 块跳过，使用 debug 签名。本地 debug APK 和 CI release APK 的签名不同，无法互相覆盖安装。这是正常行为。

### Q: keystore 丢失怎么办？
**无法恢复**。一旦 keystore 丢失，用户必须卸载旧版本才能安装新版本，所有数据将丢失。
建议：
1. 将 keystore 文件备份到安全的地方（如加密云存储）
2. 不要在多台机器上分别生成 keystore
3. 记录密码并妥善保存

---
## 注意事项
- **keystore 是不可替代的**，丢失后无法再发布原地更新版本
- `versionCode` 必须严格递增，Android 不允许降级安装
- `REQUEST_INSTALL_PACKAGES` 权限在 Android 8.0+ 需要动态申请
- 无第三方依赖，仅使用 Android SDK 原生 API，兼容 API Level 14+

---
## 配合其他 Skill 使用
| Skill | 配合方式 |
|-------|---------|
| [sftp-deploy](../sftp-deploy/skill.md) | SFTP 上传的详细配置参考 |
| [version-display](../version-display/skill.md) | 版本号展示组件 |
| [android-external-storage](../android-external-storage/skill.md) | Android 外部存储配置持久化 |
