# vget Base Image for CUDA builds
# Contains pre-built whisper.cpp and sherpa-onnx libraries
# Built bi-weekly via build-base-images.yml workflow

FROM nvidia/cuda:12.6.3-devel-ubuntu24.04

# Install Go
RUN apt-get update && apt-get install -y --no-install-recommends \
    wget ca-certificates && \
    wget -q https://go.dev/dl/go1.25.4.linux-amd64.tar.gz && \
    tar -C /usr/local -xzf go1.25.4.linux-amd64.tar.gz && \
    rm go1.25.4.linux-amd64.tar.gz && \
    rm -rf /var/lib/apt/lists/*

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

RUN curl -L "https://github.com/k2-fsa/sherpa-onnx/releases/download/v${SHERPA_VERSION}/sherpa-onnx-v${SHERPA_VERSION}-cuda-12.x-cudnn-9.x-linux-x64-gpu.tar.bz2" | tar -xjf - && \
    cp sherpa-onnx-v${SHERPA_VERSION}-cuda-12.x-cudnn-9.x-linux-x64-gpu/lib/*.so* /usr/local/lib/ && \
    rm -rf sherpa-onnx-v${SHERPA_VERSION}-cuda-12.x-cudnn-9.x-linux-x64-gpu && \
    ldconfig

# whisper.cpp with CUDA
ARG WHISPER_VERSION=1.8.2

RUN git clone --depth 1 --branch v${WHISPER_VERSION} https://github.com/ggerganov/whisper.cpp /tmp/whisper.cpp && \
    cd /tmp/whisper.cpp && \
    cmake -B build \
        -DCMAKE_BUILD_TYPE=Release \
        -DBUILD_SHARED_LIBS=ON \
        -DGGML_NATIVE=OFF \
        -DGGML_CUDA=ON \
        -DWHISPER_BUILD_EXAMPLES=OFF \
        -DWHISPER_BUILD_TESTS=OFF && \
    cmake --build build --config Release -j$(nproc) && \
    cp build/src/libwhisper.so* /usr/local/lib/ && \
    cp build/ggml/src/libggml*.so* /usr/local/lib/ && \
    cp include/whisper.h /usr/local/include/ && \
    cp ggml/include/ggml*.h /usr/local/include/ && \
    rm -rf /tmp/whisper.cpp && \
    ldconfig

# Set environment for Go builds
ENV CGO_ENABLED=1
ENV LD_LIBRARY_PATH=/usr/local/lib
ENV C_INCLUDE_PATH=/usr/local/include
ENV LIBRARY_PATH=/usr/local/lib
