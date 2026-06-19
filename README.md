# my-skills

个人技能库 — 可复用的自动化 Skill 合集。

## 目录结构

```
my-skills/
├── README.md                  ← 你在这里
├── auto-deploy/               ← 自动部署～无密码
│   ├── skill.md               # 使用文档
│   └── templates/
│       └── deploy.yml         # GitHub Actions 模板
├── appflowy-editor/           ← AppFlowy Editor～Flutter 富文本编辑器集成
│   ├── skill.md               # 使用文档 + 踩坑记录
│   └── templates/
│       ├── editor_basic.dart  # 编辑器 Widget 模板（含工具栏、表格）
│       └── android_signing.gradle  # Android 固定签名配置
├── sftp-deploy/               ← SFTP 上传 APK/APP 等安装包
│   ├── skill.md               # 使用文档
│   └── templates/
│       └── build-upload.yml   # SFTP 上传模板
├── version-display/           ← Web 应用显示版本号
│   ├── skill.md               # 使用文档
│   └── templates/
│       └── version.yml        # 版本注入模板
├── flutter-build-release/      ← Flutter 多平台构建 + 版本号 + SFTP 上传
│   ├── skill.md               # 使用文档
│   └── templates/
│       ├── generate_version.sh  # 版本号生成脚本
│       ├── build-all-platforms.yml  # 多平台构建 workflow
│       ├── version.dart        # Dart 版本信息读取模板
│       └── android_signing.gradle.kts  # Android 签名配置（可更新 APK）
├── android-external-storage/   ← Android 外部存储配置持久化
│   ├── skill.md               # 使用文档
│   └── templates/
│       └── settings_service.dart  # 配置服务模板
└── [skill-name]/              ← 更多 skills...
    ├── skill.md
    └── templates/
        └── ...
```

## Skills

| Skill | 说明 | 触发方式 |
|-------|------|----------|
| [flutter-build-release](./flutter-build-release/skill.md) | Flutter 多平台构建 + 自动版本号 + SFTP 上传 + **可更新 APK** | push 代码或手动触发 |
| [android-external-storage](./android-external-storage/skill.md) | Android 配置保存到外部存储（SAF），卸载后配置保留 | 按需集成 |
| [auto-deploy](./auto-deploy/skill.md) | WebDAV (版本+latest) + GitHub Pages 三通道自动部署，自动清理旧版本 | git push tag (`v*`) |
| [version-display](./version-display/skill.md) | Web 应用显示 GitHub tag 版本号和发布时间 | 构建时自动注入 |
| [sftp-deploy](./sftp-deploy/skill.md) | SFTP 上传 APK/APP 等安装包到远程服务器 | push 代码或手动触发 |
| [appflowy-editor](./appflowy-editor/skill.md) | Flutter AppFlowy Editor 集成：富文本编辑、表格、列表、工具栏 | 按需集成 |

## 如何使用

1. 找到需要的 skill 目录
2. 阅读 `skill.md` 了解配置步骤
3. 将 `templates/` 中的模板文件复制到目标项目
4. 按文档说明配置 secrets 和参数
