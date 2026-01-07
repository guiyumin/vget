# vget Docker Image
# Build args:
#   ENABLE_CUDA=true  - amd64 builds with CUDA GPU support
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
FROM golang:1.25-bookworm AS go-builder-base
FROM nvidia/cuda:12.6.3-devel-ubuntu24.04 AS go-builder-cuda

ARG ENABLE_CUDA=false
FROM go-builder-cuda AS go-builder-select-true
FROM go-builder-base AS go-builder-select-false
FROM go-builder-select-${ENABLE_CUDA} AS go-builder

ARG ENABLE_CUDA=false

# Install Go for CUDA image
RUN if [ "$ENABLE_CUDA" = "true" ]; then \
        apt-get update && apt-get install -y --no-install-recommends \
            wget ca-certificates && \
        wget -q https://go.dev/dl/go1.25.4.linux-amd64.tar.gz && \
        tar -C /usr/local -xzf go1.25.4.linux-amd64.tar.gz && \
        rm go1.25.4.linux-amd64.tar.gz && \
        rm -rf /var/lib/apt/lists/*; \
    fi

ENV PATH="/usr/local/go/bin:${PATH}"

# Install build dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    cmake \
    curl \
    git \
    && rm -rf /var/lib/apt/lists/*

# sherpa-onnx
ARG SHERPA_VERSION=1.12.20

RUN curl -L "https://github.com/k2-fsa/sherpa-onnx/archive/refs/tags/v${SHERPA_VERSION}.tar.gz" | tar -xzf - && \
    cp sherpa-onnx-${SHERPA_VERSION}/sherpa-onnx/c-api/c-api.h /usr/local/include/ && \
    cp sherpa-onnx-${SHERPA_VERSION}/sherpa-onnx/c-api/cxx-api.h /usr/local/include/ && \
    rm -rf sherpa-onnx-${SHERPA_VERSION}

ARG TARGETARCH
RUN if [ "$ENABLE_CUDA" = "true" ]; then \
        curl -L "https://github.com/k2-fsa/sherpa-onnx/releases/download/v${SHERPA_VERSION}/sherpa-onnx-v${SHERPA_VERSION}-cuda-12.x-cudnn-9.x-linux-x64-gpu.tar.bz2" | tar -xjf - && \
        cp sherpa-onnx-v${SHERPA_VERSION}-cuda-12.x-cudnn-9.x-linux-x64-gpu/lib/*.so* /usr/local/lib/ && \
        rm -rf sherpa-onnx-v${SHERPA_VERSION}-cuda-12.x-cudnn-9.x-linux-x64-gpu; \
    else \
        if [ "$TARGETARCH" = "arm64" ]; then \
            SHERPA_ARCH="aarch64"; \
            SHERPA_SUFFIX="-cpu"; \
        else \
            SHERPA_ARCH="x64"; \
            SHERPA_SUFFIX=""; \
        fi && \
        curl -L "https://github.com/k2-fsa/sherpa-onnx/releases/download/v${SHERPA_VERSION}/sherpa-onnx-v${SHERPA_VERSION}-linux-${SHERPA_ARCH}-shared${SHERPA_SUFFIX}.tar.bz2" | tar -xjf - && \
        cp sherpa-onnx-v${SHERPA_VERSION}-linux-${SHERPA_ARCH}-shared${SHERPA_SUFFIX}/lib/*.so* /usr/local/lib/ && \
        rm -rf sherpa-onnx-v${SHERPA_VERSION}-linux-${SHERPA_ARCH}-shared${SHERPA_SUFFIX}; \
    fi && \
    ldconfig

# whisper.cpp
ARG WHISPER_COMMIT=6114e692136b

RUN git clone https://github.com/ggerganov/whisper.cpp /tmp/whisper.cpp && \
    cd /tmp/whisper.cpp && git checkout ${WHISPER_COMMIT} && \
    if [ "$ENABLE_CUDA" = "true" ]; then \
        cmake -B build \
            -DCMAKE_BUILD_TYPE=Release \
            -DBUILD_SHARED_LIBS=ON \
            -DGGML_NATIVE=OFF \
            -DGGML_CUDA=ON \
            -DWHISPER_BUILD_EXAMPLES=OFF \
            -DWHISPER_BUILD_TESTS=OFF; \
    else \
        cmake -B build \
            -DCMAKE_BUILD_TYPE=Release \
            -DBUILD_SHARED_LIBS=ON \
            -DGGML_NATIVE=OFF \
            -DWHISPER_BUILD_EXAMPLES=OFF \
            -DWHISPER_BUILD_TESTS=OFF; \
    fi && \
    cmake --build build --config Release -j$(nproc) && \
    cp build/src/libwhisper.so* /usr/local/lib/ && \
    cp build/ggml/src/libggml*.so* /usr/local/lib/ && \
    cp include/whisper.h /usr/local/include/ && \
    cp ggml/include/ggml*.h /usr/local/include/ && \
    rm -rf /tmp/whisper.cpp && \
    ldconfig

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=ui-builder /app/ui/dist ./internal/server/dist

# Build with CGO for whisper.cpp/sherpa-onnx bindings
ENV CGO_ENABLED=1
ENV LD_LIBRARY_PATH=/usr/local/lib
ENV C_INCLUDE_PATH=/usr/local/include
ENV LIBRARY_PATH=/usr/local/lib

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
COPY --from=go-builder /usr/local/lib/libsherpa-onnx-*.so* /usr/local/lib/
COPY --from=go-builder /usr/local/lib/libonnxruntime*.so* /usr/local/lib/
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
