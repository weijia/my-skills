// version.dart - 版本信息（由 GitHub Actions 构建时注入）
// 在 Dart 代码中通过 String.fromEnvironment 读取编译时变量

class AppVersion {
  /// 版本号（如 1.1.0 或 0.0.0-20260609.143000CST）
  static const String versionName = String.fromEnvironment('VERSION_NAME', defaultValue: '0.0.0');

  /// 构建类型（tag 或 datetime）
  static const String buildType = String.fromEnvironment('VERSION_TYPE', defaultValue: 'dev');

  /// Git tag（如 v1.1.0，无 tag 时为空）
  static const String buildTag = String.fromEnvironment('VERSION_TAG', defaultValue: '');

  /// 构建时间（如 20260609.143000）
  static const String buildDatetime = String.fromEnvironment('VERSION_DATETIME', defaultValue: '');

  /// 时区（如 CST）
  static const String buildTimezone = String.fromEnvironment('VERSION_TIMEZONE', defaultValue: '');

  /// 格式化显示（如 v1.1.0 或 0.0.0 (2026-06-09 14:30 CST)）
  static String get display {
    if (buildTag.isNotEmpty) {
      return buildTag;
    }
    if (buildDatetime.isNotEmpty) {
      // 20260609.143000 → 2026-06-09 14:30
      final dt = buildDatetime;
      if (dt.length >= 15) {
        final formatted = '${dt.substring(0, 4)}-${dt.substring(4, 6)}-${dt.substring(6, 8)} '
            '${dt.substring(9, 11)}:${dt.substring(11, 13)}';
        return '$versionName ($formatted $buildTimezone)';
      }
    }
    return versionName;
  }
}
