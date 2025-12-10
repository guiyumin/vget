# Build stage for UI
FROM node:22-alpine AS ui-builder
WORKDIR /app/ui
COPY ui/package*.json ./
RUN npm ci
COPY ui/ ./
RUN npm run build

# Build stage for Go binary
FROM golang:1.25.4-alpine AS go-builder
WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Copy go mod files first for caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Copy built UI into embed location
COPY --from=ui-builder /app/ui/dist ./internal/server/dist

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /vget ./cmd/vget

# Final runtime stage
FROM alpine:3.21

# Install runtime dependencies
# - ca-certificates: for HTTPS requests
# - chromium: for browser-based extractors (XHS, etc.)
# - font packages: for proper text rendering in headless browser
# - python3/pip: for yt-dlp and youtube-dl
# - ffmpeg: for merging video/audio streams
# - nodejs: for yt-dlp JS challenge solving (N parameter)
RUN apk add --no-cache \
    ca-certificates \
    chromium \
    font-noto-cjk \
    font-noto-emoji \
    tzdata \
    su-exec \
    python3 \
    py3-pip \
    ffmpeg \
    nodejs

# Install yt-dlp and youtube-dl
RUN pip3 install --no-cache-dir --break-system-packages \
    yt-dlp \
    youtube-dl

# Create non-root user
RUN addgroup -g 1000 vget && \
    adduser -u 1000 -G vget -h /home/vget -D vget && \
    mkdir -p /home/vget/downloads /home/vget/.config/vget && \
    chown -R vget:vget /home/vget

# Copy binary from builder
COPY --from=go-builder /vget /usr/local/bin/vget

# Tell rod to use system chromium instead of downloading
ENV ROD_BROWSER=/usr/bin/chromium-browser

# Copy entrypoint script
COPY docker/entrypoint.sh /usr/local/bin/
RUN chmod +x /usr/local/bin/entrypoint.sh

WORKDIR /home/vget

EXPOSE 8080

VOLUME ["/home/vget/downloads", "/home/vget/.config/vget"]

ENTRYPOINT ["entrypoint.sh"]
CMD ["server", "start"]
