import 'dart:convert';
import 'dart:io';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:docman/docman.dart';

/// 设置服务 - 支持 SAF 公共目录和 shared_preferences 双重存储
///
/// 优先级：
/// 1. SAF 公共目录（用户选择）- 卸载后保留
/// 2. shared_preferences - 降级方案
class SettingsService {
  static final SettingsService _instance = SettingsService._internal();
  factory SettingsService() => _instance;
  SettingsService._internal();

  // ============ 配置项 ============
  // 根据你的项目修改这些 key
  static const _keyConfig = 'app_config';
  static const _keyDirUri = 'saf_dir_uri';
  static const _settingsFileName = 'app_settings.json';

  SharedPreferences? _prefs;
  DocumentFile? _safDir;

  // ============ 配置数据 ============
  // 根据你的项目添加配置字段
  Map<String, dynamic> _config = {};

  // 日志回调
  void Function(String)? onLog;

  // Getters
  Map<String, dynamic> get config => _config;
  bool get hasSafDirectory => _safDir != null;
  String? get safDirectoryUri => _safDir?.uri;

  void _log(String message) {
    final line = '[Settings] $message';
    print(line);
    onLog?.call(line);
  }

  /// 初始化并加载设置
  Future<void> loadSettings() async {
    _log('初始化...');
    _prefs = await SharedPreferences.getInstance();

    // 1. 尝试从 SAF 公共目录加载
    final safUri = _prefs!.getString(_keyDirUri);
    if (safUri != null && safUri.isNotEmpty && Platform.isAndroid) {
      _log('尝试从 SAF 目录加载: $safUri');
      try {
        _safDir = await DocumentFile.fromUri(safUri);
        if (_safDir != null) {
          final safContent = await _readFromSaf();
          if (safContent != null) {
            _log('从 SAF 加载成功');
            _parseConfig(safContent);
            return;
          }
        }
      } catch (e) {
        _log('SAF 加载失败: $e');
        _safDir = null;
      }
    }

    // 2. 从 shared_preferences 加载（降级）
    _log('从 shared_preferences 加载');
    _loadFromSharedPreferences();
  }

  /// 从 SAF 读取配置文件内容
  Future<String?> _readFromSaf() async {
    if (_safDir == null) return null;

    try {
      final documents = await _safDir!.listDocuments();
      final settingsFile = documents.firstWhere(
        (doc) => doc.name == _settingsFileName,
        orElse: () => throw Exception('配置文件不存在'),
      );
      final bytes = await settingsFile.read();
      if (bytes == null) return null;
      return utf8.decode(bytes);
    } catch (e) {
      _log('读取 SAF 配置失败: $e');
      return null;
    }
  }

  /// 从 shared_preferences 加载
  void _loadFromSharedPreferences() {
    final configJson = _prefs!.getString(_keyConfig);
    if (configJson != null && configJson.isNotEmpty) {
      try {
        _config = jsonDecode(configJson) as Map<String, dynamic>;
        _log('配置已加载');
      } catch (e) {
        _log('配置解析失败: $e');
        _config = {};
      }
    } else {
      _log('无配置');
      _config = {};
    }
  }

  /// 解析配置内容
  void _parseConfig(String content) {
    try {
      final json = jsonDecode(content) as Map<String, dynamic>;
      _config = json;
      _log('配置解析成功');
    } catch (e) {
      _log('解析配置失败: $e');
      _config = {};
    }
  }

  /// 请求用户选择 SAF 目录
  Future<bool> requestSafDirectory() async {
    if (!Platform.isAndroid) {
      _log('非 Android 平台，跳过 SAF');
      return false;
    }

    try {
      _log('打开 SAF 目录选择器...');
      final dir = await DocMan.pick.directory();
      if (dir != null) {
        _safDir = dir;
        await _prefs?.setString(_keyDirUri, dir.uri);
        _log('SAF 目录已选择: ${dir.uri}');
        return true;
      }
      return false;
    } catch (e) {
      _log('SAF 目录选择失败: $e');
      return false;
    }
  }

  /// 保存配置
  Future<void> saveConfig(Map<String, dynamic> config) async {
    _config = config;
    await _saveAll();
    _log('配置已保存');
  }

  /// 更新单个配置项
  Future<void> updateConfig(String key, dynamic value) async {
    _config[key] = value;
    await _saveAll();
    _log('配置项 $key 已更新');
  }

  /// 清除配置
  Future<void> clearConfig() async {
    _config = {};
    await _saveAll();
    _log('配置已清除');
  }

  /// 保存所有配置
  Future<void> _saveAll() async {
    final content = jsonEncode(_config);

    // 1. 尝试保存到 SAF
    if (_safDir != null) {
      try {
        await _writeToSaf(content);
        _log('已保存到 SAF');
      } catch (e) {
        _log('保存到 SAF 失败: $e');
      }
    }

    // 2. 同时保存到 shared_preferences（备份）
    await _prefs?.setString(_keyConfig, content);
    if (_safDir != null) {
      await _prefs?.setString(_keyDirUri, _safDir!.uri);
    }
  }

  /// 写入到 SAF
  Future<void> _writeToSaf(String content) async {
    if (_safDir == null) return;

    final bytes = utf8.encode(content);
    // 先查找是否存在配置文件
    final existingFile = await _safDir!.find(_settingsFileName);
    if (existingFile != null) {
      // 存在则删除后重新创建
      await existingFile.delete();
    }
    // 创建新文件
    await _safDir!.createFile(
      name: _settingsFileName,
      bytes: bytes,
    );
  }

  /// 恢复 SAF 目录（从保存的 URI）
  Future<void> restoreSafDirectory() async {
    final uri = _prefs?.getString(_keyDirUri);
    if (uri == null || uri.isEmpty) {
      _log('无保存的 SAF 目录 URI');
      return;
    }

    try {
      _log('恢复 SAF 目录: $uri');
      final dir = await DocumentFile.fromUri(uri);
      if (dir != null && dir.exists) {
        _safDir = dir;
        _log('SAF 目录已恢复: ${dir.uri}');
      } else {
        _log('SAF 目录不存在或无权限');
      }
    } catch (e) {
      _log('恢复 SAF 目录失败: $e');
    }
  }

  /// 获取配置项
  T? get<T>(String key, {T? defaultValue}) {
    return _config[key] as T? ?? defaultValue;
  }

  /// 设置配置项
  Future<void> set<T>(String key, T value) async {
    _config[key] = value;
    await _saveAll();
  }
}
