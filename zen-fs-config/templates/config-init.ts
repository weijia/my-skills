/**
 * zen-fs-config 初始化模板
 *
 * 复制到项目中，根据实际情况修改 appId、后端类型和同步规则。
 */

import { createConfigRepo, registerBackend } from 'zen-fs-config';

// ---- 1. 注册自定义后端（可选，InMemory 已内置） ----

// registerBackend('S3Bucket', async (options) => {
//   const { S3Bucket } = await import('@zenfs/core');
//   return S3Bucket.create(options);
// });

// ---- 2. 创建并初始化 ConfigRepo ----

const repo = await createConfigRepo('my-app', {
  // 当前实例的主后端 ID
  primaryBackendId: 'local-memory',

  // 主后端连接信息
  backendInfo: {
    type: 'InMemory',
    options: { label: 'my-app-config' },
  },

  // 缓存配置
  cache: {
    storeType: 'MemoryCacheStore', // 或 'IdbCacheStore'（浏览器持久化）
    ttlMs: 60_000, // 60 秒内不重新验证
  },

  // 首次初始化时的引导数据（.meta/ 不存在时写入）
  bootstrap: {
    backends: [
      { id: 'local-memory', type: 'InMemory', options: { label: 'primary' } },
      // { id: 's3-backup', type: 'S3Bucket', options: { bucket: 'my-configs' } },
    ],
    syncRules: [
      {
        prefix: '/my-app/',
        direction: 'one-way',
        conflictStrategy: 'source-wins',
        replicas: ['local-memory'],
      },
      {
        prefix: '/shared/',
        direction: 'bi-directional',
        conflictStrategy: 'merge',
        replicas: ['local-memory'],
      },
      { prefix: '/nodes/', direction: 'none' },
      { prefix: '/.meta/', direction: 'none' },
    ],
  },

  // 节点 ID（可选，默认自动生成或读取 NODE_ID 环境变量）
  // nodeId: 'server-1',

  // 冲突回调（可选）
  // onConflict: async (conflict) => {
  //   console.warn('Config conflict:', conflict.path);
  //   return null; // 返回 null 使用默认策略，返回合并内容则覆盖
  // },
});

// ---- 3. 加载配置 ----

await repo.load();

// ---- 4. 使用配置 ----

// 同步读写（应用私有配置）
repo.setConfig('/database', { host: 'localhost', port: 5432 });
const db = repo.getConfig<{ host: string; port: number }>('/database');

// 节点本地配置
await repo.setNodeConfig('node-1', '/debug', { level: 'verbose' });
const debug = await repo.getNodeConfig('node-1', '/debug');

// 发布节点配置到同步后端
// await repo.publishNodeConfig('node-1');

// ---- 5. 清理 ----

// await repo.dispose();

export { repo };