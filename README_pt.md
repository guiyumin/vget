# vget

Um downloader pequeno e focado para links de mídia e bibliotecas de recursos WebDAV. Disponível como CLI e Docker.

[English](README.md) | [简体中文](README_zh.md) | [日本語](README_jp.md) | [한국어](README_kr.md) | [Español](README_es.md) | [Français](README_fr.md) | [Deutsch](README_de.md)

## O que é

vget é uma interface fina e simples sobre os recursos que estão por trás dele. Aponte-o para um link ou uma biblioteca de recursos WebDAV e ele baixa o arquivo — com uma barra de progresso no terminal, ou uma interface web no seu NAS.

A ferramenta permanece deliberadamente pequena. O valor não é o downloader, e sim as bibliotecas de recursos às quais ele se conecta. Por isso o vget fala protocolos padrão em vez de fazer scraping — ele não quebra quando um site muda a marcação, e qualquer servidor WebDAV que alguém rode vira uma biblioteca que ele pode navegar e baixar.

- **Links de mídia** — Twitter/X (vídeo/imagem), podcasts (Xiaoyuzhou, Apple Podcasts), URLs de arquivos diretos, streams m3u8/HLS
- **Bibliotecas de recursos WebDAV** — PikPak, um NAS, um seedbox, ou a biblioteca WebDAV compartilhada por qualquer pessoa: navegar, escolher, baixar
- **Nuvens sem WebDAV nativo** (Baidu, Quark, Aliyun, 115, …) — conecte-as através do [OpenList](https://github.com/OpenListTeam/OpenList)/Alist e aponte o vget para o endpoint WebDAV

> A direção e o roadmap (hospedar uma biblioteca com um comando, um cliente de bibliotecas, um diretório público) estão em [docs/PRD.md](docs/PRD.md).

## Instalação

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

Baixe `vget-windows-amd64.zip` em [Releases](https://github.com/guiyumin/vget/releases/latest), extraia e adicione ao seu PATH.

## Capturas de tela

### Progresso do download

![Progresso do download](screenshots/pikpak_download.png)

### Interface web do Docker

![](screenshots/vget_server_ui.png)

## Docker

A imagem Docker roda uma interface web + API HTTP — útil em um NAS, onde você cola links ou navega em bibliotecas WebDAV a partir de qualquer dispositivo.

```bash
docker run -d -p 8080:8080 -v ~/downloads:/home/vget/downloads ghcr.io/guiyumin/vget:latest
```

## Fontes suportadas

Veja os extractors nativos em [sites.md](sites.md). Além deles, qualquer servidor WebDAV funciona out of the box.

## Comandos

| Comando                                | Descrição                                |
| -------------------------------------- | ---------------------------------------- |
| `vget [url]`                           | Baixar mídia (`-o`, `-q`, `--info`)      |
| `vget ls <remote>:<path>`              | Listar diretório remoto (`--json`)       |
| `vget init`                            | Assistente interativo de configuração    |
| `vget update`                          | Autoatualização (use `sudo` no Mac/Linux)|
| `vget search --podcast <query>`        | Pesquisar podcasts                       |
| `vget completion [shell]`              | Gerar script de autocompletar do shell   |
| `vget config show`                     | Mostrar configuração                     |
| `vget config set <key> <value>`        | Definir valor de config (não interativo) |
| `vget config get <key>`                | Obter valor de config                    |
| `vget config path`                     | Mostrar caminho do arquivo de config     |
| `vget config webdav list`              | Listar servidores WebDAV configurados    |
| `vget config webdav add <name>`        | Adicionar um servidor WebDAV             |
| `vget config webdav show <name>`       | Mostrar detalhes do servidor             |
| `vget config webdav delete <name>`     | Remover um servidor                      |

### Exemplos

```bash
# Links de mídia
vget https://twitter.com/user/status/123456789
vget https://www.xiaoyuzhoufm.com/episode/abc123
vget https://example.com/video.mp4 -o meu_video.mp4
vget --info https://example.com/video.mp4
vget search --podcast "tech news"

# Bibliotecas de recursos WebDAV
vget config webdav add pikpak       # adicionar uma biblioteca (url + usuário + senha)
vget ls pikpak:/Movies              # navegar
vget pikpak:/path/to/file.mp4       # baixar dela
```

## Configuração

Localização do arquivo de configuração:

| SO          | Caminho                     |
| ----------- | --------------------------- |
| macOS/Linux | `~/.config/vget/config.yml` |
| Windows     | `%APPDATA%\vget\config.yml` |

Execute `vget init` para criar o arquivo de configuração de forma interativa, ou crie-o manualmente:

```yaml
language: en # en, zh, jp, kr, es, fr, de
```

**Nota:** A configuração é lida a cada comando. Nenhum reinício é necessário após alterações (útil para Docker).

## Atualização

Para atualizar o vget para a versão mais recente:

**macOS / Linux:**

```bash
sudo vget update
```

**Windows (execute o PowerShell como Administrador):**

```powershell
vget update
```

## Idiomas

O vget suporta vários idiomas:

- English (en)
- 中文 (zh)
- 日本語 (jp)
- 한국어 (kr)
- Español (es)
- Français (fr)
- Deutsch (de)

## Licença

Apache License 2.0
