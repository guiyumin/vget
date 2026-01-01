# ARG must be declared before first FROM to be available in FROM lines
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
ARG MODEL_VARIANT=none

# Model variant to bundle (none, small, medium, large)
# - none: No models, user downloads on first use (~200MB image)
# - small: Parakeet V3 + Whisper Small for NAS <8GB RAM (~1.0GB image)
# - medium: Parakeet V3 + Whisper Medium for 8-16GB RAM (~1.5GB image)
# - large: Parakeet V3 + Whisper Large Turbo for GPU or 32GB+ RAM (~2.3GB image)

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

# Download models based on variant
RUN if [ "$MODEL_VARIANT" = "small" ] || [ "$MODEL_VARIANT" = "medium" ] || [ "$MODEL_VARIANT" = "large" ]; then \
        echo "Downloading Parakeet V3 INT8 model (~640MB)..." && \
        curl -L https://github.com/k2-fsa/sherpa-onnx/releases/download/asr-models/sherpa-onnx-nemo-parakeet-tdt-0.6b-v3-int8.tar.bz2 | tar -xjf - -C /home/vget/models/ && \
        chown -R vget:vget /home/vget/models/sherpa-onnx-nemo-parakeet-tdt-0.6b-v3-int8; \
    fi

# Download Whisper ggml models for whisper.cpp
# Variants: small, medium, turbo
RUN if [ "$MODEL_VARIANT" = "small" ]; then \
        echo "Downloading Whisper Small ggml model (~466MB)..." && \
        curl -L -o /home/vget/models/ggml-small.bin https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-small.bin && \
        chown vget:vget /home/vget/models/ggml-small.bin; \
    elif [ "$MODEL_VARIANT" = "medium" ]; then \
        echo "Downloading Whisper Medium ggml model (~1.5GB)..." && \
        curl -L -o /home/vget/models/ggml-medium.bin https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-medium.bin && \
        chown vget:vget /home/vget/models/ggml-medium.bin; \
    elif [ "$MODEL_VARIANT" = "turbo" ]; then \
        echo "Downloading Whisper Turbo ggml model (~1.6GB)..." && \
        curl -L -o /home/vget/models/ggml-large-v3-turbo.bin https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-large-v3-turbo.bin && \
        chown vget:vget /home/vget/models/ggml-large-v3-turbo.bin; \
    else \
        echo "No Whisper models bundled (MODEL_VARIANT=$MODEL_VARIANT)"; \
    fi

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
