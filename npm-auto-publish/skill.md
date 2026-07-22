# Skill: npm 自动发布

> 给 npm 包项目配置 GitHub Actions，push tag 即自动构建、测试并发布到 npmjs。

---

## 概述

本 skill 用于给 **Node.js/npm 包项目** 配置 CI/CD 自动发布：

| 步骤 | 动作 | 说明 |
|------|------|------|
| **Install** | `npm ci` | 基于 lock 文件安装依赖 |
| **Test** | `npm test` | 运行单元测试 |
| **Build** | `npm run build` | 编译 TypeScript / 打包 |
| **Publish** | `npm publish --access public` | 发布到 npmjs registry |

**触发条件**：推送 `v*` 格式的 tag（如 `v1.2.0`），或手动触发 workflow。

---

## 前置条件

- 项目为 npm 包，已配置 `package.json`（含 `build` / `test` / `prepublishOnly` 脚本）
- 已安装 `gh` CLI 并登录（`gh auth login`）
- 拥有 npm 账号和 publish 权限的 Access Token

---

## 步骤一：复制 Workflow 模板

将本目录下的 [`templates/publish.yml`](./templates/publish.yml) 复制到目标项目的 `.github/workflows/publish.yml`。

---

## 步骤二：配置 GitHub Secret

在目标仓库设置 `NPM_TOKEN` secret。

```bash
# 替换为实际仓库
REPO="weijia/你的项目名"
TOKEN="你的 npm access token"

gh secret set NPM_TOKEN --repo "$REPO" --body "$TOKEN"
```

> npm Access Token 在 [npmjs.com → Access Tokens](https://www.npmjs.com/settings/tokens) 创建，需勾选 **Publish** 权限。

---

## 步骤三：发布

```bash
# 提升版本号并自动打 tag
npm version patch    # 或 minor / major

# push tag 触发 workflow 自动发布
git push --follow-tags
```

或在 GitHub 仓库的 **Actions** 页面手动点击 **Run workflow**。

---

## 新项目快速配置清单

1. **复制 workflow** → 将 `templates/publish.yml` 放入 `.github/workflows/`
2. **设置 secret** → `gh secret set NPM_TOKEN --repo owner/repo --body token`
3. **确保脚本存在** → `package.json` 中有 `build`、`test`、`prepublishOnly`
4. **push tag** → `npm version patch && git push --follow-tags`

---

## 注意事项

- npm 不允许重复发布同一版本号，每次发布前务必提升版本号
- 若包为私有，将 `npm publish --access public` 改为 `npm publish --access restricted`
- 若使用 yarn/pnpm，将 `npm ci` 和 `npm run build` 替换为对应命令
