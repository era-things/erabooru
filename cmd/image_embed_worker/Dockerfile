# ---------- build stage ----------
FROM golang:1.25-bookworm AS builder

# Install build dependencies for CGO
RUN apt-get update && apt-get install -y gcc libc6-dev wget build-essential && rm -rf /var/lib/apt/lists/*

# Install ONNX Runtime 1.22.0
RUN wget -q https://github.com/microsoft/onnxruntime/releases/download/v1.22.0/onnxruntime-linux-x64-1.22.0.tgz \
    && tar -xzf onnxruntime-linux-x64-1.22.0.tgz \
    && mkdir -p /opt/onnxruntime \
    && cp -r onnxruntime-linux-x64-1.22.0/* /opt/onnxruntime/ \
    && ln -sf /opt/onnxruntime/lib/libonnxruntime.so.1.22.0 /opt/onnxruntime/lib/onnxruntime.so \
    && echo "/opt/onnxruntime/lib" > /etc/ld.so.conf.d/onnxruntime.conf \
    && ldconfig \
    && rm -rf onnxruntime-linux-x64-1.22.0*

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
    libc6 \
    libgcc-s1 \
    libstdc++6 \
    wget \
    && rm -rf /var/lib/apt/lists/*

# Install ONNX Runtime 1.22.0 in runtime stage
RUN wget -q https://github.com/microsoft/onnxruntime/releases/download/v1.22.0/onnxruntime-linux-x64-1.22.0.tgz \
    && tar -xzf onnxruntime-linux-x64-1.22.0.tgz \
    && mkdir -p /opt/onnxruntime \
    && cp -r onnxruntime-linux-x64-1.22.0/* /opt/onnxruntime/ \
    && ln -sf /opt/onnxruntime/lib/libonnxruntime.so.1.22.0 /opt/onnxruntime/lib/onnxruntime.so \
    && echo "/opt/onnxruntime/lib" > /etc/ld.so.conf.d/onnxruntime.conf \
    && ldconfig \
    && rm -rf onnxruntime-linux-x64-1.22.0*

ENV LD_LIBRARY_PATH="/opt/onnxruntime/lib"

COPY --from=builder /bin/image_embed_worker /usr/local/bin/image_embed_worker

ENTRYPOINT ["image_embed_worker"]