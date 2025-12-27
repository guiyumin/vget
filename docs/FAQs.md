# FAQs & Troubleshooting

## FFmpeg Merge Failed: thread_create failed

**Error message:**

```
ffmpeg merge failed: thread_create failed: Operation not permitted.
Try to increase 'ulimit -v' or decrease 'ulimit -s'.
```

**Cause:** This is a system resource limitation, not a vget bug. FFmpeg cannot create threads due to OS-level restrictions on your system.

**Common scenarios:**

- Running in a Docker container with restricted resources
- VPS or shared hosting with strict ulimit settings
- Systems with low thread/memory limits

**Solutions:**

### If running in Docker

Add ulimit settings to your container:

**compose.yml:**

```yaml
services:
  vget:
    image: your-vget-image
    ulimits:
      nproc: 65535
      nofile:
        soft: 65535
        hard: 65535
```

**docker run:**

```bash
docker run --ulimit nproc=65535 --ulimit nofile=65535:65535 your-vget-image
```

### If running on bare Linux

Adjust ulimit settings before running vget:

```bash
# Reduce stack size
ulimit -s 8192

# Or increase virtual memory limit
ulimit -v unlimited
```

To make changes permanent, edit `/etc/security/limits.conf`:

```
*    soft    nproc     65535
*    hard    nproc     65535
*    soft    nofile    65535
*    hard    nofile    65535
```

### Alternative workaround

If you cannot change system limits, download video and audio separately without merging (if supported by the source).
