# Supported Sites

## General

| Source                    | URL                      | Type            |
| ------------------------- | ------------------------ | --------------- |
| Twitter/X                 | twitter.com, x.com       | Video           |
| Telegram                  | t.me                     | Video/Image     |
| Xiaoyuzhou FM (小宇宙)    | xiaoyuzhoufm.com         | Audio (Podcast) |
| Apple Podcasts            | podcasts.apple.com       | Audio (Podcast) |
| Xiaohongshu (小红书)      | xiaohongshu.com          | Video/Image     |

## NSFW

| Source         | URL                      | Type            |
| -------------- | ------------------------ | --------------- |
| hsex.icu       | hsex.icu                 | Video           |
| kanav.ad       | kanav.ad                 | Video           |

## Notes

### Twitter/X Age-Restricted Content

To download age-restricted (NSFW) content from Twitter/X, you need to set your auth token:

1. Open x.com in your browser and log in
2. Open DevTools (F12) → Application → Cookies → x.com
3. Find `auth_token` and copy its value
4. Run:
   ```bash
   vget config set twitter.auth_token YOUR_AUTH_TOKEN
   ```

### Twitter/X 年龄限制内容

要下载 Twitter/X 上的年龄限制（NSFW）内容，需要设置 auth token：

1. 在浏览器中打开 x.com 并登录
2. 打开开发者工具（F12）→ Application → Cookies → x.com
3. 找到 `auth_token` 并复制其值
4. 运行：
   ```bash
   vget config set twitter.auth_token YOUR_AUTH_TOKEN
   ```

### Telegram

To download videos and images from Telegram, you need to import your session from Telegram Desktop:

1. Update vget to v0.7.0 or later
2. Make sure you have [Telegram Desktop](https://desktop.telegram.org/) installed and logged in
3. Run the login command to import your session:
   ```bash
   vget telegram login --import-desktop
   ```
4. Download media like any other URL:
   ```bash
   vget https://t.me/channel/123
   ```

### Telegram (中文)

要从 Telegram 下载视频和图片，需要从 Telegram Desktop 导入会话：

1. 更新 vget 到 v0.7.0 或更高版本
2. 确保已安装并登录 [Telegram Desktop](https://desktop.telegram.org/)
3. 运行登录命令导入会话：
   ```bash
   vget telegram login --import-desktop
   ```
4. 像其他 URL 一样下载媒体：
   ```bash
   vget https://t.me/channel/123
   ```
