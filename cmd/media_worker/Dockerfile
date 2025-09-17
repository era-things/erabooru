# ---------- build stage ----------
FROM golang:1.25-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/media_worker/ ./cmd/media_worker/
COPY internal/ ./internal/
COPY ent/ ./ent/
RUN CGO_ENABLED=0 go build -o /bin/media_worker ./cmd/media_worker

# ---------- runtime stage ----------
FROM jrottenberg/ffmpeg:6.1-alpine
COPY --from=builder /bin/media_worker /usr/local/bin/media_worker
ENTRYPOINT ["media_worker"]
