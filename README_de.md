# vget

Ein kleiner, fokussierter Downloader für Medien-Links und WebDAV-Ressourcenbibliotheken. Verfügbar als CLI und Docker.

[English](README.md) | [简体中文](README_zh.md) | [日本語](README_jp.md) | [한국어](README_kr.md) | [Español](README_es.md) | [Français](README_fr.md) | [Português](README_pt.md)

## Was es ist

vget ist eine dünne, einfache Schnittstelle zu den Ressourcen dahinter. Richte es auf einen Link oder eine WebDAV-Ressourcenbibliothek, und es lädt die Datei herunter — mit einem Fortschrittsbalken im Terminal oder einer Web-Oberfläche auf deinem NAS.

Das Werkzeug bleibt bewusst klein. Der Wert ist nicht der Downloader, sondern die Ressourcenbibliotheken, mit denen es sich verbindet. Deshalb spricht vget Standardprotokolle statt zu scrapen — es geht nicht kaputt, wenn eine Website ihr Markup ändert, und jeder WebDAV-Server, den jemand betreibt, wird zu einer Bibliothek, die es durchsuchen und herunterladen kann.

- **Medien-Links** — Twitter/X (Video/Bild), Podcasts (Xiaoyuzhou, Apple Podcasts), direkte Datei-URLs, m3u8/HLS-Streams
- **WebDAV-Ressourcenbibliotheken** — PikPak, ein NAS, eine Seedbox oder die von jemandem geteilte WebDAV-Bibliothek: durchsuchen, auswählen, herunterladen
- **Cloud-Speicher ohne natives WebDAV** (Baidu, Quark, Aliyun, 115, …) — binde sie über [OpenList](https://github.com/OpenListTeam/OpenList)/Alist ein und richte vget auf den WebDAV-Endpunkt

> Ausrichtung und Roadmap (Bibliothek mit einem Befehl hosten, ein Bibliotheks-Client, ein öffentliches Verzeichnis) stehen in [docs/PRD.md](docs/PRD.md).

## Installation

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

Laden Sie `vget-windows-amd64.zip` von [Releases](https://github.com/guiyumin/vget/releases/latest) herunter, entpacken Sie es und fügen Sie es zum PATH hinzu.

## Screenshots

### Download-Fortschritt

![Download-Fortschritt](screenshots/pikpak_download.png)

### Docker Server-Benutzeroberfläche

![](screenshots/vget_server_ui.png)

## Docker

```bash
docker run -d -p 8080:8080 -v ~/downloads:/home/vget/downloads ghcr.io/guiyumin/vget:latest
```

## Unterstützte Quellen

Siehe [sites.md](sites.md) für die vollständige Liste der unterstützten Seiten.

## Befehle

| Befehl                             | Beschreibung                          |
|------------------------------------|---------------------------------------|
| `vget [url]`                       | Medien herunterladen (`-o`, `-q`, `--info`) |
| `vget ls <remote>:<path>`          | Remote-Verzeichnis auflisten (`--json`) |
| `vget init`                        | Interaktiver Konfigurationsassistent  |
| `vget update`                      | Aktualisieren (`sudo` auf Mac/Linux)  |
| `vget search --podcast <query>`    | Podcasts suchen                       |
| `vget completion [shell]`          | Shell-Vervollständigung generieren    |
| `vget config show`                 | Konfiguration anzeigen                |
| `vget config set <key> <value>`    | Konfigurationswert setzen (nicht interaktiv) |
| `vget config get <key>`            | Konfigurationswert abrufen            |
| `vget config path`                 | Konfigurationsdateipfad anzeigen      |
| `vget config webdav list`          | Konfigurierte WebDAV-Server auflisten |
| `vget config webdav add <name>`    | WebDAV-Server hinzufügen              |
| `vget config webdav show <name>`   | Serverdetails anzeigen                |
| `vget config webdav delete <name>` | Server löschen                        |

### Beispiele

```bash
vget https://twitter.com/user/status/123456789
vget https://www.xiaoyuzhoufm.com/episode/abc123
vget https://example.com/video.mp4 -o mein_video.mp4
vget --info https://example.com/video
vget search --podcast "tech news"
vget pikpak:/path/to/file.mp4              # WebDAV-Download
vget ls pikpak:/Movies                     # Remote-Verzeichnis auflisten
```

## Konfiguration

Speicherort der Konfigurationsdatei:

| OS          | Pfad                        |
| ----------- | --------------------------- |
| macOS/Linux | `~/.config/vget/config.yml` |
| Windows     | `%APPDATA%\vget\config.yml` |

Führen Sie `vget init` aus, um die Konfigurationsdatei interaktiv zu erstellen, oder erstellen Sie sie manuell:

```yaml
language: de # en, zh, jp, kr, es, fr, de
```

**Hinweis:** Die Konfiguration wird bei jedem Befehl neu gelesen. Kein Neustart nach Änderungen erforderlich (nützlich für Docker).

## Aktualisierung

Um vget auf die neueste Version zu aktualisieren:

**macOS / Linux:**
```bash
sudo vget update
```

**Windows (PowerShell als Administrator ausführen):**
```powershell
vget update
```

## Sprachen

vget unterstützt mehrere Sprachen:

- English (en)
- 中文 (zh)
- 日本語 (jp)
- 한국어 (kr)
- Español (es)
- Français (fr)
- Deutsch (de)

## Lizenz

Apache License 2.0
