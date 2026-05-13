FROM golang:1.24 AS builder

WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/douyin-nas-monitor ./cmd/douyin-nas-monitor

FROM python:3.13-slim-bookworm

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    ffmpeg \
    sqlite3 \
    && python -m pip install --no-cache-dir -U "yt-dlp[default]" \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=builder /out/douyin-nas-monitor /usr/local/bin/douyin-nas-monitor

EXPOSE 3456

CMD ["douyin-nas-monitor", "--config", "/app/config.yaml", "--web", "--addr", ":3456"]
