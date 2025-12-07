# vget

오디오, 비디오, 팟캐스트 등을 다운로드하는 다목적 명령줄 도구.

[English](README.md) | [简体中文](README_zh.md) | [日本語](README_jp.md) | [Español](README_es.md) | [Français](README_fr.md) | [Deutsch](README_de.md)

## 설치

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

[Releases](https://github.com/guiyumin/vget/releases/latest)에서 `vget-windows-amd64.zip`을 다운로드하고 압축을 푼 후 PATH에 추가하세요.

## 스크린샷

### 다운로드 진행률

![다운로드 진행률](screenshots/pikpak_download.png)

## 지원 소스

지원 사이트 전체 목록은 [sites.md](sites.md)를 참조하세요.

## 명령어

| 명령어                             | 설명                                  |
|------------------------------------|---------------------------------------|
| `vget [url]`                       | 미디어 다운로드 (`-o`, `-q`, `--info`) |
| `vget ls <remote>:<path>`          | 원격 디렉토리 목록 (`--json`)         |
| `vget init`                        | 대화형 설정 마법사                    |
| `vget update`                      | 자동 업데이트 (Mac/Linux는 `sudo` 필요) |
| `vget search --podcast <query>`    | 팟캐스트 검색                         |
| `vget completion [shell]`          | 쉘 자동완성 스크립트 생성             |
| `vget config show`                 | 설정 표시                             |
| `vget config path`                 | 설정 파일 경로 표시                   |
| `vget config webdav list`          | 설정된 WebDAV 서버 목록               |
| `vget config webdav add <name>`    | WebDAV 서버 추가                      |
| `vget config webdav show <name>`   | 서버 상세 정보 표시                   |
| `vget config webdav delete <name>` | 서버 삭제                             |
| `vget telegram login --import-desktop` | 데스크톱 앱에서 Telegram 세션 가져오기 |

### 예시

```bash
vget https://twitter.com/user/status/123456789
vget https://www.xiaoyuzhoufm.com/episode/abc123
vget https://example.com/video -o my_video.mp4
vget --info https://example.com/video
vget search --podcast "tech news"
vget pikpak:/path/to/file.mp4              # WebDAV 다운로드
vget ls pikpak:/Movies                     # 원격 디렉토리 목록
```

## 설정

설정 파일 위치:

| OS          | 경로                        |
| ----------- | --------------------------- |
| macOS/Linux | `~/.config/vget/config.yml` |
| Windows     | `%APPDATA%\vget\config.yml` |

`vget init`으로 대화형으로 설정 파일을 생성하거나 수동으로 생성하세요:

```yaml
language: kr # en, zh, jp, kr, es, fr, de
```

## 업데이트

vget을 최신 버전으로 업데이트:

**macOS / Linux:**
```bash
sudo vget update
```

**Windows (관리자 권한으로 PowerShell 실행):**
```powershell
vget update
```

## 언어

vget은 여러 언어를 지원합니다:

- English (en)
- 中文 (zh)
- 日本語 (jp)
- 한국어 (kr)
- Español (es)
- Français (fr)
- Deutsch (de)

## 라이선스

Apache License 2.0
