# ---------- build stage ----------
FROM golang:1.25-bookworm AS builder

# Install build dependencies for CGO
RUN apt-get update && apt-get install -y gcc libc6-dev wget build-essential && rm -rf /var/lib/apt/lists/*

ARG TARGETARCH
ARG ONNXRUNTIME_VERSION=1.22.0
# Install ONNX Runtime 1.22.0
RUN set -eux; \
    case "${TARGETARCH}" in \
        amd64) onnx_pkg="onnxruntime-linux-x64-${ONNXRUNTIME_VERSION}.tgz" ;; \
        arm64) onnx_pkg="onnxruntime-linux-aarch64-${ONNXRUNTIME_VERSION}.tgz" ;; \
        *) echo "Unsupported architecture: ${TARGETARCH}" >&2; exit 1 ;; \
    esac; \
    wget -q "https://github.com/microsoft/onnxruntime/releases/download/v${ONNXRUNTIME_VERSION}/${onnx_pkg}"; \
    tar -xzf "${onnx_pkg}"; \
    onnx_dir="${onnx_pkg%.tgz}"; \
    mkdir -p /opt/onnxruntime; \
    cp -r "${onnx_dir}/"* /opt/onnxruntime/; \
    ln -sf /opt/onnxruntime/lib/libonnxruntime.so.${ONNXRUNTIME_VERSION} /opt/onnxruntime/lib/onnxruntime.so; \
    echo "/opt/onnxruntime/lib" > /etc/ld.so.conf.d/onnxruntime.conf; \
    ldconfig; \
    rm -rf "${onnx_dir}"*
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/image_embed_worker/ ./cmd/image_embed_worker/
COPY internal/ ./internal/
COPY ent/ ./ent/

# Build with CGO enabled and explicit paths
ENV CGO_CFLAGS="-I/opt/onnxruntime/include"
ENV CGO_LDFLAGS="-L/opt/onnxruntime/lib -lonnxruntime"
ENV LD_LIBRARY_PATH="/opt/onnxruntime/lib"
RUN CGO_ENABLED=1 go build -tags="embeddings" -o /bin/image_embed_worker ./cmd/image_embed_worker

# ---------- runtime stage ----------
FROM debian:bookworm-slim

# Install runtime dependencies
RUN apt-get update && apt-get install -y \
    ca-certificates \
    ffmpeg \
    libc6 \
    libgcc-s1 \
    libstdc++6 \
    wget \
    && rm -rf /var/lib/apt/lists/*

ARG TARGETARCH
ARG ONNXRUNTIME_VERSION=1.22.0
# Install ONNX Runtime 1.22.0 in runtime stage
RUN set -eux; \
    case "${TARGETARCH}" in \
        amd64) onnx_pkg="onnxruntime-linux-x64-${ONNXRUNTIME_VERSION}.tgz" ;; \
        arm64) onnx_pkg="onnxruntime-linux-aarch64-${ONNXRUNTIME_VERSION}.tgz" ;; \
        *) echo "Unsupported architecture: ${TARGETARCH}" >&2; exit 1 ;; \
    esac; \
    wget -q "https://github.com/microsoft/onnxruntime/releases/download/v${ONNXRUNTIME_VERSION}/${onnx_pkg}"; \
    tar -xzf "${onnx_pkg}"; \
    onnx_dir="${onnx_pkg%.tgz}"; \
    mkdir -p /opt/onnxruntime; \
    cp -r "${onnx_dir}/"* /opt/onnxruntime/; \
    ln -sf /opt/onnxruntime/lib/libonnxruntime.so.${ONNXRUNTIME_VERSION} /opt/onnxruntime/lib/onnxruntime.so; \
    echo "/opt/onnxruntime/lib" > /etc/ld.so.conf.d/onnxruntime.conf; \
    ldconfig; \
    rm -rf "${onnx_dir}"*

ENV LD_LIBRARY_PATH="/opt/onnxruntime/lib"

COPY --from=builder /bin/image_embed_worker /usr/local/bin/image_embed_worker

ENTRYPOINT ["image_embed_worker"]