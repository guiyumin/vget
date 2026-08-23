# vget

Un descargador pequeño y enfocado para enlaces multimedia y bibliotecas de recursos WebDAV. Disponible como CLI y Docker.

[English](README.md) | [简体中文](README_zh.md) | [日本語](README_jp.md) | [한국어](README_kr.md) | [Français](README_fr.md) | [Deutsch](README_de.md) | [Português](README_pt.md)

## Qué es

vget es una interfaz fina y simple sobre los recursos que hay detrás. Apúntalo a un enlace o a una biblioteca de recursos WebDAV y descarga el archivo — con una barra de progreso en la terminal, o una interfaz web en tu NAS.

La herramienta se mantiene deliberadamente pequeña. El valor no es el descargador, sino las bibliotecas de recursos a las que se conecta. Por eso vget habla protocolos estándar en lugar de hacer scraping — no se rompe cuando un sitio cambia su maquetación, y cualquier servidor WebDAV que alguien monte se convierte en una biblioteca que puede explorar y descargar.

- **Enlaces multimedia** — Twitter/X (vídeo/imagen), podcasts (Xiaoyuzhou, Apple Podcasts), URLs de archivos directos, streams m3u8/HLS
- **Bibliotecas de recursos WebDAV** — PikPak, un NAS, un seedbox o la biblioteca WebDAV compartida por cualquiera: explorar, elegir, descargar
- **Nubes sin WebDAV nativo** (Baidu, Quark, Aliyun, 115, …) — conéctalas a través de [OpenList](https://github.com/OpenListTeam/OpenList)/Alist y apunta vget al endpoint WebDAV

> La dirección y la hoja de ruta (hospedar una biblioteca con un comando, un cliente de bibliotecas, un directorio público) están en [docs/PRD.md](docs/PRD.md).

## Instalación

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

Descarga `vget-windows-amd64.zip` desde [Releases](https://github.com/guiyumin/vget/releases/latest), extráelo y agrégalo al PATH.

## Capturas de pantalla

### Progreso de descarga

![Progreso de descarga](screenshots/pikpak_download.png)

### Interfaz del servidor Docker

![](screenshots/vget_server_ui.png)

## Docker

```bash
docker run -d -p 8080:8080 -v ~/downloads:/home/vget/downloads ghcr.io/guiyumin/vget:latest
```

## Fuentes compatibles

Consulta [sites.md](sites.md) para la lista completa de sitios compatibles.

## Comandos

| Comando                            | Descripción                           |
|------------------------------------|---------------------------------------|
| `vget [url]`                       | Descargar medios (`-o`, `-q`, `--info`) |
| `vget ls <remote>:<path>`          | Listar directorio remoto (`--json`)   |
| `vget init`                        | Asistente de configuración interactivo |
| `vget update`                      | Actualizar (usar `sudo` en Mac/Linux) |
| `vget search --podcast <query>`    | Buscar podcasts                       |
| `vget completion [shell]`          | Generar script de autocompletado      |
| `vget config show`                 | Mostrar configuración                 |
| `vget config set <key> <value>`    | Establecer valor de config (no interactivo) |
| `vget config get <key>`            | Obtener valor de configuración        |
| `vget config path`                 | Mostrar ruta del archivo de config    |
| `vget config webdav list`          | Listar servidores WebDAV configurados |
| `vget config webdav add <name>`    | Agregar servidor WebDAV               |
| `vget config webdav show <name>`   | Mostrar detalles del servidor         |
| `vget config webdav delete <name>` | Eliminar servidor                     |

### Ejemplos

```bash
vget https://twitter.com/user/status/123456789
vget https://www.xiaoyuzhoufm.com/episode/abc123
vget https://example.com/video.mp4 -o mi_video.mp4
vget --info https://example.com/video
vget search --podcast "tech news"
vget pikpak:/path/to/file.mp4              # Descarga WebDAV
vget ls pikpak:/Movies                     # Listar directorio remoto
```

## Configuración

Ubicación del archivo de configuración:

| SO          | Ruta                        |
| ----------- | --------------------------- |
| macOS/Linux | `~/.config/vget/config.yml` |
| Windows     | `%APPDATA%\vget\config.yml` |

Ejecuta `vget init` para crear el archivo de configuración interactivamente, o créalo manualmente:

```yaml
language: es # en, zh, jp, kr, es, fr, de
```

**Nota:** La configuración se lee en cada comando. No se requiere reinicio después de cambios (útil para Docker).

## Actualización

Para actualizar vget a la última versión:

**macOS / Linux:**
```bash
sudo vget update
```

**Windows (ejecutar PowerShell como Administrador):**
```powershell
vget update
```

## Idiomas

vget soporta múltiples idiomas:

- English (en)
- 中文 (zh)
- 日本語 (jp)
- 한국어 (kr)
- Español (es)
- Français (fr)
- Deutsch (de)

## Licencia

Apache License 2.0
