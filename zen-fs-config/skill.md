# Skill: zen-fs-config 配置管理

> 基于 ZenFS 的分布式配置管理，多应用隔离、多后端同步、节点本地配置、冲突安全。

---

## 概述

本 skill 用于在项目中集成 **zen-fs-config** —— 一个基于 ZenFS 虚拟文件系统的分布式配置管理库。

| 能力 | 说明 |
|------|------|
| **应用隔离** | 每个应用只能读写自己的 `/{appId}/` 目录 |
| **共享配置** | `/shared/` 目录支持跨应用双向同步 |
| **节点本地** | `/nodes/{nodeId}/` 不同步，适合调试配置 |
| **多后端同步** | InMemory、IndexedDB、S3 等任意 ZenFS 后端互相同步 |
| **冲突安全** | 双方内容归档到 `.meta/.conflicts/`，永不丢失 |

---

## 前置条件

- Node.js 项目（TypeScript）
- 已安装依赖：`@zenfs/core`、`zen-fs-cache`、`zen-fs-sync`、`zen-fs-config`

```bash
npm install zen-fs-config @zenfs/core zen-fs-cache zen-fs-sync
```

---

## 步骤一：复制模板

将 `templates/` 中的模板文件复制到目标项目。

| 模板 | 说明 |
|------|------|
| `config-init.ts` | 初始化 ConfigRepo 的完整示例 |

---

## 步骤二：在 AI Agent 规则中引用

将以下内容添加到项目的 AI 规则文件（`CLAUDE.md`、`.cursor/rules`、`AGENT.md` 等）中：

```markdown
## 配置管理

本项目使用 zen-fs-config 管理配置。

### 读写应用配置
- 同步读取：`repo.getConfig<T>('/database')` — 返回内存缓存的值，无需 await
- 同步写入：`repo.setConfig('/database', { host: 'localhost' })` — 异步持久化，fire-and-forget
- 路径会自动加 .json 后缀，直接用 `/database` 不需要写 `/database.json`
- 读取前必须先调用 `await repo.load()` 完成初始化

### 目录约定
- `/{appId}/` — 当前应用私有配置，只能读写自己的
- `/shared/` — 跨应用共享配置
- `/nodes/{nodeId}/` — 节点本地配置，不自动同步

### 节点配置
- 写入：`await repo.setNodeConfig('node-1', '/debug', { level: 'verbose' })`
- 读取：`await repo.getNodeConfig('node-1', '/debug')`
- 发布到同步后端（调试用）：`await repo.publishNodeConfig('node-1')`
- 查看其他节点：`await repo.peekNodeConfig('node-2', '/debug')`

### 直接文件操作
- `repo.fs.promises.readFile('/shared/feature-flags.json', 'utf-8')` — 标准 fs API
- 路径已 chroot 隔离，只能访问 /{appId}/ 和 /shared/ 下的文件

### 冲突处理
- 冲突会自动归档到 .meta/.conflicts/，双方内容都保存
- 查看冲突：`await repo.listConflicts()`
- 手动解决：`await repo.resolveConflict(conflictId, mergedData)`

### 注册自定义后端
- `import { registerBackend } from 'zen-fs-config'`
- `registerBackend('S3Bucket', async (options) => S3Bucket.create(options))`
- 内置后端：InMemory

### 生命周期
- 不再需要时调用 `await repo.dispose()` 停止同步并释放资源
```

---

## 步骤三：初始化代码

参考 `templates/config-init.ts`，在项目入口初始化 ConfigRepo：

```typescript
import { createConfigRepo } from 'zen-fs-config';

const repo = await createConfigRepo('my-app', {
  primaryBackendId: 'local-memory',
  backendInfo: {
    type: 'InMemory',
    options: { label: 'my-app-config' },
  },
  cache: { storeType: 'MemoryCacheStore', ttlMs: 60_000 },
});

await repo.load();
```

---

## 核心 API 速查

### 应用配置（同步）

```typescript
// 写入（fire-and-forget，异步持久化）
repo.setConfig('/database', { host: 'localhost', port: 5432 });

// 读取（从内存缓存，同步返回）
const db = repo.getConfig<{ host: string; port: number }>('/database');
```

### 节点本地配置（异步）

```typescript
// 写入（不自动同步）
await repo.setNodeConfig('node-1', '/debug', { level: 'verbose' });

// 读取
const debug = await repo.getNodeConfig('node-1', '/debug');

// 发布到同步后端（调试用）
await repo.publishNodeConfig('node-1');

// 查看其他节点的已发布配置
const other = await repo.peekNodeConfig('node-2', '/debug');
```

### 文件操作

```typescript
// 标准 fs API，chroot 隔离
const content = await repo.fs.promises.readFile('/shared/flags.json', 'utf-8');
await repo.fs.promises.writeFile('/shared/flags.json', JSON.stringify({ darkMode: true }));
```

### 同步与冲突

```typescript
// 手动触发所有同步
const results = await repo.flush();

// 查看同步状态
const statuses = repo.getSyncStatuses();

// 列出冲突
const conflicts = await repo.listConflicts();

// 解决冲突
await repo.resolveConflict(conflict[0].conflictId, mergedData);
```

### 后端注册

```typescript
import { registerBackend } from 'zen-fs-config';

registerBackend('S3Bucket', async (options) => {
  const { S3Bucket } = await import('@zenfs/core');
  return S3Bucket.create(options);
});
```

### 生命周期

```typescript
await repo.dispose();
```

---

## 目录结构约定

```
/
├── {appId}/                # 应用私有配置（单向同步）
│   ├── database.json
│   └── .database.json.version   # sidecar 版本文件
├── shared/                 # 跨应用共享配置（双向同步）
│   ├── feature-flags.json
│   └── .feature-flags.json.version
├── nodes/{nodeId}/        # 节点本地配置（不同步）
│   └── debug.json
└── .meta/
    ├── backends.json      # 后端拓扑（自描述）
    ├── sync-rules.json    # 同步规则
    └── .conflicts/        # 冲突归档
```

每个配置文件自动生成 sidecar `.version` 文件，记录版本号 + SHA-256 哈希。

---

## 注意事项

- `getConfig` 返回内存缓存，`load()` 之前调用会抛错
- `setConfig` 是同步 API 但持久化是异步的，不保证写入后立即可在其他后端读到
- `registerBackend` 必须在 `createConfigRepo` 之前调用
- InMemory 后端数据不持久化，适合测试；生产环境使用 IndexedDB / S3 等持久化后端
- `/nodes/` 和 `/.meta/` 默认不同步（direction: none）
- 没有全局"主后端"，每个实例自己指定 primary，类似 Git 的 remote origin