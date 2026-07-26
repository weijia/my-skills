# Skill: zen-fs-config 配置管理

> 基于 ZenFS 的分布式配置管理，以 IndexedDB 为本地主后端（offline-first），远程后端作为副本自动同步。支持应用隔离、共享空间、节点本地配置和冲突安全。

---

## 概述

本 skill 用于在项目中集成 **zen-fs-config** —— 一个基于 ZenFS 虚拟文件系统的分布式配置管理库。

核心设计原则：**IndexedDB 始终是本地主后端**，所有读写操作直接操作 IndexedDB，保证离线可用；用户提供的远程后端（Gitee、S3 等）作为**副本**，通过 `addBackend()` 添加后自动双向同步。

| 能力 | 说明 |
|------|------|
| **Offline-first** | IndexedDB 为主后端，离线可用，重新打开只需 appId |
| **应用隔离** | 每个应用只能读写自己的 `/{appId}/` 目录 |
| **共享配置** | `/shared/` 目录支持跨应用双向同步 |
| **节点本地** | `/nodes/{nodeId}/` 不同步，适合调试配置 |
| **多后端同步** | 动态添加 Gitee、S3、RemoteStorage 等副本后端 |
| **冲突安全** | 双方内容归档到 `.meta/.conflicts/`，永不丢失 |
| **删除传播** | 墓碑机制确保删除操作跨后端传播 |

---

## 前置条件

- Node.js / 浏览器项目（TypeScript）
- 已安装依赖：`@zenfs/core`、`@zenfs/dom`、`zen-fs-sync`、`zen-fs-config`

```bash
npm install zen-fs-config @zenfs/core @zenfs/dom zen-fs-sync
```

> `@zenfs/dom` 提供 IndexedDB 后端（浏览器环境必需）。`zen-fs-cache` 为可选依赖。

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

### 核心架构
- IndexedDB 始终是本地主后端（offline-first），所有读写直接操作 IndexedDB
- 远程后端（Gitee、S3 等）是副本，通过 addBackend() 添加后自动双向同步
- 重新打开应用只需传入 appId，IndexedDB + .meta/backends/ 包含所有状态

### 读写应用配置
- 同步读取：`repo.getConfig<T>('/database')` — 返回内存缓存的值，无需 await
- 同步写入：`repo.setConfig('/database', { host: 'localhost' })` — 异步持久化 + 自动同步
- 路径会自动加 .json 后缀，直接用 `/database` 不需要写 `/database.json`
- 读取前必须先完成初始化（createConfigRepo 内部会自动 load）

### 目录约定
- `/{appId}/` — 当前应用私有配置（双向同步到副本）
- `/shared/` — 跨应用共享配置（双向同步）
- `/nodes/{nodeId}/` — 节点本地配置（不自动同步）
- `/.meta/` — 后端拓扑、墓碑、冲突归档（同步到副本）

### 节点配置
- 写入：`await repo.setNodeConfig('node-1', '/debug', { level: 'verbose' })`
- 读取：`await repo.getNodeConfig('node-1', '/debug')`
- 发布到同步后端（调试用）：`await repo.publishNodeConfig('node-1')`
- 查看其他节点：`await repo.peekNodeConfig('node-2', '/debug')`

### 添加/移除副本后端
- 动态添加：`await repo.addBackend('gitee-prod', 'Gitee', { token, owner, repo, branch }, '描述')`
- 移除：`await repo.removeBackend('gitee-prod')`
- 查看所有后端：`await repo.getBackends()`

### 删除文件（带墓碑传播）
- `await repo.deleteFile('/shared/old-config.json')` — 删除并写墓碑，传播到所有后端
- 不要直接用 fs.unlink，否则其他后端会重新同步回来

### 直接文件操作
- `repo.fs.promises.readFile('/shared/feature-flags.json', 'utf-8')` — 标准 fs API
- 路径已 chroot 隔离，只能访问 /{appId}/ 下的文件
- `repo.rootFS` — 无 chroot 的 fs，可访问 /.meta/、/shared/、/nodes/ 等（管理用）

### 冲突处理
- 冲突会自动归档到 .meta/.conflicts/，双方内容都保存为独立文件
- 查看冲突：`await repo.listConflicts()`
- 读取冲突备份：`await repo.readConflictBackup(conflictId, 'source' | 'target' | 'resolved')`
- 手动解决：`await repo.resolveConflict(conflictId, mergedData)`

### 注册自定义后端
- `import { registerBackend } from 'zen-fs-config'`
- `registerBackend('Gitee', async (options) => Gitee.create(options))`
- 内置后端：IndexedDB（本地主后端）、InMemory（测试用）

### 生命周期
- 不再需要时调用 `await repo.dispose()` 停止同步并释放资源
```

---

## 步骤三：初始化代码

参考 `templates/config-init.ts`，在项目入口初始化 ConfigRepo：

```typescript
import { createConfigRepo } from 'zen-fs-config';

// 零参数初始化 — 仅使用 IndexedDB 本地主后端（offline-first）
const repo = await createConfigRepo('my-app');

// 或带初始副本后端初始化
const repo = await createConfigRepo('my-app', {
  primaryBackendId: 'gitee-prod',
  backendInfo: {
    type: 'Gitee',
    options: { token: 'xxx', owner: 'xxx', repo: 'xxx', branch: 'main' },
  },
  idbStoreName: 'my-app-config', // 可选，默认 zen-fs-config-{appId}
});
```

> `createConfigRepo` 内部会自动调用 `load()`，无需手动调用。

---

## 核心 API 速查

### 初始化

```typescript
import { createConfigRepo } from 'zen-fs-config';

// 方式一：零参数（仅 IndexedDB，offline-first）
const repo = await createConfigRepo('my-app');

// 方式二：带初始副本后端
const repo = await createConfigRepo('my-app', {
  primaryBackendId: 'gitee-prod',        // 副本后端 ID
  backendInfo: {                          // 副本连接信息
    type: 'Gitee',
    options: { token, owner, repo, branch: 'main' },
  },
  idbStoreName: 'my-app-config',          // 可选
  nodeId: 'server-1',                     // 可选，默认自动生成
  syncPollIntervalMs: 1_800_000,          // 可选，默认 30 分钟
  // onConflict: async (conflict) => null, // 可选冲突回调
});

// 重新打开时只需 appId，IndexedDB + .meta/backends/ 自动恢复所有状态
const repo2 = await createConfigRepo('my-app');
```

### 应用配置（同步）

```typescript
// 写入（fire-and-forget，异步持久化 + 自动同步到副本）
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

// 发布到同步后端（调试用，可指定路径）
await repo.publishNodeConfig('node-1');
await repo.publishNodeConfig('node-1', { paths: ['/debug'] });

// 查看其他节点的已发布配置
const other = await repo.peekNodeConfig('node-2', '/debug');
```

### 后端管理

```typescript
// 注册自定义后端类型（必须在 createConfigRepo 之前）
import { registerBackend } from 'zen-fs-config';

registerBackend('Gitee', async (options) => {
  const { Gitee } = await import('zen-fs-gitee');
  return Gitee.create(options);
});

// 动态添加副本后端（自动建立双向同步）
await repo.addBackend('gitee-prod', 'Gitee', {
  token: 'your-token',
  owner: 'your-name',
  repo: 'config-repo',
  branch: 'main',
}, '生产环境 Gitee 配置仓库');

// 移除副本后端
await repo.removeBackend('gitee-prod');

// 查看所有后端拓扑
const backends = await repo.getBackends();
// backends.backends = [{ id: 'local-idb', ... }, { id: 'gitee-prod', ... }]
```

### 删除文件（墓碑传播）

```typescript
// 删除文件并写墓碑，确保删除操作传播到所有后端
await repo.deleteFile('/shared/old-config.json');

// 注意：不要直接用 fs.unlink 删除，否则其他后端会重新同步该文件回来
```

### 文件操作

```typescript
// 标准 fs API，chroot 隔离到 /{appId}/
const content = await repo.fs.promises.readFile('/shared/flags.json', 'utf-8');
await repo.fs.promises.writeFile('/shared/flags.json', JSON.stringify({ darkMode: true }));

// rootFS — 无 chroot，可访问所有目录（管理用）
const metaContent = await repo.rootFS.promises.readFile('/.meta/backends/gitee-prod.json', 'utf-8');
```

### 同步与冲突

```typescript
// 手动触发所有同步（处理墓碑 + 数据同步 + 墓碑确认 + GC）
const results = await repo.flush();

// 查看同步状态
const statuses = repo.getSyncStatuses();

// 同步 .meta/ 到所有副本
await repo.syncMetaToReplicas();

// 列出冲突
const conflicts = await repo.listConflicts();

// 读取冲突备份内容
const sourceContent = await repo.readConflictBackup(conflicts[0].conflictId, 'source');
const targetContent = await repo.readConflictBackup(conflicts[0].conflictId, 'target');

// 解决冲突
await repo.resolveConflict(conflicts[0].conflictId, mergedData);
```

### 生命周期

```typescript
await repo.dispose();
```

---

## 目录结构约定

```
/
├── {appId}/                # 应用私有配置（双向同步到副本）
│   ├── database.json
│   └── .database.json.version   # sidecar 版本文件
├── shared/                 # 跨应用共享配置（双向同步）
│   ├── feature-flags.json
│   └── .feature-flags.json.version
├── nodes/{nodeId}/        # 节点本地配置（不同步）
│   └── debug.json
└── .meta/
    ├── backends/           # 后端拓扑（每个后端一个 JSON 文件）
    │   ├── local-idb.json
    │   ├── gitee-prod.json
    │   └── ...
    ├── .deleted/           # 删除墓碑（跨后端删除传播）
    └── .conflicts/         # 冲突归档（双方内容都保存）
```

每个配置文件（包括 `.meta/` 下的文件）自动生成 sidecar `.version` 文件，记录版本号 + SHA-256 哈希。

### 后端拓扑文件格式

`.meta/backends/{id}.json`（每个后端一个文件，支持原子增删）：

```json
{
  "id": "gitee-prod",
  "type": "Gitee",
  "options": { "token": "...", "owner": "...", "repo": "...", "branch": "main" },
  "description": "生产环境 Gitee 配置仓库"
}
```

> 旧版 `.meta/backends.json`（单文件格式）会在启动时自动迁移为独立文件。

---

## 内置后端

| 后端 | 类型名 | 说明 | 依赖 |
|------|--------|------|------|
| **IndexedDB** | `IndexedDB` | 本地主后端（始终创建，offline-first） | `@zenfs/dom` |
| **InMemory** | `InMemory` | 内存后端，用于测试 | `@zenfs/core` |

注册自定义后端：

```typescript
import { registerBackend } from 'zen-fs-config';

// 注册 Gitee 后端
registerBackend('Gitee', async (options) => {
  const { Gitee } = await import('zen-fs-gitee');
  return Gitee.create(options);
});

// 注册 S3 后端
registerBackend('S3Bucket', async (options) => {
  const { S3Bucket } = await import('@zenfs/core');
  return S3Bucket.create(options);
});
```

---

## 注意事项

- **IndexedDB 是唯一主后端**：所有读写直接操作 IndexedDB，远程后端是副本，不是直接操作目标
- **重新打开零参数**：只需传入 appId，IndexedDB + `.meta/backends/` 包含所有状态
- **`getConfig` 返回内存缓存**：`createConfigRepo` 内部自动 `load()`，无需手动调用
- **`setConfig` 是同步 API 但持久化是异步的**：不保证写入后立即可在副本后端读到
- **`registerBackend` 必须在 `createConfigRepo` 之前调用**
- **删除文件用 `deleteFile()`**：不要直接用 `fs.unlink`，否则其他后端会重新同步该文件
- **`local-idb` 是保留 ID**：不能添加或移除 ID 为 `local-idb` 的后端
- **InMemory 后端数据不持久化**：适合测试；生产环境使用 IndexedDB（浏览器自动创建）
- **`/nodes/` 和 `/.meta/` 默认不参与数据同步**（但 `.meta/` 会通过 `syncMetaToReplicas` 同步）
- **nodeId 由调用方持久化**：不再自动读写 `/nodes/.node-id` 文件，建议存到 localStorage
