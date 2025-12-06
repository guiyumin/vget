# vget

オーディオ、ビデオ、ポッドキャストなどをダウンロードする多機能コマンドラインツール。

[English](README.md) | [简体中文](README_zh.md) | [한국어](README_kr.md) | [Español](README_es.md) | [Français](README_fr.md) | [Deutsch](README_de.md)

## インストール

### macOS

```bash
curl -fsSL https://github.com/guiyumin/vget/releases/latest/download/vget-darwin-arm64 -o vget
chmod +x vget
sudo mv vget /usr/local/bin/
```

### Linux / WSL

```bash
curl -fsSL https://github.com/guiyumin/vget/releases/latest/download/vget-linux-amd64 -o vget
chmod +x vget
sudo mv vget /usr/local/bin/
```

### Windows

[Releases](https://github.com/guiyumin/vget/releases/latest) から `vget-windows-amd64.exe` をダウンロードし、PATH に追加してください。

## コマンド

| コマンド                           | 説明                                  |
|------------------------------------|---------------------------------------|
| `vget [url]`                       | メディアをダウンロード (`-o`, `-q`, `--info`) |
| `vget ls <remote>:<path>`          | リモートディレクトリを一覧表示 (`--json`) |
| `vget init`                        | 対話式設定ウィザード                  |
| `vget update`                      | 自動更新（Mac/Linux は `sudo` が必要）|
| `vget search --podcast <query>`    | ポッドキャスト検索                    |
| `vget completion [shell]`          | シェル補完スクリプトを生成            |
| `vget config show`                 | 設定を表示                            |
| `vget config path`                 | 設定ファイルのパスを表示              |
| `vget config webdav list`          | 設定済み WebDAV サーバー一覧          |
| `vget config webdav add <name>`    | WebDAV サーバーを追加                 |
| `vget config webdav show <name>`   | サーバー詳細を表示                    |
| `vget config webdav delete <name>` | サーバーを削除                        |

### 例

```bash
vget https://twitter.com/user/status/123456789
vget https://www.xiaoyuzhoufm.com/episode/abc123
vget https://example.com/video -o my_video.mp4
vget --info https://example.com/video
vget search --podcast "tech news"
vget pikpak:/path/to/file.mp4              # WebDAV ダウンロード
vget ls pikpak:/Movies                     # リモートディレクトリを一覧表示
```

## 対応ソース

対応サイトの一覧は [sites.md](sites.md) をご覧ください。

### Twitter/X 年齢制限コンテンツ

Twitter/X の年齢制限（NSFW）コンテンツをダウンロードするには、auth token を設定する必要があります：

1. ブラウザで x.com を開いてログイン
2. 開発者ツール（F12）→ Application → Cookies → x.com を開く
3. `auth_token` を見つけて値をコピー
4. 実行：
   ```bash
   vget config twitter set
   # プロンプトで auth_token を貼り付け
   ```

## 設定

設定ファイルの場所：

| OS          | パス                        |
| ----------- | --------------------------- |
| macOS/Linux | `~/.config/vget/config.yml` |
| Windows     | `%APPDATA%\vget\config.yml` |

`vget init` で対話的に設定ファイルを作成するか、手動で作成してください：

```yaml
language: jp # en, zh, jp, kr, es, fr, de
```

## 更新

vget を最新バージョンに更新：

**macOS / Linux:**
```bash
sudo vget update
```

**Windows（管理者として PowerShell を実行）:**
```powershell
vget update
```

## 言語

vget は複数の言語をサポートしています：

- English (en)
- 中文 (zh)
- 日本語 (jp)
- 한국어 (kr)
- Español (es)
- Français (fr)
- Deutsch (de)

## ライセンス

Apache License 2.0
