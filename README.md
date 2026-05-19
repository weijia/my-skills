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
└── [skill-name]/              ← 更多 skills...
    ├── skill.md
    └── templates/
        └── ...
```

## Skills

| Skill | 说明 | 触发方式 |
|-------|------|----------|
| [auto-deploy](./auto-deploy/skill.md) | WebDAV + GitHub Pages 双通道自动部署 | git push tag (`v*`) |
| [version-display](./version-display/skill.md) | Web 应用显示 GitHub tag 版本号和发布时间 | 构建时自动注入 |

## 如何使用

1. 找到需要的 skill 目录
2. 阅读 `skill.md` 了解配置步骤
3. 将 `templates/` 中的模板文件复制到目标项目
4. 按文档说明配置 secrets 和参数
