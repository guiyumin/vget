# vget

Herramienta de línea de comandos versátil para descargar audio, video, podcasts y más.

[English](README.md) | [简体中文](README_zh.md) | [日本語](README_jp.md) | [한국어](README_kr.md) | [Français](README_fr.md) | [Deutsch](README_de.md)

## Instalación

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

Descarga `vget-windows-amd64.exe` desde [Releases](https://github.com/guiyumin/vget/releases/latest) y agrégalo al PATH.

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
| `vget config path`                 | Mostrar ruta del archivo de config    |
| `vget config webdav list`          | Listar servidores WebDAV configurados |
| `vget config webdav add <name>`    | Agregar servidor WebDAV               |
| `vget config webdav show <name>`   | Mostrar detalles del servidor         |
| `vget config webdav delete <name>` | Eliminar servidor                     |

### Ejemplos

```bash
vget https://twitter.com/user/status/123456789
vget https://www.xiaoyuzhoufm.com/episode/abc123
vget https://example.com/video -o mi_video.mp4
vget --info https://example.com/video
vget search --podcast "tech news"
vget pikpak:/path/to/file.mp4              # Descarga WebDAV
vget ls pikpak:/Movies                     # Listar directorio remoto
```

## Fuentes compatibles

Consulta [sites.md](sites.md) para la lista completa de sitios compatibles.

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
