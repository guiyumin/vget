# TODO

> 产品方向见 docs/PRD.md（资源下载器 + WebDAV 资源库客户端）。
> 本文只记通用下载器的零散改进项；库相关路线图在 PRD。

## Features

- [ ] Resume interrupted downloads
- [ ] Retry on failure
- [ ] Quiet/verbose modes
- [ ] Dry run mode
- [ ] Playlist support
- [ ] Rate limiting
- [ ] Metadata embedding
  - Audio (MP3/M4A): ID3 tags - title, artist, album, cover art
  - Video (MP4): title, description, thumbnail
  - Auto-fill from source (podcast name, episode title, artwork)
  - Media players (Apple Music, VLC, etc.) would then show this info instead of just the filename.
