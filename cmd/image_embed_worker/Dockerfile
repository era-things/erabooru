# ---------- build stage ----------
FROM golang:1.25-alpine AS builder

# Install build dependencies for CGO (required for onnxruntime_go)
RUN apk add --no-cache gcc musl-dev

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/image_embed_worker/ ./cmd/image_embed_worker/
COPY internal/ ./internal/
COPY ent/ ./ent/

# Build with CGO enabled (required for onnxruntime_go)
RUN CGO_ENABLED=1 GOOS=linux go build -a -ldflags '-linkmode external -extldflags "-static"' -o /bin/image_embed_worker ./cmd/image_embed_worker

# ---------- runtime stage ----------
FROM alpine:3.20

# Install runtime dependencies for CGO binaries
RUN apk add --no-cache libc6-compat libgcc libstdc++

COPY --from=builder /bin/image_embed_worker /usr/local/bin/image_embed_worker
ENTRYPOINT ["image_embed_worker"]