# vget Docker Image
# Single image for all users - runtime GPU detection determines behavior:
#   - With NVIDIA GPU (--gpus all): Local transcription, download models on demand
#   - Without GPU: Cloud API mode (OpenAI Whisper, Groq, etc.)
#
# Build args:
#   ENABLE_CUDA=true  - amd64 builds (includes CUDA runtime for GPU support)
#   ENABLE_CUDA=false - arm64 builds (no NVIDIA GPUs available)

ARG ENABLE_CUDA=false

# Build stage for UI
FROM node:22-slim AS ui-builder
WORKDIR /app/ui
COPY ui/package*.json ./
RUN npm ci
COPY ui/ ./
RUN npm run build

# Build stage for Go binary
# Uses pre-built base image with sherpa-onnx + whisper.cpp already installed
# Base image is built by .github/workflows/build-base-image.yml
FROM ghcr.io/guiyumin/vget-base:latest AS go-builder-false
FROM ghcr.io/guiyumin/vget-base:cuda AS go-builder-true

# Select builder based on CUDA (false=CPU, true=CUDA)
FROM go-builder-${ENABLE_CUDA} AS go-builder
WORKDIR /app

# Copy go mod files first for caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Copy built UI into embed location
COPY --from=ui-builder /app/ui/dist ./internal/server/dist

# Build the server binary with CGO enabled
RUN go build -ldflags="-s -w" -o /vget-server ./cmd/vget-server

# Final runtime stage - use CUDA runtime for GPU support
FROM debian:bookworm-slim AS runtime-false
FROM nvidia/cuda:12.6.3-runtime-ubuntu24.04 AS runtime-true

# Select runtime based on CUDA (false=CPU, true=CUDA)
FROM runtime-${ENABLE_CUDA} AS runtime

# Re-declare args after FROM
ARG ENABLE_CUDA=false

# No models bundled - downloaded on demand at runtime
# GPU users: download from HuggingFace or vmirror (China)
# No GPU: use cloud API (OpenAI Whisper, Groq, etc.)

# Install runtime dependencies
# - ca-certificates: for HTTPS requests
# - chromium: for browser-based extractors (XHS, etc.)
# - font packages: for proper text rendering in headless browser
# - python3/pip: for yt-dlp and youtube-dl
# - ffmpeg: for merging video/audio streams and audio conversion
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
    bzip2 \
    && rm -rf /var/lib/apt/lists/*

# Install yt-dlp and youtube-dl
RUN pip3 install --no-cache-dir --break-system-packages \
    yt-dlp \
    youtube-dl

# Create non-root user
# Models go in /home/vget/models (separate from config to avoid bind mount conflicts)
RUN groupadd -g 1000 vget && \
    useradd -u 1000 -g vget -m -d /home/vget vget && \
    mkdir -p /home/vget/downloads /home/vget/.config/vget /home/vget/models && \
    chown -R vget:vget /home/vget

# Copy sherpa-onnx shared libraries from builder (for Parakeet)
COPY --from=go-builder /usr/local/lib/libsherpa-onnx-*.so* /usr/local/lib/
COPY --from=go-builder /usr/local/lib/libonnxruntime*.so* /usr/local/lib/

# Copy whisper.cpp shared libraries from builder (for Whisper)
COPY --from=go-builder /usr/local/lib/libwhisper*.so* /usr/local/lib/
COPY --from=go-builder /usr/local/lib/libggml*.so* /usr/local/lib/
RUN ldconfig

# Models are downloaded on demand at runtime via:
#   vget ai download whisper-large-v3-turbo
#   vget ai download whisper-large-v3-turbo --from=vmirror  (for China)

# Copy binary from builder
COPY --from=go-builder /vget-server /usr/local/bin/vget-server

# Tell rod to use system chromium instead of downloading
ENV ROD_BROWSER=/usr/bin/chromium
ENV LD_LIBRARY_PATH=/usr/local/lib

# For CUDA: set NVIDIA visible devices
ENV NVIDIA_VISIBLE_DEVICES=all
ENV NVIDIA_DRIVER_CAPABILITIES=compute,utility

# Copy entrypoint script
COPY docker/vget/entrypoint.sh /usr/local/bin/
RUN chmod +x /usr/local/bin/entrypoint.sh

WORKDIR /home/vget

EXPOSE 8080

VOLUME ["/home/vget/downloads", "/home/vget/.config/vget"]

ENTRYPOINT ["entrypoint.sh"]
CMD []
