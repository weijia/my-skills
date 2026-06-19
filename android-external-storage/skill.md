## Skill: Android 外部存储配置持久化

> Flutter Android 应用配置保存到外部存储（SAF），卸载后配置保留

---

## 纯 Java Android 项目使用

> 适用于使用 Groovy DSL (`build.gradle`) 的传统 Android 项目，兼容 API Level 14+。

### 配置步骤

**1. 复制配置服务文件**
```bash
cp templates/settings_service.java your-project/app/src/main/java/com/your_app/util/ExternalStorageManager.java
```

**2. 修改包名和应用名**
编辑 `ExternalStorageManager.java`：
```java
// 第 32 行：替换为你的应用名
private static final String LEGACY_CONFIG_DIR = "YourApp";

// 第 1 行：替换为你的包名
package com.your_app.util;
```

**3. 添加存储权限**
编辑 `AndroidManifest.xml`：
```xml
<!-- 外部存储权限（用于 Android 4.0-4.3） -->
<uses-permission 
    android:name="android.permission.WRITE_EXTERNAL_STORAGE"
    android:maxSdkVersion="28" />
<uses-permission 
    android:name="android.permission.READ_EXTERNAL_STORAGE"
    android:maxSdkVersion="28" />
```

**4. 在 Activity 中使用**
```java
public class MainActivity extends Activity {
    private ExternalStorageManager externalStorageManager;
    
    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        
        externalStorageManager = new ExternalStorageManager(this);
        
        // 加载配置
        JSONObject config = externalStorageManager.loadConfig();
        String serverIp = config.optString("server_ip", "localhost");
        
        // 保存配置
        JSONObject newConfig = new JSONObject();
        newConfig.put("server_ip", "192.168.1.100");
        externalStorageManager.saveConfig(newConfig);
    }
    
    @Override
    protected void onActivityResult(int requestCode, int resultCode, Intent data) {
        super.onActivityResult(requestCode, resultCode, data);
        
        if (externalStorageManager.handleSafResult(requestCode, resultCode, data)) {
            // SAF 目录已选择，配置将保存到外部存储
        }
    }
    
    // 选择外部存储目录
    private void selectExternalStorage() {
        if (externalStorageManager.isSafAvailable()) {
            externalStorageManager.requestSafDirectory(this);
        }
    }
}
```

### 版本兼容说明

| Android 版本 | 存储方式 | 说明 |
|--------------|----------|------|
| 4.0-4.3 (API 14-18) | 外部存储直接写入 | `/sdcard/YourApp/settings.json` |
| 4.4+ (API 19+) | SAF | 用户选择的外部目录 |
| 10+ (API 29+) | Scoped Storage | 应用专属外部目录 |

### 配置保存位置

配置会同时保存到三个位置，确保最大兼容性：

1. **SAF 目录**（如果用户选择）- 卸载后保留
2. **外部存储公共目录** - 卸载后保留（Android 4.0-4.3）
3. **SharedPreferences** - 降级方案

### 加载优先级

```
SAF → 外部存储 → SharedPreferences
```

---

## 概述

Android 应用的配置默认保存在应用内部存储，卸载后会丢失。本 skill 使用 **Storage Access Framework (SAF)** 将配置保存到用户选择的外部目录，实现：

- **卸载后配置保留** - 重装应用后自动恢复配置
- **用户可控** - 用户选择保存位置
- **双重备份** - 同时保存到 SAF 和 SharedPreferences

---

## Android 版本要求

| Android 版本 | API Level | SAF 支持 | 推荐方案 |
|-------------|-----------|---------|---------|
| Android 4.0-4.3 | 14-18 | ❌ 不支持 | 外部存储直接写入 |
| Android 4.4+ | 19+ | ✅ 支持 | SAF（本 skill） |

**重要说明：**
- Storage Access Framework (SAF) 在 **Android 4.4 (API 19)** 引入
- Android 4.0-4.3 市场占有率极低（< 0.1%），建议将最低 SDK 设为 19 或更高
- 如需支持 Android 4.0-4.3，请参考下方的替代方案

---

## 前置条件

- Flutter 项目
- **Android 4.4 (API 19) 或更高版本**
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

## Android 4.0-4.3 替代方案

如果需要支持 Android 4.4 以下的系统，可以使用以下替代方案：

### 方案一：外部存储直接写入

需要 `WRITE_EXTERNAL_STORAGE` 权限，配置保存在公共目录（如 `/sdcard/YourApp/`）。

**1. 添加权限**

```xml
<!-- android/app/src/main/AndroidManifest.xml -->
<uses-permission android:name="android.permission.WRITE_EXTERNAL_STORAGE" />
<uses-permission android:name="android.permission.READ_EXTERNAL_STORAGE" />
```

**2. 配置服务代码**

```dart
import 'dart:convert';
import 'dart:io';

class LegacySettingsService {
  static const _configDir = '/sdcard/YourApp';
  static const _configFile = 'settings.json';
  
  Map<String, dynamic> _config = {};
  
  Future<void> loadSettings() async {
    try {
      final file = File('$_configDir/$_configFile');
      if (await file.exists()) {
        final content = await file.readAsString();
        _config = jsonDecode(content) as Map<String, dynamic>;
      }
    } catch (e) {
      _config = {};
    }
  }
  
  Future<void> saveSettings() async {
    try {
      final dir = Directory(_configDir);
      if (!await dir.exists()) {
        await dir.create(recursive: true);
      }
      final file = File('$_configDir/$_configFile');
      await file.writeAsString(jsonEncode(_config));
    } catch (e) {
      print('保存配置失败: $e');
    }
  }
  
  Map<String, dynamic> get config => _config;
}
```

**缺点：**
- 需要申请存储权限
- 用户可能拒绝权限
- Android 10+ 需要使用 Scoped Storage

### 方案二：应用外部存储目录

使用 `getExternalStorageDirectory()` 获取应用专属外部目录。

**1. 添加依赖**

```yaml
dependencies:
  path_provider: ^2.0.0
```

**2. 配置服务代码**

```dart
import 'dart:convert';
import 'dart:io';
import 'package:path_provider/path_provider.dart';

class LegacySettingsService {
  static const _configFile = 'settings.json';
  
  Map<String, dynamic> _config = {};
  String? _configPath;
  
  Future<void> loadSettings() async {
    try {
      final dir = await getExternalStorageDirectory();
      if (dir == null) return;
      
      _configPath = '${dir.path}/$_configFile';
      final file = File(_configPath!);
      
      if (await file.exists()) {
        final content = await file.readAsString();
        _config = jsonDecode(content) as Map<String, dynamic>;
      }
    } catch (e) {
      _config = {};
    }
  }
  
  Future<void> saveSettings() async {
    if (_configPath == null) return;
    
    try {
      final file = File(_configPath!);
      await file.writeAsString(jsonEncode(_config));
    } catch (e) {
      print('保存配置失败: $e');
    }
  }
  
  Map<String, dynamic> get config => _config;
}
```

**缺点：**
- 卸载应用后目录会被删除
- 配置无法保留

### 方案三：版本兼容处理

根据 Android 版本自动选择存储方案：

```dart
import 'dart:io';
import 'package:flutter/services.dart';

class SettingsService {
  static const int SAF_MIN_API = 19; // Android 4.4
  
  Future<bool> _isSafSupported() async {
    if (!Platform.isAndroid) return false;
    
    try {
      final version = await MethodChannel('device_info').invokeMethod<int>('androidVersion');
      return version != null && version >= SAF_MIN_API;
    } catch (e) {
      return false;
    }
  }
  
  Future<void> loadSettings() async {
    if (await _isSafSupported()) {
      // 使用 SAF 方案
      await _loadFromSaf();
    } else {
      // 使用传统方案
      await _loadFromExternalStorage();
    }
  }
}
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
5. **版本兼容** - Android 4.4 以下不支持 SAF，需使用替代方案

---

## 常见问题

### Q: 重装应用后 SAF 目录访问失败

**原因**：SAF 权限与应用签名绑定，重装后签名变化导致权限失效。

**解决**：检测访问失败后，提示用户重新选择目录。

### Q: 用户选择了不可访问的目录

**解决**：在选择后立即尝试写入测试文件，验证权限。

### Q: 配置文件损坏

**解决**：使用 try-catch 包裹解析逻辑，失败时使用默认配置。

### Q: 如何支持 Android 4.0-4.3？

**解决**：参考上方「Android 4.0-4.3 替代方案」章节，使用外部存储直接写入或版本兼容处理。

---

## 配合其他 Skill 使用

| Skill | 配合方式 |
|-------|----------|
| [flutter-build-release](../flutter-build-release/skill.md) | 构建可更新 APK |
| [sftp-deploy](../sftp-deploy/skill.md) | 上传 APK 到服务器 |
