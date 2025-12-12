# Build stage for UI
FROM node:22-slim AS ui-builder
WORKDIR /app/ui
COPY ui/package*.json ./
RUN npm ci
COPY ui/ ./
RUN npm run build

# Build stage for Go binary
FROM golang:1.25-bookworm AS go-builder
WORKDIR /app

# Copy go mod files first for caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Copy built UI into embed location
COPY --from=ui-builder /app/ui/dist ./internal/server/dist

# Build the server binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /vget-server ./cmd/vget-server

# Final runtime stage
FROM debian:bookworm-slim

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
    && rm -rf /var/lib/apt/lists/*

# Install yt-dlp and youtube-dl
RUN pip3 install --no-cache-dir --break-system-packages \
    yt-dlp \
    youtube-dl

# Create non-root user
RUN groupadd -g 1000 vget && \
    useradd -u 1000 -g vget -m -d /home/vget vget && \
    mkdir -p /home/vget/downloads /home/vget/.config/vget && \
    chown -R vget:vget /home/vget

# Copy binary from builder
COPY --from=go-builder /vget-server /usr/local/bin/vget-server

# Tell rod to use system chromium instead of downloading
ENV ROD_BROWSER=/usr/bin/chromium

# Copy entrypoint script
COPY docker/entrypoint.sh /usr/local/bin/
RUN chmod +x /usr/local/bin/entrypoint.sh

WORKDIR /home/vget

EXPOSE 8080

VOLUME ["/home/vget/downloads", "/home/vget/.config/vget"]

ENTRYPOINT ["entrypoint.sh"]
CMD []
