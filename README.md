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
├── zen-fs-config/              ← Node.js 分布式配置管理（ZenFS + 同步 + 冲突安全）
│   ├── skill.md               # 使用文档 + AI Agent 提示词
│   └── templates/
│       └── config-init.ts     # ConfigRepo 初始化模板
├── npm-auto-publish/           ← npm 包自动发布到 npmjs
│   ├── skill.md               # 使用文档
│   └── templates/
│       └── publish.yml        # GitHub Actions 发布模板
├── doc-writer/                ← 开发项目文档写作（需求 / 用例 / 设计）· CodeBuddy skill
│   ├── skill.md               # frontmatter + 写作纪律（Windows 下 SKILL.md 大小写不敏感，等同识别）
│   └── templates/
│       ├── requirements-template.md  # 需求文档骨架
│       ├── usecase-template.md       # 用例文档骨架
│       └── design-template.md        # 设计文档骨架
├── testcase-writer/           ← 测试用例文档写作（计划 / 用例 / 清单 / 报告）· CodeBuddy skill
│   ├── SKILL.md               # frontmatter + 写作纪律（Windows 下 skill.md 同名，故用 SKILL.md）
│   └── templates/
│       └── testcase-template.md     # 测试用例文档骨架
├── cpp-daemon-mqtt-hang/      ← 修复 MQTT 挂起导致退出后进程不退 / 托盘图标残留
│   ├── skill.md               # 根因分析 + 超时修复方案 + 复盘清单
│   └── templates/
│       └── mqtt-timeout-reference.cpp  # 连接/断开/订阅/HTTP 带超时代码模式
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
| [zen-fs-config](./zen-fs-config/skill.md) | Node.js 分布式配置管理：多应用隔离、多后端同步、节点本地配置、冲突安全 | 按需集成 |
| [npm-auto-publish](./npm-auto-publish/skill.md) | npm 包自动发布：GitHub Actions CI/CD，push tag 即构建测试并发布到 npmjs | git push tag (`v*`) |
| [doc-writer](./doc-writer/SKILL.md) | 开发项目文档写作：需求（PRD）/ 用例 / 设计文档的结构化规范与模板 | 写 / 改需求、用例、设计文档时 |
| [testcase-writer](./testcase-writer/SKILL.md) | 测试用例写作：测试计划 / 用例 / 清单 / 报告，含可追溯矩阵与正负向/边界覆盖 | 写 / 改测试文档时 |
| [cpp-daemon-mqtt-hang](./cpp-daemon-mqtt-hang/skill.md) | 修复 paho-mqtt-cpp 挂起导致托盘退出后进程不退 / 图标残留（连接/断开/上传加有限超时 + 锁外 wait） | 遇到 MQTT 线程退出卡死 / join 不返回时 |

## 如何使用

1. 找到需要的 skill 目录
2. 阅读 `skill.md` 了解配置步骤
3. 将 `templates/` 中的模板文件复制到目标项目
4. 按文档说明配置 secrets 和参数
