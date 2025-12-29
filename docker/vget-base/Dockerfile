# vget Base Image - Go 1.25 with whisper.cpp pre-built
# Pre-compiled with libwhisper.a for CGO bindings
# Published to ghcr.io/guiyumin/vget-base:latest

FROM golang:1.25-bookworm

LABEL org.opencontainers.image.source=https://github.com/guiyumin/vget
LABEL org.opencontainers.image.description="vget Base - Go 1.25 with whisper.cpp for local speech-to-text"
LABEL org.opencontainers.image.licenses=MIT

# Install build dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    git \
    && rm -rf /var/lib/apt/lists/*

# Clone and build whisper.cpp
# We build the library in a known location for easy reference
WORKDIR /opt/whisper.cpp
RUN git clone --depth 1 https://github.com/ggerganov/whisper.cpp.git . && \
    cd bindings/go && \
    make whisper && \
    ls -la libwhisper.a

# Set environment variables for CGO to find whisper.cpp
ENV WHISPER_PATH=/opt/whisper.cpp/bindings/go
ENV C_INCLUDE_PATH=/opt/whisper.cpp/bindings/go
ENV LIBRARY_PATH=/opt/whisper.cpp/bindings/go
ENV CGO_ENABLED=1

# Reset workdir
WORKDIR /app

# Metadata
ENV BASE_IMAGE_VERSION=1.0.0
