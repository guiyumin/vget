# vget Base Image - Go 1.25 with sherpa-onnx + whisper.cpp
# Supports both CPU and CUDA builds via ENABLE_CUDA arg
# Published to ghcr.io/guiyumin/vget-base:latest (CPU) and :cuda (CUDA)

ARG ENABLE_CUDA=false

# Base image selection based on CUDA support
FROM golang:1.25-bookworm AS base-false
FROM nvidia/cuda:12.6.3-devel-ubuntu24.04 AS base-true

# Select the appropriate base (false=CPU, true=CUDA)
FROM base-${ENABLE_CUDA} AS base

LABEL org.opencontainers.image.source=https://github.com/guiyumin/vget
LABEL org.opencontainers.image.description="vget Base - Go 1.25 with sherpa-onnx + whisper.cpp for local speech-to-text"
LABEL org.opencontainers.image.licenses=MIT

# Re-declare ARG after FROM
ARG ENABLE_CUDA=false

# Install Go for CUDA image (already present in golang image)
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

# sherpa-onnx version (for Parakeet)
ARG SHERPA_VERSION=1.12.20

# Download sherpa-onnx headers from source (not included in pre-built binaries)
RUN curl -L "https://github.com/k2-fsa/sherpa-onnx/archive/refs/tags/v${SHERPA_VERSION}.tar.gz" | tar -xzf - && \
    cp sherpa-onnx-${SHERPA_VERSION}/sherpa-onnx/c-api/c-api.h /usr/local/include/ && \
    cp sherpa-onnx-${SHERPA_VERSION}/sherpa-onnx/c-api/cxx-api.h /usr/local/include/ && \
    rm -rf sherpa-onnx-${SHERPA_VERSION}

# Download and install pre-built sherpa-onnx shared libraries
# Use GPU version for CUDA, CPU version otherwise
ARG TARGETARCH
RUN if [ "$ENABLE_CUDA" = "true" ]; then \
        echo "Downloading sherpa-onnx CUDA version..." && \
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

# whisper.cpp commit (must match Go bindings in go.mod)
# Go bindings: github.com/ggerganov/whisper.cpp/bindings/go v0.0.0-20251224075216-6114e692136b
ARG WHISPER_COMMIT=6114e692136b

# Build whisper.cpp from source with shared library
# Enable CUDA if ENABLE_CUDA=true
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

# Set environment variables for CGO
ENV CGO_ENABLED=1
ENV LD_LIBRARY_PATH=/usr/local/lib

# C_INCLUDE_PATH and LIBRARY_PATH for Go bindings
ENV C_INCLUDE_PATH=/usr/local/include
ENV LIBRARY_PATH=/usr/local/lib

# Reset workdir
WORKDIR /app

# Metadata
ENV BASE_IMAGE_VERSION=3.0.0
ENV SHERPA_ONNX_VERSION=${SHERPA_VERSION}
ENV WHISPER_CPP_COMMIT=${WHISPER_COMMIT}
