# ---------- build stage ----------
FROM golang:1.24-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/video_worker/ ./cmd/video_worker/
COPY internal/ ./internal/
RUN CGO_ENABLED=0 go build -o /bin/video_worker ./cmd/video_worker

# ---------- runtime stage ----------
FROM jrottenberg/ffmpeg:6.1-alpine
COPY --from=builder /bin/video_worker /usr/local/bin/video_worker
ENTRYPOINT ["video_worker"]
