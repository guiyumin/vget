# vget 产品需求文档（PRD v2）

> 状态：草稿 · 2026-08-22
> 本文取代 2025-12 的 PRD v1（旧版在 git 历史中，commit `f04ae3a` 之前）。

## 0. 一句话

**vget 是一个资源下载器 + 资源库客户端**：原生抓取 Twitter/X 媒体，用标准 WebDAV 协议连接任意资源库（网盘、NAS、seedbox、别人开的库），并让任何人用一条命令开一个自己的库。

vget 不是爬虫。

## 1. 为什么重新定位

**过去**：一个"什么都能下"的下载器——Twitter、小红书、B 站、YouTube、播客、网盘、BT、Telegram，外加 AI 转写、快递查询、桌面端。

**问题**：

- 爬虫类来源（小红书 / B 站 / YouTube）维护成本极高：追平台接口、伪装浏览器、cookie 逆向，随时会坏，还有封号和法律风险
- 为了这些来源，Docker 镜像背了 Chromium + Python + yt-dlp，CLI 背了浏览器自动化（Rod）
- 功能发散，用户画像说不清，推广没有抓手

**洞察**：推广靠的是**资源本身**，下载器只是入口。资源在"库"里；库要多，只能靠社区；社区要一个统一、开放、不锁定的协议——**WebDAV + HTTP Basic Auth 就是这个协议**，vget 的 WebDAV 客户端已经是这个形态。

**参照**：阅读（legado）的"书源"模式——客户端不托管内容、不内置源，社区自己流通源；作者的法律风险最小化。

## 2. 定位与原则

**定位**：Twitter 原生抓取 + 任意 WebDAV 资源库的浏览 / 挑选 / 下载（队列、进度、历史、Web UI）+ 一条命令开库 + 把磁力链分发给远端 BT 客户端。

**原则**：

1. **标准协议优先**。库 = WebDAV + Basic Auth over HTTPS，不发明私有协议。任何 WebDAV 客户端（Infuse、rclone、Finder）都能连 vget 开的库；vget 只是体验最好的那个。
2. **不做爬虫**。需要浏览器伪装、cookie 逆向、追平台接口的来源一律不做。非 WebDAV 的源（百度 / 夸克 / 阿里 / 115、只给 SFTP 的 seedbox）交给 OpenList 转成 WebDAV，vget 不碰。
3. **不托管内容、不内置源、不经手支付**。vget 是中立的客户端和工具；公开目录只放元数据。
4. **人来挑，不做同步器**。不和 rclone / Syncthing 竞争：不做 mount、不做自动同步。vget 的价值是发现、挑选、排队下载。
5. **合法内容是生存条件**。尤其在"公开目录 + 付费"的组合下，这不是道德选项，是能不能活的问题。

## 3. 目标用户

| 角色 | 画像 | 要解决的事 |
|---|---|---|
| **消费者** | 愿意为网盘 / 库付费、家里有 NAS 或常开机器的人 | 从 Twitter、网盘、朋友的库把资源拉到本地 / NAS；看到有什么可下 |
| **库主** | 有 seedbox / NAS / 网盘，有资源（pdf、video、audio、数据集、anything），想分享或售卖访问权的人 | 一条命令开库；不出流量；发账号自动化；被人看见 |

资源类型不设限：对协议来说都是文件。唯一要认真对待的是体量跨度（几 MB 的 pdf 到几百 GB 的数据集），所以断点续传和队列比格式识别重要。

## 4. 功能范围

### 4.1 保留（现有）

- **Twitter/X**：视频 / 图片，`twitter.auth_token` 解锁 NSFW
- **WebDAV 客户端**：`vget config webdav add|list|show|delete`，`vget ls remote:/path`（TUI 浏览），`vget remote:/path/file` 下载；Web UI 浏览 / 下载
- **下载引擎**：并发、进度、m3u8/HLS、批量（txt）；断点续传待补
- **Docker：`vget-server` + Web UI**：任务队列、历史（SQLite）、配置、WebDAV 浏览、磁力分发、API Token（JWT）
- **Torrent dispatch**：把磁力 / 种子发给 Transmission / qBittorrent / 群晖（待定保留）
- **播客**：小宇宙 / Apple Podcasts 搜索与下载（待定保留）
- i18n 七种语言、自更新、`vget init`、`config set|get|unset`

### 4.2 已砍

- 本次：**小红书**（Rod 浏览器）、**B 站**（extractor + 扫码登录 + Web UI 页）、**YouTube**（yt-dlp，仅 Docker）、**通用浏览器抓取**（`sites.yml` 配置站点 + 未知站点 m3u8 嗅探，Rod + Chromium）；Docker 镜像同步去掉 python / yt-dlp / chromium / CJK 字体，Go 依赖去掉 go-rod
- 本次：**Telegram**（官方 MTProto / gotd，导入 Desktop 会话）——不属于 WebDAV 资源库模型、需登录、gotd 是最重的依赖，且使用者少
- 此前：AI 转写（whisper）、快递查询（kuaidi100）、Tauri 桌面端

### 4.3 新增（路线图，按先后）

**P1 — `vget serve`：一条命令开库**

- 只读 WebDAV + Basic Auth（`golang.org/x/net/webdav`）
- **橱窗模式**：匿名可浏览目录（PROPFIND 放开），下载（GET）要账号——让人先看见有什么
- **邀请码 → 自动发放可撤销账号**：库主卖邀请码 / 送邀请码都行，不用手工建用户
- **网盘 302 直链模式**：库挂在 PikPak / 123 等网盘上时，vget 只做目录 + 鉴权，下载时向网盘取临时直链并 302 过去，库主零流量、速度等于网盘速度
- 自动生成 `index.json`（见 P2）
- 配套文档：HTTPS（Caddy 自动证书 / Cloudflare Tunnel / Tailscale Funnel）、机器选型（VPS / NAS / 网盘 + 302）

**P2 — 库协议**

- `vget.json`（库清单）：名称、简介、标签、维护者、访问方式（公开 / 邀请 / 付费 + 链接）、更新时间
- `index.json`（文件索引）：路径、大小、类型、标题、标签、校验和；由 `vget serve` 自动生成，公开可读
- 规范文档随仓库发布；其他 WebDAV 客户端忽略这两个文件也不受影响

**P3 — `vget lib add|ls|search|rm`**

- 通过链接 / 邀请码 / 二维码 / JSON 一键导入库（legado 书源的体验）
- 跨库搜索：聚合已导入库的 `index.json`
- Web UI 对应页面：库列表、浏览、搜索、下载

**P4 — 公开目录**

- 一个 GitHub 仓库：一库一 JSON（即 `vget.json` 的副本 + 索引地址），PR 提交，社区审核；提交规则第一条是"只收你有权分发的内容"，并配 takedown 流程
- 从仓库生成静态站（GitHub Pages，可搜索）；`vget lib search --public` 读同一份数据
- 目录只有元数据，文件永远在库主自己的机器 / 网盘上

**P5 — 可选**

- SFTP 作为第二协议（纯 Go，`pkg/sftp`），仅当大量 seedbox 用户确实拿不到 WebDAV 时
- rTorrent / Deluge 客户端（seedbox 常用）
- vget-server 把自己的 downloads 目录作为库暴露（每个跑 vget 的 NAS 自动成为可接入的库）

## 5. 非目标

- 任何需要浏览器伪装 / cookie 逆向的站点
- 自动同步、mount（rclone / Syncthing 的事）
- 托管内容、运营中心化文件存储
- 经手支付、做交易平台
- 多用户权限系统（一库一账号或邀请码足够）
- 元数据刮削、媒体库整理（*arr 的事）

## 6. 架构

```
cmd/vget            CLI（goreleaser / Homebrew 发布）
cmd/vget-server     HTTP + 内嵌 Web UI（Docker 发布）
internal/core       config · downloader · extractor(twitter, podcast) · webdav · i18n
internal/server     任务队列 · 历史 · 配置 API · WebDAV 浏览 · 磁力分发 · JWT
internal/torrent    transmission · qbittorrent · synology
ui/                 React Web UI → internal/server/dist（go:embed）
```

- 两个二进制解耦：CLI 不 import server，gin / sqlite / jwt / UI 不进 CLI
- 库客户端：`emersion/go-webdav`（现有）；库服务端：`golang.org/x/net/webdav` + Basic Auth 中间件 + 橱窗模式 + 302 模式
- 分工：**OpenList 负责把非 WebDAV 源变成 WebDAV；vget 只做 WebDAV 客户端与库服务端**
- 配置文件 `~/.config/vget/config.yml` 每次执行都重新读取（Docker 改配置不用重启，必须保持）

## 7. 社区与商业模式

- **供给侧优先**：库主是稀缺资源。开库一条命令；302 让库主零流量；邀请码让发账号自动化；`index.json` 让库被看见
- **钱不经过 vget**：库主在目录里写价格和付款方式（爱发电 / Stripe / 任何），付完发邀请码。vget 只标准化"邀请码 → 账号"这一步
- **公开目录 = 橱窗**：让人看到有什么再决定付费；目录只放元数据
- **冷启动**：第一个库由作者自己开，用合法内容（公版书、课程、播客、数据集、Linux ISO）跑通"开库 → 导入 → 搜索 → 下载"整条链，再开放给社区

## 8. 风险

| 风险 | 说明 | 对策 |
|---|---|---|
| 法律 | 公开目录 + 付费 + 侵权内容是最重的组合（人人影视剧本） | 不托管、不内置、不经手钱；目录提交规则 + takedown；先用合法内容起盘 |
| 中立客户端也会被压 | Tachiyomi 2024 年被 Kakao 法务压停 | 仓库里不出现任何源；目录独立于代码仓库 |
| 冷启动 | 没有库就没有用户 | 作者先开库；把"开库"做到 10 分钟内完成 |
| 差异化 | 裸下载是 commodity（rclone / Infuse / OpenList 都能连 WebDAV） | 赢在发布端（一条命令开库）和消费端（导入、跨库搜索、队列） |
| 网盘直链限制 | 直链临时、部分网盘要会员 / 限速 | 文档说清；302 只是优化，不是依赖 |

## 9. 成功指标

- 公开目录里的库数量；`vget lib add` 导入次数
- 下载成功率、断点续传覆盖率
- "从零到一个可被别人下载的库"耗时 < 10 分钟
- Docker 镜像体积、CLI 二进制体积（砍爬虫后应显著下降）

## 10. 待决问题

1. 播客（小宇宙 / Apple Podcasts）、Torrent dispatch 去留
2. SFTP 要不要做
3. 公开目录形态：GitHub 仓库 + 静态站，还是自托管站
4. 第一个库放什么、放哪（VPS / NAS / 网盘 + 302）
5. 付费方式标准化到什么程度（只留链接 vs 邀请码标准）
6. 过时文档处理：`docs/http-server-mode.md`、`docs/multi-binary-architecture.md` 的发布表
7. 未知 URL 的兜底：现在直接报"无可用解析器"；是否改为先 HEAD 探测直链（Content-Type / Content-Disposition）再下载

## 附录 A：今天就能开一个库（不依赖 vget 新功能）

一个库 = 一台常开的机器 + 一个目录 + 带账号的 WebDAV 服务 + HTTPS。

```bash
# 选项 A：rclone 一条命令（文件在本地目录，或在 rclone 配好的网盘 remote 上）
rclone serve webdav /data/mylib --addr :8080 --user yumin --pass 'xxx'
rclone serve webdav pikpak:/mylib --addr :8080 --user yumin --pass 'xxx'

# 选项 B：OpenList（有界面，能挂多个网盘，自带 302），WebDAV 地址 http://host:5244/dav
docker run -d -p 5244:5244 -v ./data:/opt/openlist/data openlistteam/openlist:latest

# 选项 C：群晖自带 WebDAV Server 套件
```

加 HTTPS：有域名 + VPS 用 Caddy（自动证书）；机器在家不想开端口用 Cloudflare Tunnel 或 Tailscale Funnel。

用 vget 连上：

```bash
vget config webdav add mylib      # URL / 用户名 / 密码
vget ls mylib:/
vget mylib:/path/to/file.pdf
```

库根目录先放一个 `vget.json` 样板（vget 暂不读取，等协议定稿即成为第一个样板库）：

```json
{
  "name": "库名",
  "description": "一句话简介",
  "tags": ["pdf", "dataset"],
  "maintainer": { "name": "Yumin", "contact": "..." },
  "access": { "type": "invite", "url": "怎么拿到账号" },
  "updated_at": "2026-08-22"
}
```

机器选型：网盘 + 302 → 零流量但受制于网盘；家里 NAS → 免费但上行带宽是上限、暴露家庭 IP；VPS（如 Hetzner，每月 20TB 流量）或 seedbox → 最稳，适合给陌生人用。

## 附录 B：功能取舍记录

| 日期 | 动作 | 原因 |
|---|---|---|
| 2026-08 | 砍 AI 转写、kuaidi100、Tauri 桌面端 | 与定位无关 |
| 2026-08 | 砍小红书、B 站、YouTube | 爬虫类来源，维护成本与风险高 |
| 2026-08 | 砍通用浏览器抓取（sites.yml + m3u8 嗅探），Docker 去掉 Chromium | 同上；镜像瘦身、去掉 Rod 依赖 |
| 2026-08 | 砍 Telegram（gotd/MTProto） | 不属于库模型、需登录、最重依赖、用户少 |
| 2026-08 | 重新定位为"资源下载器 + 资源库客户端" | 本文 |
