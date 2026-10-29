---
name: mdns-pairing
description: 局域网零配置发现 + 配对互认（mDNS/DNS-SD 宣告与发现 + 一次性 token 人工审批）。当用户要做「局域网自动发现设备 / 免输 IP 配对 / Android NSD 宣告与发现 / 多端互相同步」,或出现「能看到设备但配对失败」「管理页看不到待审批请求」「点了允许没反应」这类问题时使用。含 Android NsdManager 宣告与发现模板、Node 侧配对协议模板,以及状态口径与审批状态机的踩坑纪律。
---

# 局域网发现 + 配对互认

> 起因：一个 Android 上的 PouchDB/CouchDB 兼容服务端，需要「手机开热点/连同一 WiFi 后，
> 电脑端不用输 IP 就能发现并配对同步」。做完之后踩了三个月的坑，全部是**状态口径**和
> **审批状态机**的问题，而不是 mDNS 不通。

## 适用场景

- 多端（Android / PC / 树莓派）在同一局域网内互相同步，要求零配置
- 不想让用户手抄 IP:端口
- 需要「发现 ≠ 信任」：能看到对方，但必须人工确认才建立凭据
- 已有 HTTP 服务，只想加一层发现 + 配对

## 架构四段

```
①宣告(Advertise)      ②发现(Discover)        ③互认(Pair)            ④同步(Sync)
NsdManager            NsdManager             HTTP + token 审批        各自业务
registerService  →    discoverServices  →    请求/轮询/审批        →   凭据落盘后调用
_couchdb._tcp        resolve + TXT 解析      控制端口(仅本机)          对端数据端口
```

**关键分工**：宣告与发现由**宿主**（Android NsdManager）负责；Node 侧反向不实现 mDNS
（见「坑 1」）。控制端口只给本机用，数据端口才对局域网开放。

## 服务类型与 TXT 契约

服务类型自定，但要稳定且能表达用途：

```java
public static final String SERVICE_TYPE = "_couchdb._tcp";  // 数据端口协议名
```

TXT 字段（**查询方解析后无需再发 HTTP 就能拿到关键信息**）：

| 字段 | 说明 | 必需 |
|---|---|---|
| `instance` | 展示名（缺省用 `Build.MODEL`，如 `Pixel 7`） | 是 |
| `device_id` | 稳定唯一 ID（首次启动生成并持久化） | 是（否则只能靠 instance+ip 当 key） |
| `http_port` | 数据端口 | 是 |
| `version` | 版本号，便于灰度/兼容判断 | 建议 |
| `service` | 服务标识（同一主机多服务时区分） | 建议 |

## Android NSD 硬性纪律

### D1：必须持有 MulticastLock
没有 `WifiManager.MulticastLock`，组播包会被 WiFi 芯片过滤掉，**发现时好时坏**，
表现为「有时能看到、有时看不到」，极难复现。

```java
multicastLock = wifi.createMulticastLock("pouchdb-discovery");
multicastLock.setReferenceCounted(false);
multicastLock.acquire();     // start 时获取
multicastLock.release();     // stop 时释放
```

权限（**都是 normal 权限，安装即授予，不会弹运行时权限框**，别误以为没申请）：
`INTERNET`、`ACCESS_NETWORK_STATE`、`ACCESS_WIFI_STATE`、`CHANGE_WIFI_MULTICAST_STATE`。
还要 `android:usesCleartextTraffic="true"`，否则局域网 HTTP 全部失败。

### D2：不要在 App 内自己实现 mDNS
5353 端口被系统占用、且需要组播锁与原生层支持。**一律用 `NsdManager`**。

### D3：注册失败要区分「名字冲突」与「真失败」
`FAILURE_ALREADY_ACTIVE` 说明**同型号设备撞名**（两台 `Pixel 7` 很常见），
加后缀重试，不要把整个发现功能判死：

```java
if (errorCode == NsdManager.FAILURE_ALREADY_ACTIVE && retryCount < 5) {
    retryCount++;
    instanceName = instanceName + "-" + retryCount;
    register();   // 重试，并上报 state=conflict
    return;
}
notifyState("error", "注册失败 code=" + errorCode);
```

### D4：必须有自愈（周期性重注册）
换网、休眠、WiFi 重连后系统会静默丢弃注册，服务「还在跑但发现不到」。
每 60s 自检一次，未注册就重注册：

```java
if (!registered) register();      // SELF_HEAL_INTERVAL_MS = 60_000
```

发现侧同理：`onServiceLost` 无法精确定位是哪一条（解析后才知 device_id），
直接**停止再开始**一轮发现来刷新；并每 60s 主动 restart 一次，防止发现会话僵死。

### D5：服务名是 UTF-8，不要把非 ASCII 换成 `-`
DNS-SD 的 label 支持 UTF-8，`NsdServiceInfo.setServiceName()` 接受中文。
若用 `replaceAll("[^A-Za-z0-9._-]", "-")` 清洗，中文型号（「小米 14」）会变成 `----`，
对端看到一串横线。正确做法是**只处理会破坏名称结构的字符**：`.`（label 分隔符）
与控制字符；并按 UTF-8 **字节**长度截断到 63 字节（不要按 char 截，会切断多字节字符）。

### D6：`resolveService` 在 API 34+ 已废弃
新代码用 `registerServiceInfoCallback(info, executor, callback)`；
兼容旧机型时保留两条分支，否则高版本上解析永远失败。

### D7：排除自身要按 (IP, 端口) 而不是按名字
自己也会发现自己。用 `ip.equals(localIp) && port == localDataPort` 排除，
不要用 instance 名比较（撞名时会把自己的对端列表清空）。

## 状态口径纪律（本项目最大的坑）

### 坑 1：`--no-mdns` ≠ mDNS 不可用
Node 侧走 nodejs-mobile 时 `bonjour` 之类原生模块装不上，只能 `--no-mdns`，改由
Android NSD 负责。但如果 `/status` 里 `mdns.available` 只判断「服务端有没有 discovery 对象」，
就会返回 `available:false` → 管理页显示「**mDNS 不可用：未知**」→ 用户直接放弃排查。
**而实际上 mDNS 是好的**（能看到节点就是证据）。

正确口径：

```js
// host_managed = 服务端不自己做 mDNS，由宿主 App/NSD 负责 → 不是"不可用"
if (opts.noMdns) {
  return { available: true, mode: 'host',
           note: 'Android NSD（系统服务发现）' };
}
```

### 坑 2：宣告/发现的每个状态都要落日志
`registering / registered / error / conflict / stopped` 全部写进 App 日志页。
否则「能看到节点却配不上」「昨天还能发现今天不行」这类问题**完全无法定位**，
只能靠猜。回调里别传 `null`，传状态字符串。

### 坑 3：把「局域网地址」直接显示给用户
`/status` 返回 `lanIp`，UI 显示 `http://<手机IP>:<dataPort>/`。
用户拿电脑浏览器一开就知道网络层通不通（AP 隔离、不同网段、防火墙），
比让用户猜「配对为什么失败」高效得多。

## 配对互认协议

**发现 ≠ 信任**：发现只是候选，建立凭据必须人工确认。

| 端点 | 端口 | 鉴权 | 作用 |
|---|---|---|---|
| `POST /_pair/request` | 数据端口 | 无（受配对开关控制） | 请求方申请配对，返回一次性 `token` |
| `GET /_pair/status?token=` | 数据端口 | 无 | 请求方轮询：`pending` / `approved`(含 user+pass) / `notfound` |
| `GET /pair/requests` | **控制端口** | 仅本机 | 被请求方拉**待审批列表** |
| `POST /pair/approve` | 控制端口 | 仅本机 | 审批通过，返回 `{ok:true,user,pass}` |
| `POST /pair/deny` | 控制端口 | 仅本机 | 拒绝 |
| `POST /pairing/set` | 控制端口 | 仅本机 | 配对总开关（关闭时 `/_pair/request` 返回 403） |

时序：

```
请求方 A                                     被请求方 B
  │  POST /_pair/request {device_id,name}      │
  │ ─────────────────────────────────────────► │ addPending → token
  │ ◄───────────────────── {token} ─────────── │
  │                                            │  用户在 B 上点「允许」
  │  GET /_pair/status?token=  (每 2s, 共 60s)  │  POST /pair/approve {token}
  │ ─────────────────────────────────────────► │ ──► 生成 peer 账号
  │ ◄──────── {state:approved,user,pass} ───── │
  │  凭据落盘(对端 host:port + user/pass)      │
  │  开始同步                                  │
```

### 状态机纪律（三个都踩过）

**P1：待审列表必须过滤 `state === 'pending'`**
批准后条目**不能立刻删**——请求方还要轮询 `/status` 取明文密码，所以要保留。
但`待审列表`若不过滤状态，用户点「允许」后那一行**还在**，就是典型的
「点了允许没反应」。列表接口要只返回 pending：

```js
getPendingList() {
  return Object.keys(this.pending)
    .filter(t => this.pending[t].state === 'pending' && this.pending[t].type !== 'register')
    ...
}
```

**P2：已处理条目必须短 TTL，且要计入上限**
`PENDING_CAP = 20` 是防刷的。若已批准条目按 1 小时清理，反复配对测试几十次后
**新请求直接被 429（too many pending requests）**，现象还是「配对没反应」。
已处理条目给 5 分钟（够取凭据），待审批的才留 1 小时：

```js
const STALE_MS = 60 * 60 * 1000;         // pending
const HANDLED_KEEP_MS = 5 * 60 * 1000;   // approved / denied
```

**P3：审批接口必须回 `ok`，客户端必须校验它**
很多 HTTP 封装对 4xx **不抛异常**（只是把错误体当正常返回）。若客户端只看
「没抛异常 = 成功」，token 过期/已处理会被当成功，用户看到「已允许」但实际没生效。
服务端返回 `{ok:true}`，客户端显式判断，失败时把 `reason` 显示出来。

### 控制端口安全基线

- 只监听 `127.0.0.1`，并在中间件里校验 `req.socket.remoteAddress` 是 loopback
- CORS 白名单只放 `http://127.0.0.1:<port>` / `localhost`（管理页是 WebView 加载的，需要它）
- 数据端口监听 `0.0.0.0`（对局域网开放），控制端口**绝不**对外

## 网络请求纪律

- **对端请求必须设超时**。管理页每 10s 轮询同步状态，离线对端的 `http.request`
  若无限挂起会持续累积，最终耗尽 fd/内存，事件循环卡死 → 整个服务「假死」。
  3s 超时 + `req.destroy(new Error('timeout'))`。
- 心跳写盘失败不要静默 `catch{}`，至少 `console.error`。
- 心跳「过期」很多时候是**主线程被卡住**的果，不是因。先查有没有挂起请求/同步阻塞。

## 排查清单（按现象）

| 现象 | 先查什么 |
|---|---|
| 能看到节点但配对失败 | 对端 `pairingEnabled`？对端数据端口可达（浏览器开 `http://IP:port/`）？同一 WiFi 但路由器开了 **AP 隔离**？ |
| 管理页看不到待审批 | 列表接口有没有过滤 `state`？控制端口 CORS 白名单？WebView 是否被**混合内容/明文 HTTP** 拦（fetch failed 多半是这个） |
| 点「允许」没反应 | 待审批列表是否过滤 approved；审批响应里的 `ok` 有没有校验；已处理条目 TTL 是否过长把 CAP 占满 |
| 有时能发现有时不能 | MulticastLock；自愈重注册；同一 WiFi 下 AP 隔离/省电策略 |
| 发现到的名字是 `----` | 服务名清洗规则把 UTF-8 换成 `-`（见 D5） |
| 服务在跑但发现不到 | 电池优化（设置→应用→不受限制）；`registered` 状态日志 |

## 验收清单

- [ ] 两台设备同 WiFi：互相在列表中可发现，且**不包含自己**
- [ ] 关掉配对开关：`/_pair/request` 返回 403，页面给出可读原因
- [ ] 发起配对：被请求方 3s 内看到待审批；点「允许」后**该行消失**
- [ ] 请求方拿到 `user/pass` 并落盘，随后同步成功
- [ ] 连续配对 25 次不出现 429（验证 P2 的 TTL）
- [ ] 断开对端电源：管理页仍能响应（验证超时与不挂起）
- [ ] 换 WiFi / 休眠 5 分钟后仍能被发现（验证自愈）

## 模板

| 文件 | 说明 |
|---|---|
| `templates/MdnsAdvertiser.java` | Android NSD 宣告：TXT 契约、撞名重试、60s 自愈、MulticastLock、UTF-8 名称处理 |
| `templates/MdnsDiscovery.java` | Android NSD 发现：resolve + TXT 解析、排除自身、onServiceLost 重扫、60s 重启发现 |
| `templates/pair-store.js` | Node 侧一次性 token 状态机：addPending / approve / deny / status / 待审列表过滤 / CAP 与双 TTL |
| `templates/pair-routes.js` | 数据端口(申请/轮询) + 控制端口(待审/审批/开关) 路由，含 loopback 校验与 CORS 白名单、对端请求超时 |

用法：把两个 `Mdns*.java` 的包名改成目标项目；`pair-*.js` 是参考实现，
按目标项目的 state/存储层接线（token 表建议单独一个模块，别塞进用户表）。
