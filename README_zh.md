# vget

多功能命令行下载工具，支持音频、视频、播客等。

[English](README.md) | [日本語](README_jp.md) | [한국어](README_kr.md) | [Español](README_es.md) | [Français](README_fr.md) | [Deutsch](README_de.md)

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

## 支持的来源

查看 [sites.md](sites.md) 获取完整的支持网站列表。

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
| `vget config path`                 | 显示配置文件路径                      |
| `vget config webdav list`          | 列出已配置的 WebDAV 服务器            |
| `vget config webdav add <name>`    | 添加 WebDAV 服务器                    |
| `vget config webdav show <name>`   | 显示服务器详情                        |
| `vget config webdav delete <name>` | 删除服务器                            |
| `vget telegram login --import-desktop` | 从桌面应用导入 Telegram 会话      |

### 示例

```bash
vget https://twitter.com/user/status/123456789
vget https://www.xiaoyuzhoufm.com/episode/abc123
vget https://www.xiaohongshu.com/explore/abc123  # 小红书视频/图片
vget https://example.com/video -o my_video.mp4
vget --info https://example.com/video
vget search --podcast "科技"
vget pikpak:/path/to/file.mp4              # WebDAV 下载
vget ls pikpak:/Movies                     # 列出远程目录
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
