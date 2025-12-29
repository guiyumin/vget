# vget

Téléchargeur polyvalent pour audio, vidéo, podcasts, PDFs et plus. Disponible en CLI et Docker.

[English](README.md) | [简体中文](README_zh.md) | [日本語](README_jp.md) | [한국어](README_kr.md) | [Español](README_es.md) | [Deutsch](README_de.md)

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

Téléchargez `vget-windows-amd64.zip` depuis [Releases](https://github.com/guiyumin/vget/releases/latest), extrayez-le et ajoutez-le au PATH.

## Captures d'écran

### Progression du téléchargement

![Progression du téléchargement](screenshots/pikpak_download.png)

### Interface serveur Docker

![](screenshots/vget_server_ui.png)

## Docker

```bash
docker run -d -p 8080:8080 -v ~/downloads:/home/vget/downloads ghcr.io/guiyumin/vget:latest
```

### Variantes d'image

| Tag            | Modèles                           | Architecture | Support CPU/GPU |
|----------------|-----------------------------------|--------------|-----------------|
| `:latest`      | Aucun (téléchargé au premier usage) | amd64/arm64 | CPU uniquement  |
| `:small`       | Parakeet V3 + Whisper Small       | amd64/arm64  | CPU uniquement  |
| `:medium`      | Parakeet V3 + Whisper Medium      | amd64/arm64  | CPU uniquement  |
| `:large`       | Parakeet V3 + Whisper Large Turbo | amd64/arm64  | CPU uniquement  |
| `:cuda`        | Aucun (téléchargé au premier usage) | amd64      | CPU ou GPU      |
| `:cuda-small`  | Parakeet V3 + Whisper Small       | amd64        | CPU ou GPU      |
| `:cuda-medium` | Parakeet V3 + Whisper Medium      | amd64        | CPU ou GPU      |
| `:cuda-large`  | Parakeet V3 + Whisper Large Turbo | amd64        | CPU ou GPU      |

**Recommandations de modèles :**
- **NAS avec <8 Go RAM :** `:small`
- **8-16 Go RAM :** `:medium`
- **32 Go+ RAM ou NVIDIA GPU :** `:large` ou `:cuda-large`

**Pour les utilisateurs NVIDIA GPU :**

```bash
docker run -d --gpus all -p 8080:8080 -v ~/downloads:/home/vget/downloads ghcr.io/guiyumin/vget:cuda-large
```

## Sources prises en charge

Consultez [sites.md](sites.md) pour la liste complète des sites pris en charge.

## Commandes

| Commande                           | Description                           |
|------------------------------------|---------------------------------------|
| `vget [url]`                       | Télécharger des médias (`-o`, `-q`, `--info`) |
| `vget ls <remote>:<path>`          | Lister un répertoire distant (`--json`) |
| `vget init`                        | Assistant de configuration interactif |
| `vget update`                      | Mise à jour (`sudo` sur Mac/Linux)    |
| `vget search --podcast <query>`    | Rechercher des podcasts               |
| `vget completion [shell]`          | Générer un script d'autocomplétion    |
| `vget config show`                 | Afficher la configuration             |
| `vget config set <key> <value>`    | Définir une valeur de config (non interactif) |
| `vget config get <key>`            | Obtenir une valeur de configuration   |
| `vget config path`                 | Afficher le chemin du fichier config  |
| `vget config webdav list`          | Lister les serveurs WebDAV configurés |
| `vget config webdav add <name>`    | Ajouter un serveur WebDAV             |
| `vget config webdav show <name>`   | Afficher les détails du serveur       |
| `vget config webdav delete <name>` | Supprimer un serveur                  |
| `vget telegram login --import-desktop` | Importer la session Telegram depuis l'app de bureau |

### Exemples

```bash
vget https://twitter.com/user/status/123456789
vget https://www.xiaoyuzhoufm.com/episode/abc123
vget https://example.com/video -o ma_video.mp4
vget --info https://example.com/video
vget search --podcast "tech news"
vget pikpak:/path/to/file.mp4              # Téléchargement WebDAV
vget ls pikpak:/Movies                     # Lister un répertoire distant
```

## Configuration

Emplacement du fichier de configuration :

| OS          | Chemin                      |
| ----------- | --------------------------- |
| macOS/Linux | `~/.config/vget/config.yml` |
| Windows     | `%APPDATA%\vget\config.yml` |

Exécutez `vget init` pour créer le fichier de configuration de manière interactive, ou créez-le manuellement :

```yaml
language: fr # en, zh, jp, kr, es, fr, de
```

**Note :** La configuration est lue à chaque commande. Pas de redémarrage nécessaire après modification (utile pour Docker).

## Mise à jour

Pour mettre à jour vget vers la dernière version :

**macOS / Linux :**
```bash
sudo vget update
```

**Windows (exécuter PowerShell en tant qu'Administrateur) :**
```powershell
vget update
```

## Langues

vget prend en charge plusieurs langues :

- English (en)
- 中文 (zh)
- 日本語 (jp)
- 한국어 (kr)
- Español (es)
- Français (fr)
- Deutsch (de)

## Licence

Apache License 2.0
