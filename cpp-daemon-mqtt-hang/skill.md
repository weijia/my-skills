# Skill: cpp-daemon MQTT 挂起修复（退出后进程不退 / 托盘图标残留）

> 解决基于 paho-mqtt-cpp 的 cpp-daemon 各插件（iot_report / mqtt / reporter / scheduler）线程优雅退出的问题：
> 托盘右键「退出」后**托盘图标一直残留、进程不退出**。适用于任何「MQTT 异步客户端 + 后台线程 + 托盘退出」的组合。

---

## 问题现象

- 托盘右键「退出」后，**托盘图标一直还在，进程不退**（`Get-Process cpp-daemon` 仍有进程，HTTP 端口仍在监听）。
- 表现为「点了退出没反应 / 退不掉」。

## 退出链路（先理解卡点在哪）

cpp-daemon 的退出顺序（`App::quit()`，在独立线程执行）：

```
App::quit()
 └─ plugins_->shutdownAll()   ← 逐个调用插件 shutdown()：quit_=true + thread_.join()
     ├─ 只要任一插件线程卡在「不受 quit_ 影响的操作」上，join() 就永久卡死
     ├─ shutdownAll() 卡住 → 后面 http_->shutdown()、tray_->quit()(NIM_DELETE) 永不执行
     └─ 结果：托盘图标不删、进程退不出
 └─ http_->shutdown()
 └─ tray_->quit()  → 投递 WM_APP+1 结束消息循环
```

**关键**：任何插件线程在退出时若被一个不响应 `quit_` 标志的操作阻塞（如 MQTT 的 `connect()->wait()`、`disconnect()->wait()`、`publish()->wait()`，或 HTTP 上传），`shutdown()` 里的 `thread_.join()` 就会无限期卡住。

## 本次实际根因（三处）

1. **paho 的 wait() 无超时**：`mqtt::async_client` 的 `connect()` / `disconnect()` / `publish()` / `subscribe()` 返回 token，`.wait()` 若不带超时，遇到 broker 不可达 / 网络异常 / 握手卡住会**无限阻塞**，线程无法退出。
2. **双重 disconnect 竞态**：`shutdown()` 与 `loop()` 都对 `cli_` 调用 `disconnect()`；在**持锁状态下调用 `disconnect()->wait()`** 会与 paho 内部网络线程争锁 → `join()` 卡死。
3. **reporter 的 HTTP 上传无超时**：httplib 默认可能长时间挂起，阻塞上报线程退出。

## 修复方案

### 1. 所有阻塞操作一律给有限超时

| 操作 | 建议超时 | 说明 |
|------|---------|------|
| `connect()->wait(timeout)` | 15 s | 连接/握手 |
| `disconnect()->wait(timeout)` | 5 s | 断开 |
| `subscribe()->wait(timeout)` | 10 s | 订阅 |
| `publish()` | 异步，不给 wait | 发布走回调，不阻塞 |
| HTTP 上传（httplib） | 10~15 s | connect/read/write 分别设 |

### 2. disconnect 锁外 wait，锁内只 reset

`loop()` 收尾与 `shutdown()` 都必须：**先释放 `cliMu_`，再调用 `disconnect()->wait()`**；拿到锁后只做 `cli_.reset()`。绝不能在持锁下 `wait()`（会与 paho 网络线程争锁）。

### 3. quit_ 标志要能打断循环

发布循环 / 连接重试循环都要检查 `quit_`；用 `cv_.wait_for(…, quit_)` 而非裸 `wait()`，保证退出能及时唤醒。

## 关键代码模式

见 [`templates/mqtt-timeout-reference.cpp`](./templates/mqtt-timeout-reference.cpp) —— 可直接套用的连接/断开/订阅带超时写法。

## 验证方法

1. 启动最新版 daemon（会连上 MQTT 并开始上报）。
2. **走真实托盘右键「退出」**（不要用内部触发冒充——GUI 自动化在 Win11 通知区常被隔离跑不通，需人工/脚本配合）。
3. 预期：2~5 s 内进程消失（`Get-Process cpp-daemon` 为空）、托盘图标随之删除。
4. 连测 ≥3 次。

> 附一个辅助脚本思路（`tray_quit_verify.ps1`）：启动 daemon → 提示人工右键退出 → 轮询进程是否消失并报告。

## 复盘清单（通用，任何 paho/MQTT 集成都适用）

- [ ] 每个网络操作（connect/subscribe/disconnect/upload）都带**有限超时**，绝无裸 `wait()`。
- [ ] `disconnect()->wait()` 在**锁外**调用；持锁只做 reset。
- [ ] 线程循环以 `quit_` + `cv_.wait_for` 可中断，不被任何长操作阻塞。
- [ ] `shutdown()` 里 `join()` 前先置 `quit_` 并 `notify_all()`。
- [ ] 退出是否真正走到 `tray_->quit()`（NIM_DELETE）——用**真实托盘右键**验证，不轻信内部触发。
- [ ] 进程退出后 `Get-Process` 为空、监听端口释放。

---
