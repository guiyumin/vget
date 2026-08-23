# vget

一个小而专注的下载器，面向媒体链接和 WebDAV 资源库。提供 CLI 和 Docker 两种方式。

[English](README.md) | [日本語](README_jp.md) | [한국어](README_kr.md) | [Español](README_es.md) | [Français](README_fr.md) | [Deutsch](README_de.md) | [Português](README_pt.md)

## 这是什么

vget 是资源之上的一层轻接口。给它一个链接或一个 WebDAV 资源库，它就把文件拉下来——终端里有进度条，NAS 上有 Web 界面。

工具本身刻意保持简单。真正的价值不是下载器，而是它连接的那些资源库。所以 vget 走标准协议而不是爬虫——不会因为网站改版而失效，任何人搭的 WebDAV 服务都能成为它可浏览、可下载的库。

- **媒体链接** —— Twitter/X（视频/图片）、播客（小宇宙、Apple Podcasts）、直链文件、m3u8/HLS 流
- **WebDAV 资源库** —— PikPak、NAS、seedbox，或任何人分享的 WebDAV 库：浏览、挑选、下载
- **没有原生 WebDAV 的网盘**（百度、夸克、阿里云盘、115……）—— 用 [OpenList](https://github.com/OpenListTeam/OpenList)/Alist 转成 WebDAV，再把 vget 指向那个地址

> 产品方向与路线图（一条命令开库、库客户端、公开目录）见 [docs/PRD.md](docs/PRD.md)。

## 安装

### macOS

```bash
curl -fsSL https://github.com/guiyumin/vget/releases/latest/download/vget-darwin-arm64.zip -o vget.zip
unzip vget.zip
sudo mv vget /usr/local/bin/
rm vget.zip
```

### Linux / WSL

```bash
curl -fsSL https://github.com/guiyumin/vget/releases/latest/download/vget-linux-amd64.zip -o vget.zip
unzip vget.zip
sudo mv vget /usr/local/bin/
rm vget.zip
```

### Windows

从 [Releases](https://github.com/guiyumin/vget/releases/latest) 下载 `vget-windows-amd64.zip`，解压后添加到系统 PATH。

## 截图

### 下载进度

![下载进度](screenshots/pikpak_download.png)

### Docker 服务器界面

![](screenshots/vget_server_ui.png)

## Docker

Docker 镜像提供 Web 界面 + HTTP API——适合放在 NAS 上，在任意设备贴链接或浏览 WebDAV 资源库。

```bash
docker run -d -p 8080:8080 -v ~/downloads:/home/vget/downloads ghcr.io/guiyumin/vget:latest
```

## 支持的来源

原生解析器见 [sites.md](sites.md)。除此之外，任何 WebDAV 服务开箱即用。

## 命令

| 命令                               | 描述                                  |
|------------------------------------|---------------------------------------|
| `vget [url]`                       | 下载媒体 (`-o`, `-q`, `--info`)       |
| `vget ls <remote>:<path>`          | 列出远程目录 (`--json`)               |
| `vget init`                        | 交互式配置向导                        |
| `vget update`                      | 自动更新（Mac/Linux 需使用 `sudo`）   |
| `vget search --podcast <query>`    | 搜索播客                              |
| `vget completion [shell]`          | 生成 shell 补全脚本                   |
| `vget config show`                 | 显示配置                              |
| `vget config set <key> <value>`    | 设置配置值（非交互式）                |
| `vget config get <key>`            | 获取配置值                            |
| `vget config path`                 | 显示配置文件路径                      |
| `vget config webdav list`          | 列出已配置的 WebDAV 服务器            |
| `vget config webdav add <name>`    | 添加 WebDAV 服务器                    |
| `vget config webdav show <name>`   | 显示服务器详情                        |
| `vget config webdav delete <name>` | 删除服务器                            |

### 示例

```bash
# 媒体链接
vget https://twitter.com/user/status/123456789
vget https://www.xiaoyuzhoufm.com/episode/abc123
vget https://example.com/video.mp4 -o my_video.mp4
vget --info https://example.com/video.mp4
vget search --podcast "科技"

# WebDAV 资源库
vget config webdav add pikpak       # 添加一个库（地址 + 用户名 + 密码）
vget ls pikpak:/Movies              # 浏览
vget pikpak:/path/to/file.mp4       # 下载
```

## 配置

配置文件位置：

| 操作系统    | 路径                        |
| ----------- | --------------------------- |
| macOS/Linux | `~/.config/vget/config.yml` |
| Windows     | `%APPDATA%\vget\config.yml` |

运行 `vget init` 交互式创建配置文件，或手动创建：

```yaml
language: zh # en, zh, jp, kr, es, fr, de
```

**注意：** 配置文件在每次命令执行时重新读取，修改后无需重启（适用于 Docker）。

## 更新

将 vget 更新到最新版本：

**macOS / Linux:**
```bash
sudo vget update
```

**Windows（以管理员身份运行 PowerShell）:**
```powershell
vget update
```

## 语言

vget 支持多种语言：

- English (en)
- 中文 (zh)
- 日本語 (jp)
- 한국어 (kr)
- Español (es)
- Français (fr)
- Deutsch (de)

## 代理 / 翻墙

如果你需要翻墙（绕过 GFW），推荐使用 Clash。

**Clash 有两种模式：**

1. **系统代理模式** - 设置系统级 HTTP/HTTPS 代理。支持系统代理的应用会自动使用。
2. **TUN 模式** - 创建虚拟网卡，在网络层捕获所有流量。

**推荐使用 TUN 模式**：开启后，所有应用的流量都会自动经过 Clash，无需任何配置。vget 会自动走代理，无需额外设置。

**如果使用系统代理模式**：Clash 会设置 `HTTP_PROXY` / `HTTPS_PROXY` 环境变量，vget 会自动读取并使用这些代理设置。

简而言之：**只要 Clash 正常运行，vget 就能正常工作**，无需在 vget 中配置代理。

## 许可证

Apache License 2.0
