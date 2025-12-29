# Build stage for UI
FROM node:22-slim AS ui-builder
WORKDIR /app/ui
COPY ui/package*.json ./
RUN npm ci
COPY ui/ ./
RUN npm run build

# Build stage for Go binary
# Uses pre-built base image with whisper.cpp already compiled
# Base image is built by .github/workflows/build-base-image.yml
FROM ghcr.io/guiyumin/vget-base:latest AS go-builder
WORKDIR /app

# Copy go mod files first for caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Copy built UI into embed location
COPY --from=ui-builder /app/ui/dist ./internal/server/dist

# Build the server binary with CGO enabled for whisper.cpp
# WHISPER_PATH, C_INCLUDE_PATH, LIBRARY_PATH are set in base image
RUN go build -ldflags="-s -w" -o /vget-server ./cmd/vget-server

# Final runtime stage
FROM debian:bookworm-slim

# Whisper model to bundle (none, small, medium, large)
# - none: No model, user downloads on first use (~150MB image)
# - small: ggml-small.bin for NAS <8GB RAM (~400MB image)
# - medium: ggml-medium.bin for 8-16GB RAM (~900MB image)
# - large: ggml-large-v3-turbo.bin for GPU users (~1.7GB image)
ARG WHISPER_MODEL=none

# Install runtime dependencies
# - ca-certificates: for HTTPS requests
# - chromium: for browser-based extractors (XHS, etc.)
# - font packages: for proper text rendering in headless browser
# - python3/pip: for yt-dlp and youtube-dl
# - ffmpeg: for merging video/audio streams
# - nodejs: for yt-dlp JS challenge solving (N parameter)
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    chromium \
    fonts-noto-cjk \
    fonts-noto-color-emoji \
    python3 \
    python3-pip \
    python3-venv \
    ffmpeg \
    nodejs \
    gosu \
    curl \
    && rm -rf /var/lib/apt/lists/*

# Install yt-dlp and youtube-dl
RUN pip3 install --no-cache-dir --break-system-packages \
    yt-dlp \
    youtube-dl

# Create non-root user
RUN groupadd -g 1000 vget && \
    useradd -u 1000 -g vget -m -d /home/vget vget && \
    mkdir -p /home/vget/downloads /home/vget/.config/vget/models && \
    chown -R vget:vget /home/vget

# Download whisper model if specified
RUN if [ "$WHISPER_MODEL" = "small" ]; then \
        echo "Downloading ggml-small.bin (244MB)..." && \
        curl -L -o /home/vget/.config/vget/models/ggml-small.bin \
            https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-small.bin && \
        chown vget:vget /home/vget/.config/vget/models/ggml-small.bin; \
    elif [ "$WHISPER_MODEL" = "medium" ]; then \
        echo "Downloading ggml-medium.bin (769MB)..." && \
        curl -L -o /home/vget/.config/vget/models/ggml-medium.bin \
            https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-medium.bin && \
        chown vget:vget /home/vget/.config/vget/models/ggml-medium.bin; \
    elif [ "$WHISPER_MODEL" = "large" ]; then \
        echo "Downloading ggml-large-v3-turbo.bin (1.5GB)..." && \
        curl -L -o /home/vget/.config/vget/models/ggml-large-v3-turbo.bin \
            https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-large-v3-turbo.bin && \
        chown vget:vget /home/vget/.config/vget/models/ggml-large-v3-turbo.bin; \
    else \
        echo "No model bundled (WHISPER_MODEL=$WHISPER_MODEL)"; \
    fi

# Copy binary from builder
COPY --from=go-builder /vget-server /usr/local/bin/vget-server

# Tell rod to use system chromium instead of downloading
ENV ROD_BROWSER=/usr/bin/chromium

# Copy entrypoint script
COPY docker/vget/entrypoint.sh /usr/local/bin/
RUN chmod +x /usr/local/bin/entrypoint.sh

WORKDIR /home/vget

EXPOSE 8080

VOLUME ["/home/vget/downloads", "/home/vget/.config/vget"]

ENTRYPOINT ["entrypoint.sh"]
CMD []
