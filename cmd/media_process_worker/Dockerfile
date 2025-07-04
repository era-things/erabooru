# ---------- build stage ----------
FROM golang:1.24-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/media_process_worker/ ./cmd/media_process_worker/
COPY internal/ ./internal/
RUN CGO_ENABLED=0 go build -o /bin/media_process_worker ./cmd/media_process_worker

# ---------- runtime stage ----------
FROM jrottenberg/ffmpeg:6.1-alpine
COPY --from=builder /bin/media_process_worker /usr/local/bin/media_process_worker
ENTRYPOINT ["media_process_worker"]
