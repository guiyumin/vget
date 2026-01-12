# vget Docker Image
# Build args:
#   ENABLE_CUDA=true  - amd64 builds with CUDA GPU support (uses pre-built base image)
#   ENABLE_CUDA=false - arm64 builds (CPU only)
#
# Runtime: App detects GPU and enables AI features only if GPU is available

ARG ENABLE_CUDA=false

# Build stage for UI
FROM node:22-slim AS ui-builder
WORKDIR /app/ui
COPY ui/package*.json ./
RUN npm ci
COPY ui/ ./
RUN npm run build

# Go builder stage
# CUDA: use pre-built base image with whisper.cpp
# Non-CUDA: build from scratch
FROM golang:1.25-bookworm AS go-builder-base
FROM ghcr.io/guiyumin/vget-base:latest AS go-builder-cuda

ARG ENABLE_CUDA=false
FROM go-builder-cuda AS go-builder-select-true
FROM go-builder-base AS go-builder-select-false
FROM go-builder-select-${ENABLE_CUDA} AS go-builder

ARG ENABLE_CUDA=false
ARG TARGETARCH

# For non-CUDA builds, install dependencies and build from source
RUN if [ "$ENABLE_CUDA" = "false" ]; then \
        apt-get update && apt-get install -y --no-install-recommends \
            build-essential cmake curl git && \
        rm -rf /var/lib/apt/lists/*; \
    fi

# whisper.cpp (non-CUDA only, CUDA base image already has it)
ARG WHISPER_VERSION=1.8.2

RUN if [ "$ENABLE_CUDA" = "false" ]; then \
        git clone --depth 1 --branch v${WHISPER_VERSION} https://github.com/ggerganov/whisper.cpp /tmp/whisper.cpp && \
        cd /tmp/whisper.cpp && \
        cmake -B build \
            -DCMAKE_BUILD_TYPE=Release \
            -DBUILD_SHARED_LIBS=ON \
            -DGGML_NATIVE=OFF \
            -DWHISPER_BUILD_EXAMPLES=OFF \
            -DWHISPER_BUILD_TESTS=OFF && \
        cmake --build build --config Release -j$(nproc) && \
        cp build/src/libwhisper.so* /usr/local/lib/ && \
        cp build/ggml/src/libggml*.so* /usr/local/lib/ && \
        cp include/whisper.h /usr/local/include/ && \
        cp ggml/include/ggml*.h /usr/local/include/ && \
        rm -rf /tmp/whisper.cpp && \
        ldconfig; \
    fi

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=ui-builder /app/ui/dist ./internal/server/dist

# Build with CGO for whisper.cpp bindings
ENV CGO_ENABLED=1
ENV LD_LIBRARY_PATH=/usr/local/lib:/usr/local/cuda/lib64:/usr/local/cuda/lib64/stubs
ENV C_INCLUDE_PATH=/usr/local/include
ENV LIBRARY_PATH=/usr/local/lib:/usr/local/cuda/lib64:/usr/local/cuda/lib64/stubs

RUN if [ "$ENABLE_CUDA" = "true" ]; then \
        ln -sf /usr/local/cuda/lib64/stubs/libcuda.so /usr/local/lib/libcuda.so && \
        ln -sf /usr/local/cuda/lib64/stubs/libcuda.so /usr/local/lib/libcuda.so.1 && \
        ln -sf /usr/local/cuda/lib64/stubs/libcuda.so /usr/local/cuda/lib64/stubs/libcuda.so.1 && \
        ldconfig; \
    fi

RUN go build -ldflags="-s -w" -o /vget-server ./cmd/vget-server

# Runtime stage
FROM debian:bookworm-slim AS runtime-base
FROM nvidia/cuda:12.6.3-runtime-ubuntu24.04 AS runtime-cuda

ARG ENABLE_CUDA=false
FROM runtime-cuda AS runtime-select-true
FROM runtime-base AS runtime-select-false
FROM runtime-select-${ENABLE_CUDA} AS runtime

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

RUN pip3 install --no-cache-dir --break-system-packages \
    yt-dlp \
    youtube-dl

RUN (getent group 1000 >/dev/null || groupadd -g 1000 vget) && \
    (id -u 1000 >/dev/null 2>&1 || useradd -u 1000 -g 1000 -m -d /home/vget vget) && \
    mkdir -p /home/vget/downloads /home/vget/.config/vget /home/vget/models && \
    chown -R 1000:1000 /home/vget

# Copy AI libraries
COPY --from=go-builder /usr/local/lib/libwhisper*.so* /usr/local/lib/
COPY --from=go-builder /usr/local/lib/libggml*.so* /usr/local/lib/
RUN ldconfig

COPY --from=go-builder /vget-server /usr/local/bin/vget-server

ENV ROD_BROWSER=/usr/bin/chromium
ENV LD_LIBRARY_PATH=/usr/local/lib
ENV NVIDIA_VISIBLE_DEVICES=all
ENV NVIDIA_DRIVER_CAPABILITIES=compute,utility

COPY docker/vget/entrypoint.sh /usr/local/bin/
RUN chmod +x /usr/local/bin/entrypoint.sh

WORKDIR /home/vget
EXPOSE 8080
VOLUME ["/home/vget/downloads", "/home/vget/.config/vget"]

ENTRYPOINT ["entrypoint.sh"]
CMD []
