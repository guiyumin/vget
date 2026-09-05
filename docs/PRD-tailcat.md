# vget 点对点传输 PRD（不需要公网的开库与互传）

> 状态：草稿 · 2026-09-05
> 附属于 [PRD v2](./PRD.md)。范围：让 `vget serve` 不需要公网就能被连上，加上两台机器之间的一次性互传。这是 v2 P1 的完成态，不是新方向；对外表述是"开库不需要公网了"。
> 更大的"圈子 / 网络"设想不在本期，设计草稿留在附录 D。CLI 先行，UI 不动。
> 研究依据：tailcat 源码（v0.6.0，2026-09-04）、README、SECURITY.md、博客、HN 讨论、derper 源码。链接见附录 C。

## 0. 一句话

**两台跑 vget 的机器，不需要公网 IP、域名、证书、端口转发或任何账号，就能互相取文件。**

两种用法，同一个模型：

| 用法 | 对象 | 命令 | 寿命 |
|---|---|---|---|
| **开库** | 我的 NAS、VPS，或给朋友开的库 | `vget serve`，对端 `vget config webdav add name tc...` | 常驻，地址不变 |
| **互传** | 两个人偶尔传一次 | `vget send file` ↔ `vget tc...`；`vget recv` ↔ `vget push` | 一次性，用完即废 |

应用层仍是 WebDAV + Basic Auth（v2 原则 1，不发明协议）。传输层用 tailcat：Tailscale 2026-08 开源的 Go 库，只取 Tailscale 的数据面（WireGuard 加密 + NAT 穿透 + DERP 中继），不要它的控制面（账号、登录、ACL）。服务端 `Start()` 得到一个 `tc...` 地址，客户端拿地址就能拨通一条加密 TCP 连接。

## 1. 为什么做

**v2 卡在"最后一公里"**：v2 P1 是"一条命令开库，10 分钟内可被别人下载"。可 HTTPS 那一步（附录 A 写的 Caddy / Cloudflare Tunnel / Tailscale Funnel）要域名或第三方账号，家庭 NAS 用户没有公网 IP、也不想暴露家庭 IP。这一步占了 10 分钟里的大头，也是最容易放弃的一步。

**tailcat 正好补这一步**（2026-08-28 TailscaleUp 开源，BSD-3）：

- 纯 userspace：userspace WireGuard + gVisor netstack + magicsock（STUN、UDP 打洞）+ DERP（打洞信令 + 兜底中继）。不需要 root、不建 TUN、不改路由表。
- 无控制面：没有账号、没有登录、没有 IP 分配。连接元数据（服务端公钥 + DERP 信息 + 预共享密钥）编码成一个 `tc` 开头的地址字符串，怎么传都行。
- 直连优先：两端通过 DERP 交换端点后打洞，成功即直连；失败回落 DERP 中继。
- 是库不只是 CLI：`github.com/tailscale/tailcat` 导出 `Server`（回调式接 `net.Conn`）和 `Client`（`DialTCPPort` 返回 `net.Conn`），直接能塞进 `net/http`。

**叠加 Tailscale 免费**：应用层是 WebDAV over TCP，已经有 Tailscale（官方或 headscale）、ZeroTier、局域网的人只要 `vget serve --listen :7777`，对端用 IP / MagicDNS 名当普通 WebDAV URL 加即可，不需要 tailcat。

**自用刚需**：笔记本、家里 NAS、海外 VPS 互相取文件、推文件。VPS 有公网 IP 时几乎必定直连。

## 2. 原则（继承 v2，补充）

1. **传输层可插拔，应用层不变**。库协议永远是 WebDAV + Basic Auth。tailcat 只决定"怎么连上"，不决定"连上之后说什么"。把 tailcat 换成 HTTPS / Tailscale / 局域网 IP，vget 的行为完全一样。
2. **不做控制面**。不做账号、设备注册、中转服务器。谁能连我 = 谁有我的地址和账号，就这么简单。
3. **叠加而不是替代**。tailcat 面向"没有任何组网"的默认情况；已有组网的人用 `--listen`。
4. **仍然不是同步器**（v2 原则 4）。互传是人发起、一次一个（或一批）文件的推 / 拉。不 watch、不双向同步、不 mount。
5. **地址即密钥**。tailcat 地址含预共享密钥，是 bearer capability；打印、存储、传递时按密码对待。

## 3. 场景

| 场景 | 要做的事 |
|---|---|
| A 给 B 一个 20 GB 的文件 | A：`vget send big.iso`，把地址发给 B；B：`vget tcXXXX`。打洞后直连，断了能续 |
| B 要把文件塞给 A | A：`vget recv ~/inbox`（只写收件箱，一次性地址）；B：`vget push big.iso tcXXXX` |
| 库主在家没公网，给朋友开库（v2 库主） | `vget serve ~/lib --user a --pass b`，把地址 + 账号发出去；朋友 `vget config webdav add`，之后 `vget ls`、下载照旧 |
| 自用三台机器 | 每台 `vget serve`，互相加 remote；VPS 下好笔记本拉回，笔记本推到 NAS 的收件箱 |
| 已有 Tailscale / headscale | `vget serve --listen :7777 --tailcat=false`，对端加 `http://nas.tail1234.ts.net:7777`。零新代码 |

必须有一端在"听"：常驻的那台跑 `vget serve`（或 vget-server），临时的用 `vget send` / `vget recv`。

## 4. 功能范围（本期）

依赖 v2 P1 的 WebDAV 服务端本身（`golang.org/x/net/webdav` + Basic Auth），两者一起落地。

### 4.1 传输层（`internal/core/tailcat`）

- 身份：`~/.config/vget/tailcat/server.json`（tailcat `PrivateKey` JSON：节点私钥 + PSK + DERP region）和 `client.json`（客户端 node key，供对方 `--allow` 钉住），0600。首次生成时按延迟选最近的 DERP region 并固定写入（等价于 tailcat CLI 的 `--fixed-region`），地址跨重启不变。
- 地址：用库默认的 `TailcatAddr()`，内嵌完整 DERP region（约 140 字节），客户端连接时不需要访问 `tailcat.dev` 的 DERP map。
- Listener 适配（`OnTCP` 回调 → `net.Listener`）和 Dialer 适配（`http.Transport.DialContext` → `Client.DialTCPPort`），见附录 B。
- 中继可配：`--derp host[:port]`（或 `serve.derp` 配置）把自建 DERP 写进身份和地址，客户端从此不接触 Tailscale 任何服务；支持裸 IP + 自签证书（`--derp-cert-sha256`）。中继为什么绕不开、中国场景怎么选，见 §6.6。`vget serve` 启动时打印当前中继及其 RTT。
- DERP map 磁盘缓存 `~/.cache/vget/derpmap.json`（只在用公共中继时用到）。

### 4.2 开库

- `vget serve [dir]`：只读 WebDAV + Basic Auth。默认开 tailcat 监听并打印地址；`--listen host:port` 额外开普通 TCP；`--tailcat=false` 关；`--ephemeral` 用一次性身份。
- `--user/--pass`（或 `serve.user/serve.password` 配置）；`--no-auth` 允许但打印警告。v2 的橱窗、邀请码、302 直链与传输层正交，本文不重复。
- `--rw` 开写；`--inbox <dir>` 在库里开一个只写收件箱（不可列目录、不可覆盖已有文件、服务端决定落盘名，对应 tailcat `recv` 的 drop box 语义）。
- 每次请求重读 `config.yml` 做鉴权（沿用"配置每次重读"规则）；身份和监听在启动时固定。
- 客户端：`vget config webdav add <name> [tc...]` 接受 tailcat 地址（存入 `WebDAVServer.Tailcat`，`URL` 留空）。`vget ls name:/`、`vget name:/path/file`、TUI 浏览、多流下载全部不改，只是拨号走 tailcat。
- `vget push <file> <name>:/dir/`：对常驻库 PUT（目录不存在 MKCOL），要求对端 `--rw` 或 `--inbox`。
- `vget config webdav ping <name>`：DERP 往返、是否直连、直连端点；`--until-direct`。
- `vget identity`：本机地址（打码，`--full` 显示完整）和 client 公钥；`--rotate` 换身份。

### 4.3 互传（一次性）

- `vget send <file...>`：临时开一个只暴露这些文件的只读库，一次性身份，默认免密（地址即凭据，且一次性），`--once` 对端拉完即退，`--user/--pass` 可加密码。
- `vget recv [dir]`：临时开一个只写收件箱，一次性地址。
- `vget <tc...>`：root 命令在走 extractor 之前识别 tailcat 地址（`tc` 前缀 + base64url + `ParseAddr` 成功）。单文件直接下载，多文件进 TUI 选择或 `--all`。
- `vget push <file> <tc...>`：往一次性收件箱推。
- `send` / `recv` / `serve` 是同一个 handler 的三种策略，不是三套代码。

### 4.4 断点续传

- v2 列为"待补"，互传场景大文件多，本期必须落地：下载侧 `.part` + 元数据，`Range` 续传（`x/net/webdav` 原生支持 Range GET）。
- PUT 续传 WebDAV 没有标准，不做；大文件优先用"拉"的方向。

### 4.5 不在本期

圈子（成员名单、`vget circle`）、tsnet 内嵌、远程下载 `--on`、局域网 mDNS 发现、社区中继列表、Web UI。设计草稿和推迟理由见附录 D。

## 5. 非目标

- 不做账号 / 设备注册中心，不运营中继（DERP 用 Tailscale 公共的或用户自建）
- 不做自动同步、mount、watch
- 不做成员名单、发现、圈子（本期）
- 不做移动端；不做浏览器端（tailcat 有 WASM demo，但只走 DERP，且 UI 不动）
- 不做多用户权限（沿用 v2）
- 不重新实现 tailcat 的任何一层，不 fork
- 不改 `ui/`

## 6. 架构与技术方案

### 6.1 分层

```
应用层   WebDAV（+ vget.json / index.json）        不变
认证层   Basic Auth ｜ no-auth（仅一次性模式默认）
传输层   ┌ tailcat   默认：无需任何组网，地址即可连
         └ tcp       --listen：局域网 / 已有 Tailscale·headscale / 反代后 HTTPS
```

- 服务端：一个 `http.Handler`（webdav + auth 中间件 + 只读/rw/inbox 策略），`Serve` 到多个 `net.Listener`。tailcat 的 `OnTCP` 回调用 channel 适配成 `net.Listener`（约 30 行，见附录 B）。
- 客户端：`emersion/go-webdav` 的 `HTTPClient` 换成带自定义 `DialContext` 的 `http.Client`。`tailcat.Client` 按 remote 惰性建立、进程内复用；首次拨号约 1–3 秒（DERP 握手 + 打洞）。
- 同一局域网内两端也会直连局域网地址（disco 会通告本机接口地址），不出公网；但会合仍要经中继，见 §6.6。

### 6.2 包布局

```
internal/core/tailcat/    身份文件读写 · Listener 适配 · Dialer/Transport · ping · DERP map 磁盘缓存   双二进制共用
internal/core/serve/      webdav handler · Basic Auth · 只读/rw/inbox 策略                          v2 P1
internal/cli/serve.go     vget serve · send · recv
internal/cli/push.go      vget push
internal/cli/identity.go  vget identity
internal/cli/config.go    config webdav add 接受 tc 地址 · ping 子命令
internal/cli/root.go      tc 地址识别
```

约束：`tailscale.com` 只从 `internal/core/tailcat` import，其余包只见 `net.Listener` / `net.Conn` / `http.RoundTripper`。tailcat 升级或换掉时只动一个包。vget-server 本期不接 tailcat；要接时复用同一个适配器。

### 6.3 配置

```yaml
webdavServers:
  nas:                      # tailcat remote：tailcat 与 url 二选一
    tailcat: tcomFwWC...
    username: yumin
    password: xxx
  pikpak:                   # 现有形态不变
    url: https://...
serve:
  dir: ~/library
  user: yumin
  password: xxx
  tailcat: true
  listen: ""                # 可选 TCP，如 ":7777"
  rw: false
  inbox: ""
  derp:                     # 自建中继；留空则用 tailcat.dev 的公共中继（都在境外，见 §6.6）
    host: 1.2.3.4           # 域名或裸 IP
    port: 8443              # 默认 443
    stun_port: 3478
    cert_sha256: ""         # 自签证书时填指纹，走 CertName "sha256-raw:"
  allow: []                 # 允许的 client 公钥（可选加固）
```

- `IsRemotePath` / `ParseRemotePath` 不变：remote 仍然是 `name:path`。
- `config set` 新增 `serve.*` 键；`config show` / `config webdav show` 对 `tailcat` 字段打码。
- 身份文件不进 `config.yml`（它是 0644 且 `config show` 会打印）：`~/.config/vget/tailcat/{server,client}.json`，0600；Docker 里落在已挂载的 `/home/vget/.config/vget/`。

### 6.4 tailcat 事实清单（研究结论，写 PRD 时的版本）

| 项 | 结论 |
|---|---|
| 仓库 / 许可 | `github.com/tailscale/tailcat`，BSD-3-Clause；上线一周 6300+ stars |
| 版本节奏 | 2026-08-28 开源；v0.3.0（08-30，分离 disco key）→ v0.4.0（08-31，cp/recv/ls）→ v0.5.0（09-02，修 recv 漏洞、forward）→ v0.6.0（09-04，UDP、ssh 公钥认证、PSK 默认）。一周四个版本 |
| 稳定性声明 | "no API or CLI stability promises: the Go API, the CLI flags and output, and the wire format may all change"；公共 DERP "no uptime SLAs … may revoke access at any time" |
| 威胁模型（SECURITY.md） | "originally designed for use with oneself"；对不信任的一方，谨慎接受地址、谨慎提供可写目录 / shell |
| 构建要求 | `go 1.27.1`；`tailscale.com v1.103.0-pre.0.20260904030409`；wireguard-go、gvisor、cbor。纯 Go，发布包是静态 Linux 二进制（CGO 不需要，darwin / windows 交叉编译待实测） |
| 体积 | v0.6.0 linux/amd64 发布包 6.85 MB（tar.gz）；估计 vget 两个二进制各增重 15–20 MB（待实测） |
| Server API | `Server{Key, PresharedKey, Region/RegionID, DERPMapURL, DERPMapCache, AllowedClients, OnTCP, ServedTCPPorts, Logf}`；`Start() / TailcatAddr() / Addr() / Status() / AddAllowedClient() / DrainTCP() / Close()` |
| Client API | `Client{Server Addr, Key, Logf, DERPMapURL, DERPMapCache}`；`DialTCPPort(ctx, port) / Dial(ctx, net, addr) / Ping() / DiscoPing() / PublicKey() / Close()`；隧道在首次拨号时惰性建立 |
| 身份持久化 | `PrivateKey{Private key.NodePrivate; Public ConnInfo}` 可 JSON 序列化；CLI 存 `~/.config/tailcat/keys/<name>.private.json`，0600 |
| 地址格式 | `"tc" + base64url(CBOR(ServerPublic, ServerDiscoPublic, PresharedKey, RegionID ｜ Region))`；含 region ID 约 60 字节，内嵌完整 region 约 140 字节。库的 `TailcatAddr()` 默认内嵌完整 region。`wire.go` 保留 DERP 节点的 `HostName / IPv4 / IPv6 / DERPPort / STUNPort / CertName` |
| 连接流程 | 客户端经 DERP 发 Meow（带自己的 node key）→ 服务端加入 peer 表回 Meowed → WireGuard 握手（含 PSK）→ 双方经 DERP 交换 STUN 端点 + 本机地址 → disco 打洞 → 直连；失败继续走 DERP |
| 安全 | PSK 默认开：DERP 运营者看到公钥也进不了隧道；disco key 与 node key 分离；`AllowedClients` 钉客户端公钥（不在名单的 Meow 直接忽略）；每客户端的隧道内 IPv6 由其公钥派生（`fd7a:115c:a1e0::/48`），README 明说这是实现细节可能变 |
| 并发 | 一个 Server 接多个客户端；每个 Server / Client 是独立的 magicsock + WireGuard + netstack 实例，进程内可多实例（其 e2e 测试即如此） |
| 限制 | 不压缩；无 per-conn 客户端身份导出 API（只能靠 RemoteAddr 反推，非稳定）；隧道 MTU 按 IPv6 1280 计 |

### 6.5 方案对比

| 方案 | 需要 | NAT 穿透 | 中继 | 评价 |
|---|---|---|---|---|
| **tailcat（选）** | 无 | 有 | Tailscale 公共 DERP 或自建 | 零账号零域名；库 API 不稳，需锁版本 |
| Tailscale Funnel / Cloudflare Tunnel（v2 附录 A） | 账号 + 域名 | 不需要 | 商业 | 公网 HTTPS，给陌生人最方便；依赖第三方账号。继续写在文档里 |
| 只用 tsnet | Tailscale 账号或 headscale | 有 | Tailscale DERP | 适合已有 tailnet 的人；`--listen` 已覆盖，内嵌留作以后 |
| libp2p / pion WebRTC 自研 | 自建信令 + TURN | 有 | 自建 | 重新造 Tailscale，工作量大 |
| 纯 WireGuard | 公网 IP 或端口映射 | 无 | 无 | 不解决核心问题 |
| croc / magic-wormhole 式（PAKE + relay） | 自建 relay | 部分 | 自建 | 只解决一次性互传，没有"库" |

### 6.6 中继（DERP）：作用、为什么绕不开、中国场景

**中继在 tailcat 里干三件事**：会合（客户端经中继给服务端发 Meow，服务端才知道有人要连）、打洞信令（双方经中继交换 STUN 探到的公网端点和本机地址）、兜底转发（打洞失败时所有流量走它）。STUN 服务也跑在中继机上（3478/udp）。

**所以中继绕不开**：tailcat 地址里只有密钥和中继信息，没有 IP 端点；没有控制面替它交换地址。两台机器就算在同一个局域网，也要先各自连上中继才能发现彼此。中继不可达 = 连不上，哪怕对方在隔壁。这是 tailcat 和 Tailscale 共同的设计，不是 vget 能绕的。

**但直连之后中继几乎不走流量**：只剩心跳和 disco 探测，每分钟几 KB。一台最便宜的 VPS 跑 derper 足够；只有打洞失败的那对机器会把文件流量压到它身上。

**tailcat.dev 的公共中继（2026-09-05 抓取 derpmap.json）**：

| Region | 城市 | 节点 |
|---|---|---|
| 301 | New York City | tc301a.ipn.dev |
| 302 | San Francisco | tc302a.ipn.dev |
| 303 | Frankfurt | tc303a.ipn.dev |
| 304 | Tokyo | tc304a.ipn.dev |

全部在境外，离中国最近的是东京；限速、无 SLA、可随时撤。

**中国境内两个用户互传，用公共中继会怎样**：

- 会合和打洞信令要跨 GFW 到东京：TLS 443 通常能通但慢，`ipn.dev` 随时可能被封；STUN 走 UDP 到境外，丢包会让打洞变得不稳。
- 打洞成功后是境内 UDP 直连，和 GFW 无关，速度取决于双方上行。国内家庭宽带 IPv6 普及率高，两端都有 IPv6 时基本直连，没有 NAT 的问题。
- IPv4 常在运营商级 NAT 后面，打洞失败率比海外高；一旦失败，文件流量绕道东京限速中继，基本不可用。

**结论**：境内互传不应该、也不需要依赖 tailcat.dev，但需要一个境内（或近岸）中继。vget 的做法：

1. 中继由服务端决定并写进地址（tailcat 本来的模型），客户端不用配任何东西。`vget serve --derp 1.2.3.4:8443 --derp-cert-sha256 ...` 一次设定，之后地址里就带着。
2. 中继不需要域名：DERP 节点支持裸 IP + 非 443 端口 + 自签证书（`tailcfg.DERPNode` 的 `IPv4` / `DERPPort` / `CertName: "sha256-raw:<指纹>"`，tailcat 的地址编码保留这几个字段）。derper 用 `-a :8443 -certmode manual -hostname <ip> -http-port -1` 起：`-hostname` 是 IP 时它会自动生成自签证书（`cmd/derper/cert.go`），并跳过 SNI 检查。不用 Let's Encrypt，不碰 80/443，不涉及 ICP 备案（境内云厂商对未备案域名只拦 80/443 一类 HTTP 端口，裸 IP 非标端口不受影响）。指纹用 `vget derp test <ip:port>` 连一次打印出来，填进 `--derp-cert-sha256`。
3. 谁来跑：有 VPS 的那一方。家用 NAS 不适合当中继（没有公网入口）。
4. 兜底：断网的局域网走 `--listen` 直连 IP，不依赖任何中继（mDNS 自动发现留作以后）。

**要不要由项目方提供一个国内可达的公共中继**：这和 v2 "不托管、不运营"的原则冲突，而且中继虽然只见密文，也是要花钱、要担责的基础设施。本文建议不运营；见 §11。

## 7. CLI 规格（草案）

| 命令 | 说明 |
|---|---|
| `vget serve [dir]` | 开库。`--user` `--pass` `--no-auth` `--listen :port` `--tailcat=false` `--ephemeral` `--rw` `--inbox dir` `--allow key,...` `--derp host[:port]` `--derp-cert-sha256 hex` |
| `vget send <file...>` | 一次性只读库；`--once` `--user/--pass` |
| `vget recv [dir]` | 一次性只写收件箱 |
| `vget <tc...>` | 从一次性地址拉取；多文件 TUI 选或 `--all` |
| `vget push <file> <tc...｜name:/dir/>` | 推到一次性收件箱 / 常驻库 |
| `vget config webdav add <name> [tc...｜url]` | 加 remote（别名 `remote add`）；`--user` `--pass` 免交互 |
| `vget config webdav ping <name>` | DERP 往返、是否直连、端点；`--until-direct` |
| `vget config webdav show <name>` | 显示传输类型，地址打码 |
| `vget identity` | 本机地址（打码）与 client 公钥；`--full` `--rotate` |
| `vget derp test <host[:port]>` | 中继可达性、RTT，自签证书时打印指纹供 `--derp-cert-sha256` 使用 |

`vget send` 的输出（目标）：

```
$ vget send big.iso
📦 Serving 1 file (4.7 GB), one-time address, exits after the download:
🐈 tcomFwWCCcjS5nKNqAod034nWoJZW0LZqDhhC8U_dKdnDRYQ8uNGFpGQEu
On the other side:
   vget tcomFwWCCcjS5nKNqAod034nWoJZW0LZqDhhC8U_dKdnDRYQ8uNGFpGQEu
```

`vget serve` 的输出（目标）：

```
$ vget serve ~/library --user yumin --pass ****
📚 Serving /Users/yumin/library (read-only)
🐈 tailcat address (treat as a secret):
   tcomFwWCCcjS5nKNqAod034nWoJZW0LZqDhhC8U_dKdnDRYQ8uNGFpGQEu
   relay: 302 San Francisco 180ms · direct connection will be attempted per client

On another machine:
   vget config webdav add mylib tcomFwWCCcjS5nKNqAod034nWoJZW0LZqDhhC8U_dKdnDRYQ8uNGFpGQEu --user yumin
   vget ls mylib:/
```

常驻方式：`vget serve` 前台运行（同 `rclone serve`）。文档给 systemd / launchd 样例。Docker 里的 vget-server 本期不接 tailcat；NAS 用户可以在容器旁边跑一个 `vget serve`。

i18n：新增字符串走 `internal/core/i18n` 的 7 个 locale。

## 8. 安全模型

| 层 | 机制 | 说明 |
|---|---|---|
| 传输 | WireGuard + PSK（地址内） | 知道地址才能建隧道；DERP 运营者看不到明文也进不来 |
| 客户端身份 | `--allow` 钉 client 公钥（可选） | 名单外的 Meow 被忽略，连隧道都建不起来；适合固定的自用设备 |
| 应用 | Basic Auth（默认） | 隧道内明文等价于 HTTPS 下的 Basic Auth；账号可撤可换而不必换地址 |
| 权限 | 只读默认；`--rw` / `--inbox` 显式 | 与 v2 "一库一账号"一致；inbox 不可列、不可覆盖、服务端定名 |
| 泄露处置 | `vget identity --rotate` | 换地址，所有 remote 需重加 |

一次性模式（`send` / `recv`）默认免密是唯一例外：地址一次性、退出即废、只暴露指定文件或一个收件箱。

给不信任的人开库时的建议（写进文档）：只读 + 账号，不开 `--rw`，用 `--allow`。这和 tailcat 自己的 SECURITY.md 口径一致。

## 9. 风险

| 风险 | 说明 | 对策 |
|---|---|---|
| tailcat 不稳定 | 无 API / 线格式稳定承诺；一周四个版本；早期实验性质；Tailscale 说过想把类似能力做进主客户端 | 锁版本；两端同 vget 版本（`vget update` 已有）；`internal/core/tailcat` 隔离，对外只露 Listener / Dialer / 身份三样；地址解析失败提示"请升级 vget"；接受未来一年跟着上游改 |
| Go 工具链 | tailcat 要 go 1.27.1；vget 现在 go 1.25.4，CI、release、Dockerfile 都钉 1.25 | 升级 `go.mod`、`ci.yml`、`release.yml`、Dockerfile（`golang:1.27`）；先单独提一个升级 PR |
| 二进制体积 | tailscale.com 依赖预计 +15–20 MB，两个二进制都受影响；v2 把体积列为成功指标 | 接受并实测写进 `multi-binary-architecture.md`；只在一个包 import；可选 build tag 出精简版 |
| 公共中继全在境外 | tailcat.dev 只有纽约、旧金山、法兰克福、东京四个节点；限速、无 SLA、可撤回；境内用户会合信令跨 GFW，打洞失败时文件流量绕境外 | 中继写进地址，`--derp` 一次配好；文档给裸 IP + 自签证书的 derper 起法（不要域名、不备案）；`serve` 打印中继 RTT（§6.6） |
| 中继绕不开 | tailcat 无控制面，会合只能靠中继；同一局域网也要先连中继；断网局域网连不上 | 说清楚这是 tailcat 的设计；断网局域网走 `--listen` 直连 IP |
| 地址即密钥 | 含 PSK 的 bearer；泄露等于任何人能到达 WebDAV 层 | 默认 Basic Auth；`--no-auth` 显式 + 警告；`--allow`；`config show` 打码；`--rotate` |
| 吞吐 | userspace WireGuard + gVisor，单流几百 Mbps 到 1 Gbps 量级；走 DERP 时更低 | 多流下载已有（12 流）；实测后写文档 |
| 长连接健壮性 | 家用网络切换（Wi-Fi / 4G）后 DERP 重连行为未验证 | serve 端轮询 `Status()`，异常时重建 Server；手测清单 |
| 支持成本 | 联网功能会带来"为什么连不上"类 issue：NAT、运营商、中继、Windows 后台常驻 | `ping` 给出 direct / relay / 中继不可达三种明确诊断；文档先写排障 |
| 身份文件冲突 | 同一台机器两个进程用同一个 key，DERP 只认最后一个连接 | 身份文件加锁；检测到占用时报错并提示 `--ephemeral` |
| 差异化薄 | 一次性互传有 croc、LocalSend、tailcat 自己的 `cp`；库是 commodity | 不单卖任何一层；卖的是"开库不需要公网"这一句，和下载器、库、续传的组合 |

## 10. 成功指标

| 指标 | 目标 |
|---|---|
| 两个人第一次互传（`send` → `vget tc...`）成功耗时 | < 1 分钟，双方都不需要任何账号 |
| 从 `vget serve` 到另一台机器 `vget ls` 成功 | < 2 分钟，无域名、无账号 |
| 直连率（`ping` 报 direct 的比例） | 自用三台 100%；家庭双 NAT 有 DERP 回落提示 |
| 大文件互传断点续传 | > 10 GB 断网续传成功 |
| 二进制增重 | ≤ 20 MB |
| 需求验证 | 发布后看有多少 remote 是 tailcat 地址（用户主动反馈 / issue），决定附录 D 是否推进 |

## 11. 待决问题

1. `vget serve` 默认开 tailcat 还是默认只 TCP？建议默认开，`--tailcat=false` 关。
2. 一次性模式默认免密是否合适？建议是（地址一次性），常驻 `serve` 必须有账号或显式 `--no-auth`。
3. `push` 的 PUT 续传要不要做非标准 `Content-Range`？建议不做，大文件用"拉"的方向。
4. `config webdav` 已有别名 `remote`；要不要把 `vget remote add/ping` 提到顶层并逐步让 `config webdav` 成为别名？
5. 国内中继：项目方要不要运营一个国内可达的公共中继（冷启动用，与 v2 "不运营"冲突），还是只在文档里教自建？建议只教自建。
6. 要不要给 tailcat 上游提 issue：（a）导出"按连接取客户端公钥"的 API（现在只能靠 RemoteAddr 反推，README 明说会变）；（b）地址里可选携带服务端固定公网端点（IP:port），有公网 IP 的 VPS 那一端就能不经中继握手。
7. v2 §10 第 7 条（未知 URL 先 HEAD 探测直链）应一起排期，因为 `vget <tc...>` 会占用 root 命令的分发逻辑。
8. 先做 v2 P1 的 `serve`（纯 TCP）看多少人卡在 HTTPS，再接 tailcat；还是一步到位？建议一步到位，因为自用刚需已经确定，且 `--listen` 路径顺带就有。

## 12. 对 PRD v2 的改动建议

- §4.3 P1 加一行：传输层 = tailcat（默认）/ TCP（`--listen`），详见本文；一次性互传 `send` / `recv` 一并落地。
- §6 架构树加 `internal/core/tailcat`、`internal/core/serve`。
- 附录 A "加 HTTPS" 一段前加：或者不加 HTTPS，`vget serve` 默认给一个 tailcat 地址。
- §10 待决问题并入本文 §11。
- 附录 B 记一行：2026-09 引入 tailcat 作为 `vget serve` 默认传输层，新增一次性互传。

## 附录 A：端到端流程（目标体验）

**互传**：A 给 B 一个文件，B 回 A 一个文件。

```bash
A$ vget send big.iso
# 🐈 tcAAAA...
B$ vget tcAAAA...                       # 打洞直连，断了 vget tcAAAA... 续

A$ vget recv ~/inbox
# 🐈 tcCCCC...
B$ vget push reply.zip tcCCCC...
```

**开库给朋友**：

```bash
alice$ vget serve /volume1/library --user friend --pass 'p1'
# 🐈 tcAAAA...
bob$   vget config webdav add alice tcAAAA... --user friend --pass 'p1'
bob$   vget config webdav ping alice      # direct 12ms via 1.2.3.4:41641
bob$   vget ls alice:/
bob$   vget alice:/movies/x.mkv
```

**自用三台机器**：每台 `vget serve`，互相加 remote。

```bash
vps$ vget serve ~/downloads --user me --pass 'p'                  # 🐈 tcVVVV...
nas$ vget serve /volume1/library --user me --pass 'p' --inbox /volume1/inbox   # 🐈 tcNNNN...
mac$ vget config webdav add vps tcVVVV... --user me --pass 'p'
mac$ vget config webdav add nas tcNNNN... --user me --pass 'p'
mac$ vget vps:/xxx.mp4                    # VPS 下好的拉回来
mac$ vget push ~/Downloads/vget/xxx.mp4 nas:/inbox/
```

**国内两点：自己的中继**（一方有一台境内 VPS，裸 IP，不要域名不备案）：

```bash
vps$ go install tailscale.com/cmd/derper@latest
vps$ derper -a :8443 -certmode manual -hostname 1.2.3.4 -http-port -1 -stun-port 3478
# 防火墙放开 8443/tcp、3478/udp；证书自动自签，落在 certdir 里

alice$ vget derp test 1.2.3.4:8443
# reachable · RTT 18ms · self-signed cert sha256: ab12...
alice$ vget serve /volume1/library --user friend --pass 'p1' --derp 1.2.3.4:8443 --derp-cert-sha256 ab12...
# 地址里带着这台中继；bob 加 remote 时什么都不用配，会合、打洞信令、兜底流量都不碰 tailcat.dev
```

**已有 Tailscale / headscale**：

```bash
nas$ vget serve /volume1/library --user me --pass 'p' --listen :7777 --tailcat=false
mac$ vget config webdav add nas http://nas.tail1234.ts.net:7777 --user me --pass 'p'
```

## 附录 B：tailcat 接入示意（Go，可行性验证用）

服务端：`OnTCP` 回调 → `net.Listener` → `http.Serve`。

```go
// internal/core/tailcat/listener.go
type listener struct {
    conns chan net.Conn
    done  chan struct{}
    addr  net.Addr
}

func (l *listener) Accept() (net.Conn, error) {
    select {
    case c := <-l.conns:
        return c, nil
    case <-l.done:
        return nil, net.ErrClosed
    }
}
func (l *listener) Close() error   { close(l.done); return nil }
func (l *listener) Addr() net.Addr { return l.addr }

func Listen(id *Identity) (net.Listener, tailcat.Addr, error) {
    ln := &listener{conns: make(chan net.Conn), done: make(chan struct{})}
    srv := &tailcat.Server{
        Key:            id.Key.Private,
        PresharedKey:   id.Key.Public.PresharedKey,
        Region:         id.Region, // nil 则由 tailcat 按延迟选；自建中继时是一个只含自家节点的 DERPRegion
        ServedTCPPorts: []filter.PortRange{{First: 80, Last: 80}},
        AllowedClients: id.Allowed,
        Logf:           logger.Discard,
        OnTCP: func(port uint16) func(net.Conn) {
            if port != 80 {
                return nil
            }
            return func(c net.Conn) {
                select {
                case ln.conns <- c:
                case <-ln.done:
                    c.Close()
                }
            }
        },
    }
    if err := srv.Start(); err != nil {
        return nil, "", err
    }
    return ln, srv.TailcatAddr(), nil
}
```

客户端：`DialContext` → `DialTCPPort`，塞给现有 WebDAV client。

```go
// internal/core/tailcat/dialer.go
func HTTPClient(addr tailcat.Addr, clientKey key.NodePrivate) *http.Client {
    cl := tailcat.NewClient(addr)
    cl.Key = clientKey
    cl.Logf = logger.Discard
    return &http.Client{Transport: &http.Transport{
        DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
            return cl.DialTCPPort(ctx, 80)
        },
    }}
}

// internal/core/webdav/client.go：NewClientFromConfig 里分支
if server.Tailcat != "" {
    hc := tailcat.HTTPClient(tailcat.Addr(server.Tailcat), clientKey)
    httpClient = webdav.HTTPClientWithBasicAuth(hc, server.Username, server.Password)
    baseURL = "http://vget.tailcat" // 占位主机名，拨号时忽略
}
```

自建中继的 `DERPRegion`：

```go
&tailcfg.DERPRegion{RegionID: 900, RegionCode: "own", Nodes: []*tailcfg.DERPNode{{
    Name: "own", RegionID: 900,
    HostName: "1.2.3.4", DERPPort: 8443, STUNPort: 3478,
    CertName: "sha256-raw:ab12...", // 自签证书；用域名 + 正规证书时留空
}}}
```

现有 `ls` / 浏览 / 多流下载都通过 `webdav.Client` 或它的 `GetFileURL + GetAuthHeader`；多流下载自己建 `http.Transport` 的地方也要拿同一个 `DialContext`，这是唯一需要改到下载引擎的点。

## 附录 C：参考资料

- tailcat 仓库：https://github.com/tailscale/tailcat （README、SECURITY.md、`tailcat.go`、`wire.go`）
- tailcat Go API：https://pkg.go.dev/github.com/tailscale/tailcat
- Tailscale 博客《Tailcat: An open-source CLI for Tailscale's WireGuard, NAT traversal, and DERP》：https://tailscale.com/blog/tailcat
- HN 讨论（bradfitz 关于 DERP 政策、无锁定的说明）：https://news.ycombinator.com/item?id=49452990
- tailcat releases（版本节奏、二进制体积）：https://github.com/tailscale/tailcat/releases
- tailcat 公共 DERP map：https://tailcat.dev/derpmap.json
- 自建 DERP（derper）：https://github.com/tailscale/tailscale/tree/main/cmd/derper ，证书逻辑 `cmd/derper/cert.go`
- DERP 节点字段（`DERPPort` / `CertName` / `IPv4`）：https://github.com/tailscale/tailscale/blob/main/tailcfg/derpmap.go
- tsnet 文档（附录 D 用）：https://tailscale.com/docs/features/tsnet
- headscale：https://headscale.net

## 附录 D：以后再说（设计草稿留档）

本期只做两点互传。下面这些想法有过讨论，先不做，理由是：单看每一层差异化都薄（一次性互传有 croc、LocalSend、tailcat 自己的 `cp`；圈子有 Syncthing、免费 Tailscale 家庭版），vget 的用户基数撑不起网络效应，而联网功能的支持成本高。等看到有人真的互加 tailcat remote 再推进。

### D.1 圈子（`vget circle`）

圈子 = 一份名单（roster）+ 圈内每人一个库。名单是 JSON，放在 hub（圈里最常开的机器）的库里，路径 `/.vget/circle.json`，用 hub 的账号读。

```json
{
  "circle": "friends",
  "hub": "alice",
  "updated_at": "2026-09-05",
  "derp": { "host": "1.2.3.4", "port": 8443, "cert_sha256": "ab12..." },
  "members": [
    { "name": "alice", "tailcat": "tcAAAA...", "user": "friends", "pass": "..." },
    { "name": "bob",   "tailcat": "tcBBBB...", "user": "friends", "pass": "..." },
    { "name": "carol", "url": "https://carol.example.com/dav", "user": "...", "pass": "..." }
  ]
}
```

- 一圈一账号；成员可以是 tailcat 地址或普通 URL；名单带圈子推荐的中继，`join` 时写进各成员的 `serve.derp`，整个圈子只用自己的中继。
- 命令：`create`（hub 建圈）、`invite`（打印邀请）、`join <hub-addr>`（拉名单导入 remotes，把自己的 `member.json` 投到 hub 的 `/.vget/join/` 收件箱）、`approve`（hub 主合并，默认人工）、`add|rm`、`sync`（刷新 remotes）、`ls`（成员 + 在线状态）。
- 之后就是普通 remote 操作。不做消息、不做全圈搜索（v2 P3 的 `index.json` 聚合到位后自然覆盖）、不做多圈权限。
- 手动替代：每人互加 remote，今天就能用。

### D.2 叠加 tailnet

- 零代码路径已在本期：tailscaled + `--listen`。
- `vget circle from-tailnet`：读 `tailscale status --json`，对每个 peer 探测 7777 端口，生成名单。
- tsnet 内嵌：`vget serve --tsnet --hostname vget-nas [--auth-key tskey-...] [--control-url https://headscale.example.com]`，进程内成为 tailnet 节点，适合 Docker / NAS / 无 root；`--tsnet-auth whois` 让来自 tailnet 的连接免密。`tsnet.Server{Dir, Hostname, AuthKey, ControlURL, Ephemeral, Logf, Port}`；`Listen / Dial / LocalClient().WhoIs`；`ControlURL` 为空读 `TS_CONTROL_URL`，指向 headscale 即可。`tailscale.com` 已因 tailcat 在依赖里，边际成本是编译进 tsnet 和一个状态目录。

### D.3 其他

- 远程下载 `vget <url> --on <name>`：把 URL 交给对端下到它的库再拉回；对端是 vget-server 时复用 `POST /api/download`，vget-server 接 tailcat 监听并把 downloads 目录作为库暴露（v2 P5 最后一条）。
- 局域网 mDNS 发现 + `--listen` 直连，断网局域网唯一可用的路。
- 社区中继列表（只是列表，项目不运营）。
- Web UI：地址二维码、remotes 管理。
