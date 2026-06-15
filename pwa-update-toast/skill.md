# PWA 自动更新检测 + 用户提示刷新

为 React + Vite 项目添加 PWA 新版本检测，用户可感知并手动刷新更新。

## 适用场景

- 使用 Vite PWA (`vite-plugin-pwa`) 的 Web 应用
- 需要用户知道有新版本，并主动刷新
- 避免 Service Worker 自动更新导致页面突然刷新

## 核心问题

### 1. `generateSW` 模式无法处理 `SKIP_WAITING`

Vite PWA 默认 `generateSW` 模式自动生成的 Service Worker **不监听 `message` 事件**。用户点击"刷新"发送 `SKIP_WAITING` 消息会被忽略，新 Service Worker 永远不会激活。

**解决方案**：使用 `injectManifest` 策略 + 自定义 Service Worker。

### 2. 版本检测时机

Service Worker 在后台检查更新，页面加载后 3 秒开始检测，每 5 分钟轮询一次。用户先看到旧版本，检测到更新后显示提示。

### 3. 构建时间缓存问题

`vite.config.ts` 中的函数在 Vite 进程启动时执行一次，如果部署平台有构建缓存，构建时间不会更新。

**解决方案**：使用 `__APP_BUILD_TIME__` 全局变量，每次构建时重新注入。

## 文件结构

```
project/
├── src/
│   ├── sw.ts                    # 自定义 Service Worker
│   ├── components/
│   │   └── UpdateToast.tsx      # 更新提示组件
│   ├── App.tsx                  # 挂载 UpdateToast
│   └── main.tsx                 # (可选) 手动注册 SW
├── vite.config.ts               # Vite PWA 配置
└── package.json
```

## 配置步骤

### 1. 自定义 Service Worker (`src/sw.ts`)

```typescript
/// <reference lib="WebWorker" />
import { precacheAndRoute, cleanupOutdatedCaches } from 'workbox-precaching'

declare const self: ServiceWorkerGlobalScope

// 预缓存所有构建产物（Vite PWA 自动注入 __WB_MANIFEST）
precacheAndRoute(self.__WB_MANIFEST)

// 清理旧缓存
cleanupOutdatedCaches()

// 监听 SKIP_WAITING 消息 —— 关键！generateSW 模式没有这段代码
self.addEventListener('message', (event) => {
  if (event.data && event.data.type === 'SKIP_WAITING') {
    self.skipWaiting()
  }
})
```

### 2. Vite 配置 (`vite.config.ts`)

```typescript
import { VitePWA } from 'vite-plugin-pwa'

export default defineConfig({
  plugins: [
    VitePWA({
      registerType: 'prompt',           // prompt 模式，不自动更新
      injectRegister: 'auto',           // 自动注入 SW 注册代码
      strategies: 'injectManifest',     // 使用自定义 SW
      srcDir: 'src',
      filename: 'sw.ts',
      injectManifest: {
        injectionPoint: undefined,      // 禁用默认注入点，手动控制
        globPatterns: ['**/*.{js,css,html,ico,png,svg}'],
      },
      manifest: {
        // ... manifest 配置
      }
    })
  ],
  define: {
    '__APP_BUILD_TIME__': JSON.stringify(new Date().toISOString()),
    '__APP_VERSION__': JSON.stringify(pkg.version),
  },
})
```

**关键配置**：
- `registerType: 'prompt'` — 不自动激活新 SW，等待用户操作
- `strategies: 'injectManifest'` — 使用自定义 SW 替代自动生成
- `injectionPoint: undefined` — 禁用默认注入点，避免冲突

### 3. 更新提示组件 (`src/components/UpdateToast.tsx`)

```typescript
import { useState, useEffect } from 'react'
import { RefreshCw } from 'lucide-react'

export default function UpdateToast() {
  const [show, setShow] = useState(false)

  useEffect(() => {
    if (!('serviceWorker' in navigator)) return

    const handleUpdate = (reg: ServiceWorkerRegistration) => {
      reg.addEventListener('updatefound', () => {
        const newWorker = reg.installing
        if (!newWorker) return

        newWorker.addEventListener('statechange', () => {
          // 新 Worker 已安装且等待中
          if (newWorker.state === 'installed' && navigator.serviceWorker.controller) {
            console.log('[PWA] 新版本可用')
            setShow(true)
          }
        })
      })
    }

    // 注册后监听更新
    navigator.serviceWorker.ready.then((reg) => {
      handleUpdate(reg)
      // 3 秒后检查更新
      setTimeout(() => reg.update().catch(console.error), 3000)
    })

    // 每 5 分钟轮询
    const interval = setInterval(() => {
      navigator.serviceWorker.ready.then((reg) => reg.update().catch(console.error))
    }, 5 * 60 * 1000)

    return () => clearInterval(interval)
  }, [])

  const handleRefresh = async () => {
    const reg = await navigator.serviceWorker.ready
    const newWorker = reg.waiting
    if (newWorker) {
      // 发送消息让新 SW 跳过等待 —— 需要自定义 SW 监听此消息
      newWorker.postMessage({ type: 'SKIP_WAITING' })
      // 新 SW 激活后刷新页面
      navigator.serviceWorker.addEventListener('controllerchange', () => {
        window.location.reload()
      })
    }
    setShow(false)
  }

  if (!show) return null

  return (
    <div className="fixed bottom-4 left-1/2 -translate-x-1/2 z-50">
      <div className="flex items-center gap-3 bg-gray-900/95 text-white px-4 py-3 rounded-xl shadow-lg">
        <span className="text-sm">新版本可用</span>
        <button onClick={handleRefresh} className="px-3 py-1.5 bg-white/20 rounded-lg text-sm">
          <RefreshCw className="w-3.5 h-3.5 inline mr-1" />
          点击刷新
        </button>
      </div>
    </div>
  )
}
```

### 4. 挂载组件 (`App.tsx`)

```tsx
import UpdateToast from '@/components/UpdateToast'

export default function App() {
  return (
    <>
      <MainPage />
      <UpdateToast />
    </>
  )
}
```

## 踩坑记录

| 问题 | 原因 | 解决方案 |
|------|------|----------|
| 点击刷新无反应 | `generateSW` 不监听 `SKIP_WAITING` | 改用 `injectManifest` + 自定义 SW |
| 构建时间不变 | Vite define 在进程启动时求值 | 使用 `__APP_BUILD_TIME__` 全局变量 |
| SW 文件后缀不对 | `injectManifest` 输出 `.mjs` | 检查 `dist/sw.js` 是否存在，或配置 `filename: 'sw.ts'` |
| 更新提示不显示 | `registerType: 'autoUpdate'` | 改为 `'prompt'` 模式 |

## 依赖安装

```bash
npm install vite-plugin-pwa workbox-precaching
```

## 验证方法

1. 构建项目：`npm run build`
2. 检查 `dist/sw.js` 是否包含 `self.addEventListener('message', ...)`
3. 部署后，修改代码重新部署
4. 打开页面，等待 3 秒，应看到"新版本可用"提示
5. 点击刷新，页面应加载新版本

## 相关 Skill

- [version-display](../version-display/skill.md) — 构建时注入版本号
