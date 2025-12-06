# vget

Outil en ligne de commande polyvalent pour télécharger audio, vidéo, podcasts et plus.

[English](README.md) | [简体中文](README_zh.md) | [日本語](README_jp.md) | [한국어](README_kr.md) | [Español](README_es.md) | [Deutsch](README_de.md)

## Installation

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

Téléchargez `vget-windows-amd64.exe` depuis [Releases](https://github.com/guiyumin/vget/releases/latest) et ajoutez-le au PATH.

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
| `vget config path`                 | Afficher le chemin du fichier config  |
| `vget config webdav list`          | Lister les serveurs WebDAV configurés |
| `vget config webdav add <name>`    | Ajouter un serveur WebDAV             |
| `vget config webdav show <name>`   | Afficher les détails du serveur       |
| `vget config webdav delete <name>` | Supprimer un serveur                  |

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

## Sources prises en charge

Consultez [sites.md](sites.md) pour la liste complète des sites pris en charge.

### Contenu Twitter/X réservé aux adultes

Pour télécharger du contenu avec restriction d'âge (NSFW) de Twitter/X, vous devez configurer votre auth token :

1. Ouvrez x.com dans votre navigateur et connectez-vous
2. Ouvrez DevTools (F12) → Application → Cookies → x.com
3. Trouvez `auth_token` et copiez sa valeur
4. Exécutez :
   ```bash
   vget config twitter set
   # collez votre auth_token quand demandé
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
