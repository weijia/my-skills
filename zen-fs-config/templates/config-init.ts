/**
 * zen-fs-config 初始化模板
 *
 * 复制到项目中，根据实际情况修改 appId 和后端类型。
 *
 * 核心架构：IndexedDB 始终是本地主后端（offline-first），
 * 远程后端（Gitee、S3 等）作为副本，通过 addBackend() 添加后自动双向同步。
 */

import { createConfigRepo, registerBackend } from 'zen-fs-config';

// ---- 1. 注册自定义后端（可选，IndexedDB + InMemory 已内置） ----

// registerBackend('Gitee', async (options) => {
//   const { Gitee } = await import('zen-fs-gitee');
//   return Gitee.create(options);
// });

// registerBackend('S3Bucket', async (options) => {
//   const { S3Bucket } = await import('@zenfs/core');
//   return S3Bucket.create(options);
// });

// ---- 2. 创建并初始化 ConfigRepo ----

// 方式一：零参数初始化（仅 IndexedDB 本地主后端，offline-first）
// 适合首次使用或重新打开应用
const repo = await createConfigRepo('my-app');

// 方式二：带初始副本后端初始化（首次使用时传入远程后端）
// const repo = await createConfigRepo('my-app', {
//   // 副本后端 ID（用于 .meta/backends/ 标识）
//   primaryBackendId: 'gitee-prod',
//
//   // 副本后端连接信息
//   backendInfo: {
//     type: 'Gitee',
//     options: { token: 'your-token', owner: 'your-name', repo: 'config-repo', branch: 'main' },
//   },
//
//   // IndexedDB store 名称（可选，默认 zen-fs-config-{appId}）
//   // idbStoreName: 'my-app-config',
//
//   // 节点 ID（可选，默认自动生成，建议持久化到 localStorage）
//   // nodeId: 'server-1',
//
//   // 同步轮询间隔（可选，默认 30 分钟）
//   // syncPollIntervalMs: 1_800_000,
//
//   // 冲突回调（可选）
//   // onConflict: async (conflict) => {
//   //   console.warn('Config conflict:', conflict.path);
//   //   return null; // 返回 null 使用默认策略，返回合并内容则覆盖
//   // },
// });

// 注意：createConfigRepo 内部会自动调用 load()，无需手动调用。

// ---- 3. 使用配置 ----

// 同步读写（应用私有配置，自动同步到副本后端）
repo.setConfig('/database', { host: 'localhost', port: 5432 });
const db = repo.getConfig<{ host: string; port: number }>('/database');

// 节点本地配置（不自动同步）
await repo.setNodeConfig('node-1', '/debug', { level: 'verbose' });
const debug = await repo.getNodeConfig('node-1', '/debug');

// 发布节点配置到同步后端（调试用）
// await repo.publishNodeConfig('node-1');

// ---- 4. 动态管理副本后端 ----

// 添加副本后端（自动建立双向同步）
// await repo.addBackend('gitee-prod', 'Gitee', {
//   token: 'your-token',
//   owner: 'your-name',
//   repo: 'config-repo',
//   branch: 'main',
// }, '生产环境 Gitee 配置仓库');

// 移除副本后端
// await repo.removeBackend('gitee-prod');

// 查看所有后端拓扑
// const backends = await repo.getBackends();

// ---- 5. 删除文件（带墓碑传播） ----

// 删除文件并写墓碑，确保删除操作传播到所有后端
// await repo.deleteFile('/shared/old-config.json');
// 注意：不要直接用 fs.unlink 删除，否则其他后端会重新同步该文件回来

// ---- 6. 清理 ----

// await repo.dispose();

export { repo };
