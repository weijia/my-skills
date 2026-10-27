# go-mdns —— Go 中用 mDNS / DNS-SD 做局域网零配置发现

## 触发方式

- 需要在 **Go** 程序里"自动发现局域网内的某服务/服务器"，不想硬编码 IP；
- 需要让某个服务（NATS、HTTP、数据库……）在局域网里被其他程序自动找到；
- 程序跑在**笔记本/台式机**上，会因**休眠唤醒、切换 Wi-Fi 而换 IP**，要求发现关系在 IP 变化后仍然成立；
- 排查 mDNS 在 Go 里"一开始能发现、过一会儿/换网络后失效"的问题。

> 标准库 **没有** mDNS。Go 里做 mDNS 必须引入第三方库（本项目用
> `github.com/grandcat/zeroconf`）。其它可选：`github.com/brutella/dnssd`、
> `github.com/hashicorp/mdns`（仅响应，不带完整 DNS-SD 浏览）。

## 典型用法（广播端 + 发现端）

```go
import natsdiscovery "github.com/stock-server/nats-discovery"

// 广播端：让本机服务在局域网被发现
adv, _ := natsdiscovery.Advertise(ctx, "my-nats", 4222, []string{"source=whoami"})
defer adv.Shutdown()

// 发现端（一次性）：返回 ["nats://192.168.1.20:4222"]
urls, _ := natsdiscovery.Discover(3 * time.Second)

// 发现端（持续）：地址集合变化时回调（推荐用于长连接客户端）
natsdiscovery.Watch(ctx, func(urls []string) { log.Println("servers:", urls) })
```

## 必须知道的坑

1. **zeroconf 在 `Register()` 时把当时的 IP 写死缓存**，之后所有查询应答都用这个旧 IP
  （`server.go` 的 `appendAddrs` 仅当缓存地址列表为空才回退到实时接口地址）。
   后果：**机器休眠、DHCP 换新 IP 醒来后，广播端会一直用旧 IP 应答 → 对端发现到失效地址，连不上。**
   这正是"休眠唤醒后 mDNS 失效"的根因。

2. **mDNS 依赖局域网组播** `224.0.0.251:5353`。多数家庭/办公 LAN 可用；
   云服务器、部分企业 VPN、跨网段通常**拦截组播**，发现不到。生产多机集群不要用 mDNS 替代
   NATS 的 `-routes` 显式互联。

3. **发现端要区分"一次性"和"持续"**：
   - 一次性 `Discover(timeout)` 只在调用那一刻扫一遍，之后地址变了它不知道；
   - 长连接客户端应改用 `Watch`（持续浏览 + 定时清理过期条目），并在回调里**重定连接地址**。

4. **广播端注册后不会自动重宣告新地址**。zeroconf 的 `probe()` 只在启动时跑一次宣告。
   要让"换 IP 后对新地址生效"，广播端必须自己监听接口地址变化并重注册。

5. **回环地址要跳过**：同机同时跑客户端和服务端时，若把 `127.0.0.1` 当可发现地址，
   客户端会"发现到自己"且跨机不可用。发现端应优先非回环 IPv4；广播端 zeroconf 自己会带全量地址，
   发现端做筛选即可。

6. **重注册/重宣告要带 cache-flush**：重新注册时新 responder 的宣告阶段会给 A/AAAA 记录打
   cache-flush 位，对端据此**立即替换**旧 IP；重注册前对旧 responder 调 `Shutdown()` 会发
   TTL=0 的 goodbye，进一步加速旧条目失效。

## 模式：让 mDNS 在休眠唤醒/换网后仍然可用（本仓库实现）

核心思路两条：

### A. 广播端 —— 轮询接口地址，变化时重广播（核心修复）

完整实现见 `templates/advertise_ipwatch.go`（即 `nats-server/discovery` 的 `Advertise`）。要点：

- `Advertise` 内部起一个 `ipWatch` goroutine，每 `ipPollInterval`（5s）调一次
  `net.InterfaceAddrs()` 拿到"当前所有 up 接口的非回环全局单播 IP"集合；
- 与上次集合比较，**集合变了**才 `reRegister()`：先 `old.Shutdown()`（发 goodbye TTL=0），
  等 250ms 让 goodbye 发出去，再 `zeroconf.Register(...)` 拿新 responder（带 cache-flush 宣告）；
- 用 `sync.Mutex` 保护 `srv` 指针，避免与 `Shutdown()` 并发。

> 为什么用**轮询**而不是 OS 事件？`net.InterfaceAddrs()` 在 Windows/macOS/Linux 都可用且跨平台一致；
> Windows 没有 netlink，事件方案要分平台写。5s 轮询对"休眠唤醒换 IP"这种秒级事件完全够用，开销可忽略。

### B. 发现端（长连接客户端）—— 持续 Watch + 动态重定地址

完整实现见 `templates/discover_watch.go`。要点：

- 客户端启动时用 `Discover` 拿到初始地址；
- 同时起 `Watch` 后台协程，地址集合一变就回调；
- 回调里把"新的备服务器地址"喂给故障转移/连接层（`SetBackupURL`），由它**重连**到新地址；
- 若备是当前 active 后端，重连成功后重新订阅并触发 `onConnect`，业务恢复收消息。

本项目落地位置：

| 角色 | 文件 | 说明 |
|---|---|---|
| 广播端（核心库） | `C:\codes\go\nats-server\discovery\discovery.go` | `Advertise` 含 `ipWatch`/`reRegister`/`currentGlobalAddrs` |
| 广播端（旁路） | `discovery/cmd/nats-advertise` | 配合已有 `nats-server.exe` |
| 广播端（一体化） | `discovery/cmd/nats-mdns` | 内嵌官方 server + 自动广播 |
| 发现端（客户端） | `msg-failover/go/client.go` 的 `SetBackupURL` | 运行时重定备 NATS 地址 |
| 发现端（接线） | `go-stock-server/internal/core/main.go` | `--jetstream-discover` 下启动 `Watch` 并调 `SetBackupURL` |
| 发现端（自实现收包，Windows 上必用） | `discovery/mdns_resolver.go` + `parse.go` | 绑接口 IP + QU 查询 + 手写 DNS 解析（替代 grandcat Resolver，见下文实战节） |

## 验证 checklist

- [ ] `go build ./...` 通过（discovery 库 + 两个 sidecar）
- [ ] `go test ./...` 通过（`sameStringSlices`/`currentGlobalAddrs` 纯逻辑单测）
- [ ] 端到端：启动广播端 + `Discover(3s)` 能拿到 `nats://<LAN_IP>:4222`
- [ ] 休眠唤醒 / 手动换 IP 后，旧地址对应的连接失效、新地址可被 `Watch` 发现并重连
  （无实体机换 IP 环境时，至少确认 `ipWatch` 在地址集合变化时触发 `reRegister` 日志）

## 构建备注（本机环境）

本环境 `sum.golang.org` 被墙，`go mod tidy` 会卡死；依赖已缓存时用离线构建：

```bash
# discovery 库 / sidecar（仅依赖 zeroconf，已缓存）
cd discovery && GOFLAGS=-mod=mod GOSUMDB=off GOPROXY=off go build ./...

# nats-mdns 一体化启动器（内嵌完整 nats-server，需联网拉依赖）
cd discovery/cmd/nats-mdns && GOTOOLCHAIN=local GOFLAGS=-mod=mod GOSUMDB=off GOPROXY=https://goproxy.cn,direct go build -o nats-mdns .
```

## 实战：Windows 上发现端收不到远程 mDNS（2026-10-27 决定性排障）

### 现象
- 局域网另一台机器（`192.168.1.13`）上的 NATS 在广播 mDNS，`python-zeroconf` 能稳定发现；
- 本机 Go 程序（grandcat/zeroconf 的 `Resolver.Browse`，即 `test-nats` 依赖的 `nats-discovery` 发现端）却**一直收不到**，报 `no NATS server discovered via mDNS`；
- 已排除：TCP 直连正常（`-url nats://192.168.1.13:4222` 通）、防火墙已放行、窗口长度（30s/60s 都是 0）。

### 根因（三层，全部用决定性实验钉死，非猜测）
1. **Windows 组播投递只稳定交给"绑具体接口 IP"的 5353 socket**。
   绑定 `0.0.0.0` / `224.0.0.0` 的 socket 收不到网卡组播——因为本机常驻的 `svchost`(Dnscache) 和 `msedge` 已绑 `0.0.0.0:5353` / `:::5353`，抢占了组播投递；只有绑 `具体接口IP:5353` 的 socket 能稳定收到。
2. **远程响应稀疏、且优先回单播** → 必须发 **QU 位（QCLASS=0x8001，单播应答请求）** 才能稳定触发响应。
3. **grandcat/zeroconf 两者都缺**：绑 `224.0.0.0:5353` + 查询不带 QU + 指数退避稀疏重查（4s→8s→16s）→ 必然收不到。

### 决定性对照数据（各自"收到来自远程 .13 的报文数"，多次 30s 窗口）
| socket 配置 | .13 报文数 |
|---|---|
| 绑 `0.0.0.0` + join + 普通查询 | 0~1 |
| 绑 `224.0.0.0`（grandcat 方式）+ 普通查询 | 0~1 |
| 绑具体接口 IP + join + 普通查询 | 0/0/1 |
| **绑具体接口 IP + join + QU 查询** | **8/9/8（稳定）** |
| 绑接口 IP + 普通+QU 轮发（python-zeroconf） | 26/27/25（最稳） |
| 不 join 组播组 | 0（证明响应走组播，必须 join） |

> 关键分水岭 = **绑定地址是具体接口 IP（如 192.168.1.50）而非通配** + **查询带 QU 位**。

### 修复：重写发现端（已落地，不是 workaround）
`nats-discovery` 的发现端已自实现（不再用 grandcat 的 Resolver 收包）：
- 枚举每个 up 非回环接口，对每个全局单播 IPv4 建一个 socket：绑 `ifaceIP:5353` + `SO_REUSEADDR` + **bind 之后**再 `IP_ADD_MEMBERSHIP(224.0.0.251 + ifaceIP)`（bind 前设置无效，实测 0 报文）；
- `1.5s` 周期向组播发 **QU PTR 查询**（QCLASS=0x8001），持续 recv；
- 手写 DNS 解析（PTR/SRV/A），产出 `nats://<ip>:<port>`。

落地文件：
- `discovery/mdns_resolver.go`：socket 建立（`listenOnInterface`）+ 周期 QU 查询（`buildQuery`）+ 读循环（`readLoop`/`serve`）
- `discovery/parse.go`：手写 DNS 解析（`parseMDNS`/`parseName`）
- `discovery/discovery.go`：`browse`/`Watch` 切换到新客户端（**`Discover`/`Watch`/`Advertise` 签名与广播端不变**，生产 go-stock-server 无感）
- 保留的回归测试：`discovery/mdns_test.go`（`TestBrowseMDNS`）

### 解析层一个极易踩的 bug：压缩指针
mDNS 响应里 name 的压缩指针 `0xc0 0xXX` 是**相对整个报文**的绝对偏移。
若把 rdata 切片当独立报文调 `parseName(rdata, 0)`，指针会在切片里自循环 → 无限递归/栈溢出（Go runtime 崩）。
**必须始终基于完整报文 data + 报文偏移解析**：PTR 的 rdata name 用 `parseName(data, rdataOff)`，SRV 的 target 用 `parseName(data, rdataOff+6)`。

### 排障脚本（本机 `C:\codes\` 下可复用）
- `mdns_probe.py`：venv python + zeroconf，双服务诊断（浏览 + 主动查询）。
- `mdns_raw.py` / `mdns_ifbind.py` / `mdns_nonjoin.py`：标准库 socket 对照（绑接口 IP vs 通配、join vs 不 join、普通 vs QU）。
- `mdns_goifip\`（Go）：绑接口 IP ± QU 收包对照——**排障核心工具**（改 `ifaceIP`/`remoteIP` 即用）。
- `build-all.bat`（`C:\codes\build-all.bat`）：一键离线重编译 nats-discovery + nats-mdns + nats-advertise + test-nats + go-stock-server。
- 检测验证：`test-nats.exe`（发现+连接+JetStream 全套，`run-nats-test.bat` 已指向修复版）。

### 通用结论
- **服务类型**（`_nats._tcp`）必须一致（发现匹配键）；**实例名**在同一网络应唯一——mDNS 冲突检测会重命名（`nats-server-2`），勿多机广播同名。
- mDNS 只解决**客户端发现服务**（广播客户端端口 4222）。**NATS 集群节点加入**不靠 mDNS，靠 `cluster.routes`（至少一个 seed route）+ route gossip。
- `nats-mdns` 无参数也会广播（默认端口 4222），但实例名是 nats-server 随机 ServerName；要固定名用 `-n my-nats` 或 `MDNS_NAME`，且避免与网内其他实例同名。
