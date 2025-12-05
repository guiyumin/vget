# YouTube Extractor Notes

## Status: Experimental

The YouTube extractor works but is fragile due to YouTube's aggressive anti-bot measures.

## What We Learned (2025-12-04)

### 1. The code architecture is sound - it worked earlier
- Browser automation (Rod + stealth) captures BotGuard tokens
- Innertube API with iOS client returns unencrypted stream URLs (no cipher)
- Separate video/audio streams can be downloaded and merged with ffmpeg

### 2. YouTube's anti-bot (BotGuard) is detecting rod/stealth
- Error 153 in browser = automation detected
- Without successful playback, POToken is not captured
- go-rod/stealth may need updates as YouTube improves detection

### 3. IP binding on stream URLs is strict
- Stream URLs contain the IP address that requested them
- Downloads from a different IP get 403 Forbidden
- VPNs can cause IP mismatch (browser vs Go http.Client)
- IPv6 can cause issues - different processes may get different IPv6 addresses

### 4. Rate limiting after heavy testing
- Too many requests flags the IP/session
- Even new IPs may not help if browser profile is flagged
- Solution: Clear `~/.config/vget/browser/` and `~/.config/vget/youtube_session.json`
- Wait 24-48 hours for rate limiting to expire

## Troubleshooting

### 403 on download
1. Clear browser profile: `rm -rf ~/.config/vget/browser/`
2. Disable IPv6: `sudo networksetup -setv6off Wi-Fi`
3. Try a different network/IP
4. Wait for rate limiting to expire

### No POToken captured
- YouTube is detecting automation
- Browser shows error 153
- Stealth mode may need updates

### IP mismatch
- Ensure VPN tunnels ALL traffic (not just browser)
- Disable IPv6 to force IPv4
- Browser, API call, and download must use same IP

## Future Improvements

1. **yt-dlp fallback** - Shell out to yt-dlp when available
2. **Better stealth** - Keep rod/stealth updated
3. **Session reuse** - Cache valid sessions to reduce detection
4. **Proxy support** - Residential proxies for reliability
