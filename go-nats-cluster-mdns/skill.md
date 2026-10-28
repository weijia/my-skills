# go-nats-cluster-mdns —— 用"route 版 mDNS + 动态 reload"让 NATS 集群自动组网

## 触发方式

- 想让同一局域网内的多个 NATS 节点**自动互相发现并加入集群**，不再手工给每台写 `cluster.routes`；
- NATS **官方不支持 mDNS 做集群发现**，需要自己补"自动发现新节点 + 动态 reload"这环；
- 节点跑在笔记本/台式机上，会因**休眠唤醒、切换 Wi-Fi 换 IP**，要求 IP 变化后集群仍能自愈；
- Windows 上用**普通 exe 进程**跑 nats-server（非 Windows Service），`--signal reload` 不可用。

## 一句话原理

NATS 集群靠 `cluster.routes`（至少一个 seed）+ **route gossip**：一个 server 连上任意一条 route 后，会学到那个 peer 已知的**所有** server 并自动拨号连全（全 mesh 自动形成）。所以每个节点**只需要**往 config 里注入"到任一发现的 peer 的 route"，不用列全量列表。mDNS 补齐的正是"自动发现 peer"这环。

## 架构

每集群节点并列跑两个进程：

```
nats-server ── cluster.listen 0.0.0.0:6222 ── 主 config 里 include "cluster-routes.conf"
nats-cluster-mdns 守护进程：
  ├─ 广播本机 route：_nats-route._tcp @6222，TXT 带 cluster=<名>（区别于客户端 _nats._tcp）
  └─ 持续 mDNS 发现同 cluster 其他节点 → 写 cluster-routes.conf → nats-server -t 校验
     → 系统账号 reload（$SYS.REQ.SERVER.<id>.RELOAD）
```

端口：mDNS 5353 / route 6222 / 客户端 4222 / http 8222，互不冲突。与 `nats-mdns`/`go-stock-server` 共存：它们广播 `_nats._tcp`（客户端发现），本方案 `_nats-route._tcp`（集群路由），服务类型不同、互不干扰。

## 工作流

1. **广播**：每节点启动即广播 route 端口（TXT 带 cluster 名）。
2. **发现**：持续 QU 查询 `_nats-route._tcp`，收集**非本机**、**cluster 匹配**的地址（`nats-route://<ip>:<route-port>`）。发现端必须用"绑接口 IP + QU"的修复版（见 go-mdns），否则 Windows 收不到远程广播。
3. **写盘**：路由集合变化时重写 `cluster-routes.conf`。
4. **校验**：`nats-server -c <主config> -t` dry-run，失败则保持旧 config，不 reload。
5. **reload**：校验通过后用**系统账号**触发 `$SYS.REQ.SERVER.<server-id>.RELOAD`，节点 in-place 应用新 routes，已有连接不断。

## 动态 reload（Windows 关键）

- `nats-server --signal reload` 只在 server 以 **Windows Service** 方式运行时有效；普通 exe 进程（bat 用 start 直接跑）报 `Access is denied`（Windows 无 Unix 信号机制）。
- 改用**系统账号（System account）**通过 NATS 协议触发，这是官方支持、跨平台统一的运行时热重载：`$SYS.REQ.SERVER.PING` 拿 server_id → `$SYS.REQ.SERVER.<id>.RELOAD`，reply 的 `error` 字段即失败原因。
- 要求 server config 配 `system_account` + 给它一个用户（默认 `sys` / `changeme`）。
- NATS server 需 ≥2.10（支持带 server-id 的 RELOAD subject）。

## 必踩的坑（全部实测钉死）

1. **`include` 路径相对主 config 所在目录解析**。主 config 写 `include "cluster-routes.conf"`（相对路径），routes 文件必须放主 config **同目录**；写绝对路径会拼错 `<dir>\<dir>\file`。
2. **config 账号名不能写 `$SYS`**——NATS 把 `$xxx` 当变量引用，报 `variable reference for 'SYS' ... can not be found`。用 `SYS` + `system_account: SYS`（系统请求 subject 仍固定 `$SYS.REQ.*`，与账号名无关）。
3. **主循环汇总必须用 `time.Ticker`，不能写成 select 里的 `time.After`**：`time.After(o.checkEvery)` 作为 select case 表达式，每次进 select 都重建 timer。当网内有持续 mDNS route 广播（entries channel 频繁命中触发 `continue`）时，循环快速重进 select、timer 永远被重置 → 汇总/写盘/reload 被**饿死**（现象：routez 已有路由，但 routes 文件不更新、守护进程无"路由变化"日志）。用 `time.NewTicker`（跨循环保持）。
4. **广播地址在启动时被固定**（grandcat `Register()` 只枚举一次接口 IP，无 IP 变化重广播）。IP 变化后仍广播旧 IP → 对端收到旧地址、routes 不变、连不上（**死锁**）。必须事件驱动自愈（见下节）。

## IP 变化自适应（Windows 事件，非轮询）

节点休眠唤醒/换网 IP 变化后，NATS 集群 route 的 TCP 连接断开、对端仍按 config 里的旧 IP 重连。mDNS 广播地址是启动时固定的，守护进程必须在 IP 变化后**重新 announce 新地址**。

- 用 `NotifyIpInterfaceChange`（IPHLPAPI，Vista+，`golang.org/x/sys/windows` 封装）：本机 IP 变化时主动通知，**不是轮询**。
- 事件只报"何时变"，不报"变成什么" → 收到事件后**枚举一次**接口地址比对（`interfaceIPv4s()`），确有变化才重建：
  1. 重建广播（重新 announce 新 IP）；
  2. 重建发现端（新 selfIPs + 新 socket，旧 goroutine 用 `done` channel 回收，避免泄漏）；
  3. 清空 discovered，强制下一轮重写 routes 并 reload。
- **Windows 回调运行在系统线程**：回调里只置 atomic 标志，由 goroutine 以 100ms ticker 非阻塞中转，**严禁阻塞/分配/阻塞发 channel**。
- **一次 IP 变化触发多次回调**（Add/Delete/Change）→ 主循环收到事件后 debounce 400ms 合并，避免频繁重建。
- **discovered 地址带 TTL（30s）**：对端停止广播后自动淘汰其旧地址，防止旧 IP 永久残留。
- 依赖升级：`golang.org/x/sys v0.48.0`（`NotifyIpInterfaceChange` / `CancelMibChangeNotify2` / `MibIpInterfaceRow` / `MibNotificationType`；2019 旧版没有）。

> 相比轮询：Windows 没有 netlink，轮询 `net.InterfaceAddrs()` 也可用且跨平台一致，但对"休眠唤醒换 IP"用系统事件更及时、开销更低、且是官方推荐；若需跨平台一致可退回 5s 轮询（见 go-mdns 的 advertise_ipwatch）。

## 模板

- `templates/ipwatch.go`：`NotifyIpInterfaceChange` 事件驱动（回调置标志 + goroutine 中转 + 取消）。
- `templates/reload_sys_account.go`：系统账号 reload 最小 NATS 客户端（`$SYS.REQ.SERVER.<id>.RELOAD`，零第三方依赖）。
- `templates/main_loop_ticker.go`：主循环 ticker + discovered TTL + IP 事件处理骨架。

## 部署

`start-cluster.bat <集群名> [server名] [route端口]`：生成含系统账号的 `nats.conf`（cluster 名 + `include "cluster-routes.conf"` + http 8222）+ 预建 `cluster-routes.conf` + start nats-server + nats-cluster-mdns。每节点运行同一 bat（集群名一致）即自动组网。

关键参数：`-cluster`（必填，各节点一致）、`-route-port` 6222、`-seed`（手工种子，mDNS 不可达时的 fallback）、`-sys-user`/`-sys-pass`、`-check-every` 3s、`-reload-window` 10s、`-dry-run`（只发现不写盘）、`-one-shot`（发现一轮打印）。

## 验证 checklist

- [ ] 单机组件：`nats-cluster-mdns.exe -cluster x -server-name y -one-shot`（发现一轮打印）
- [ ] 多机：各自起同一 `-cluster` → 守护进程日志"路由变化，标记待 reload / 已 reload"
- [ ] 检查路由：`curl http://127.0.0.1:8222/routez`，见互连 route（`did_solicit=True` 表示本机主动拨号）
- [ ] reload 生效：nats-server 日志 `Reloaded: cluster routes / Reloaded server configuration`
- [ ] IP 变化：真实换 IP 后日志出现"IP 变化，已用新地址重新广播 / 发现端已重建"
- [ ] 跨节点收发：A `nats pub` → B `nats sub` 能收到

## 边界

- **同一网段**：mDNS 组播只在同一二层广播域生效；跨网段用 `-seed` 显式种子路由。
- **第一台节点**：无 peer 可发现，作为 seed；routes 文件可为空 `routes: []`。
- **实例名唯一**：同一网络广播同名触发 mDNS 冲突检测重命名，勿多机相同 `-server-name`。
- **cluster 名过滤**：TXT 带 cluster，只加入同名集群；不同集群同网段互不干扰。
- **系统账号密码明文 + `no_auth_user: sys`** 让无认证客户端获得系统权限——**仅限内网/可信环境**，勿暴露公网；生产用 bcrypt 密码并收紧权限。
- **reload 节流**：`-reload-window` 防止路由抖动触发频繁 reload。

## 落地位置（本机）

- `C:\codes\nats-cluster-mdns\`：`mdns_route.go`（广播+自实现发现端）、`config.go`（routes 文件维护+`-t` 校验）、`reload.go`（系统账号 reload）、`ipwatch.go`（IP 事件）、`main.go`（主循环+IP 重建）、`start-cluster.bat`、`README.md`
- `deploy\`：`nats-server.exe` + `nats-cluster-mdns.exe` + `start-cluster.bat`（一键部署三件套）
- 离线构建：`GOFLAGS=-mod=mod GOSUMDB=off GOPROXY=off GOTOOLCHAIN=local go build -o nats-cluster-mdns.exe .`
