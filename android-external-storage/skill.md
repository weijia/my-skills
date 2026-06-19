## Skill: Android 外部存储配置持久化

> Flutter Android 应用配置保存到外部存储（SAF），卸载后配置保留

---

## 概述

Android 应用的配置默认保存在应用内部存储，卸载后会丢失。本 skill 使用 **Storage Access Framework (SAF)** 将配置保存到用户选择的外部目录，实现：

- **卸载后配置保留** - 重装应用后自动恢复配置
- **用户可控** - 用户选择保存位置
- **双重备份** - 同时保存到 SAF 和 SharedPreferences

---

## 前置条件

- Flutter 项目
- Android 平台
- 添加依赖：`docman`（SAF 操作库）

```yaml
# pubspec.yaml
dependencies:
  docman: ^1.0.0
  shared_preferences: ^2.0.0
```

---

## 步骤一：添加依赖

```bash
flutter pub add docman shared_preferences
```

---

## 步骤二：复制配置服务模板

将 `templates/settings_service.dart` 复制到项目的 `lib/services/` 目录。

---

## 步骤三：使用配置服务

### 初始化

```dart
final settingsService = SettingsService();
await settingsService.loadSettings();

// 恢复 SAF 目录（从保存的 URI）
await settingsService.restoreSafDirectory();
```

### 请求用户选择目录

```dart
if (!settingsService.hasSafDirectory) {
  final success = await settingsService.requestSafDirectory();
  if (!success) {
    // 用户取消或失败，使用 SharedPreferences 降级
  }
}
```

### 保存配置

```dart
// 保存自定义配置
await settingsService.setGitConfig(myConfig);

// 保存 UI 状态
await settingsService.saveUiState(
  notePath: '/path/to/note.md',
  sourceMode: true,
);
```

### 读取配置

```dart
final config = settingsService.gitConfig;
final lastNote = settingsService.lastOpenedNotePath;
```

---

## 工作原理

### 存储优先级

```
1. SAF 公共目录（用户选择）→ 卸载后保留
2. SharedPreferences → 降级方案
```

### 配置文件格式

```json
{
  "gitConfig": {
    "repoUrl": "https://github.com/user/repo.git",
    "branch": "main",
    ...
  },
  "lastOpenedNotePath": "/path/to/note.md",
  "lastOpenedFolderPath": "/path/to/folder",
  "lastSourceMode": false,
  "showArchived": true,
  "safDirectoryUri": "content://com.android.externalstorage.documents/..."
}
```

### SAF 目录 URI 持久化

- SAF 目录 URI 保存到配置文件和 SharedPreferences
- 重装应用后从 SharedPreferences 恢复 URI
- 使用 `DocumentFile.fromUri()` 恢复目录访问

---

## 关键代码说明

### 1. 选择 SAF 目录

```dart
Future<bool> requestSafDirectory() async {
  final dir = await DocMan.pick.directory();
  if (dir != null) {
    _safDir = dir;
    await _prefs?.setString(_keyDirUri, dir.uri);
    return true;
  }
  return false;
}
```

### 2. 写入配置到 SAF

```dart
Future<void> _writeToSaf(String content) async {
  final bytes = utf8.encode(content);
  // 删除旧文件
  final existingFile = await _safDir!.find(_settingsFileName);
  if (existingFile != null) {
    await existingFile.delete();
  }
  // 创建新文件
  await _safDir!.createFile(
    name: _settingsFileName,
    bytes: bytes,
  );
}
```

### 3. 从 SAF 读取配置

```dart
Future<String?> _readFromSaf() async {
  final documents = await _safDir!.listDocuments();
  final settingsFile = documents.firstWhere(
    (doc) => doc.name == _settingsFileName,
    orElse: () => throw Exception('配置文件不存在'),
  );
  final bytes = await settingsFile.read();
  return bytes != null ? utf8.decode(bytes) : null;
}
```

### 4. 恢复 SAF 目录

```dart
Future<void> restoreSafDirectory() async {
  final uri = _prefs?.getString(_keyDirUri);
  if (uri != null) {
    final dir = await DocumentFile.fromUri(uri);
    if (dir != null && dir.exists) {
      _safDir = dir;
    }
  }
}
```

---

## 注意事项

1. **权限持久性** - SAF 授权在应用重启后仍然有效，但重装应用后需要重新授权
2. **降级处理** - 如果 SAF 不可用，自动降级到 SharedPreferences
3. **iOS 不支持** - SAF 是 Android 特有功能，iOS 使用 iCloud 或其他方案
4. **文件覆盖** - 写入时先删除旧文件再创建新文件

---

## 常见问题

### Q: 重装应用后 SAF 目录访问失败

**原因**：SAF 权限与应用签名绑定，重装后签名变化导致权限失效。

**解决**：检测访问失败后，提示用户重新选择目录。

### Q: 用户选择了不可访问的目录

**解决**：在选择后立即尝试写入测试文件，验证权限。

### Q: 配置文件损坏

**解决**：使用 try-catch 包裹解析逻辑，失败时使用默认配置。

---

## 配合其他 Skill 使用

| Skill | 配合方式 |
|-------|----------|
| [flutter-build-release](../flutter-build-release/skill.md) | 构建可更新 APK |
| [sftp-deploy](../sftp-deploy/skill.md) | 上传 APK 到服务器 |
