#!/bin/bash
set -e

# GPU detection and user guidance
if nvidia-smi &>/dev/null; then
    echo "✓ NVIDIA GPU detected - local transcription enabled"
    nvidia-smi --query-gpu=name,memory.total --format=csv,noheader 2>/dev/null | head -1
else
    echo "─────────────────────────────────────────────────────────"
    echo "  No GPU detected - using cloud API mode"
    echo ""
    echo "  Have an NVIDIA GPU? Run with GPU access:"
    echo "    docker run --gpus all -p 8080:8080 ghcr.io/guiyumin/vget:latest"
    echo ""
    echo "  Or in compose.yml:"
    echo "    deploy:"
    echo "      resources:"
    echo "        reservations:"
    echo "          devices:"
    echo "            - driver: nvidia"
    echo "              count: all"
    echo "              capabilities: [gpu]"
    echo ""
    echo "  See: docs/ai/docker-gpu-passthrough.md"
    echo "─────────────────────────────────────────────────────────"
fi

# Fix ownership of mounted volumes if running as root
if [ "$(id -u)" = "0" ]; then
    chown -R vget:vget /home/vget/downloads /home/vget/.config/vget
    exec gosu vget vget-server "$@"
else
    exec vget-server "$@"
fi
