# vget Docker Image

# Build stage for UI
FROM node:22-slim AS ui-builder
WORKDIR /app/ui
COPY ui/package*.json ./
RUN npm ci
COPY ui/ ./
RUN npm run build

# Go builder stage
FROM golang:1.25-bookworm AS go-builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=ui-builder /app/ui/dist ./internal/server/dist

RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /vget-server ./cmd/vget-server

# Runtime stage
FROM debian:bookworm-slim

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
    mkdir -p /home/vget/downloads /home/vget/.config/vget && \
    chown -R 1000:1000 /home/vget

COPY --from=go-builder /vget-server /usr/local/bin/vget-server

ENV ROD_BROWSER=/usr/bin/chromium

COPY docker/vget/entrypoint.sh /usr/local/bin/
RUN chmod +x /usr/local/bin/entrypoint.sh

WORKDIR /home/vget
EXPOSE 8080
VOLUME ["/home/vget/downloads", "/home/vget/.config/vget"]

ENTRYPOINT ["entrypoint.sh"]
CMD []
