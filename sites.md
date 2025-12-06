# Supported Sites

## General

| Source                    | URL                      | Type            |
| ------------------------- | ------------------------ | --------------- |
| Twitter/X                 | twitter.com, x.com       | Video           |
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
   vget config twitter set
   # paste your auth_token when prompted
   ```

### Twitter/X 年龄限制内容

要下载 Twitter/X 上的年龄限制（NSFW）内容，需要设置 auth token：

1. 在浏览器中打开 x.com 并登录
2. 打开开发者工具（F12）→ Application → Cookies → x.com
3. 找到 `auth_token` 并复制其值
4. 运行：
   ```bash
   vget config twitter set
   # 按提示粘贴 auth_token
   ```
