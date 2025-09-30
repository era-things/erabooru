# ---------- build stage ----------
FROM golang:1.25-bookworm AS builder

ARG ONNXRUNTIME_VERSION=1.22.0
ARG LIBVIPS_VERSION=8.17.2
ARG LIBVIPS_PACKAGE=libvips-${LIBVIPS_VERSION}-linux-x64.tar.gz

# Install build dependencies for CGO and download helpers
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    ca-certificates \
    pkg-config \
    wget \
 && rm -rf /var/lib/apt/lists/*

# Install libvips from the official prebuilt package so we match the
# generated bindings shipped with github.com/cshum/vipsgen (libvips 8.17).
RUN wget -q https://github.com/kleisauke/libvips-packaging/releases/download/v${LIBVIPS_VERSION}/${LIBVIPS_PACKAGE} \
    && mkdir -p /opt \
    && tar -xzf ${LIBVIPS_PACKAGE} -C /opt \
    && mv /opt/libvips-${LIBVIPS_VERSION}-linux-x64 /opt/libvips \
    && rm ${LIBVIPS_PACKAGE}

ENV PKG_CONFIG_PATH="/opt/libvips/lib/pkgconfig:${PKG_CONFIG_PATH}"
ENV LD_LIBRARY_PATH="/opt/libvips/lib:${LD_LIBRARY_PATH}"

# Install ONNX Runtime
RUN wget -q https://github.com/microsoft/onnxruntime/releases/download/v${ONNXRUNTIME_VERSION}/onnxruntime-linux-x64-${ONNXRUNTIME_VERSION}.tgz \
    && tar -xzf onnxruntime-linux-x64-${ONNXRUNTIME_VERSION}.tgz \
    && mkdir -p /opt/onnxruntime \
    && cp -r onnxruntime-linux-x64-${ONNXRUNTIME_VERSION}/* /opt/onnxruntime/ \
    && ln -sf /opt/onnxruntime/lib/libonnxruntime.so.${ONNXRUNTIME_VERSION} /opt/onnxruntime/lib/onnxruntime.so \
    && echo "/opt/onnxruntime/lib" > /etc/ld.so.conf.d/onnxruntime.conf \
    && ldconfig \
    && rm -rf onnxruntime-linux-x64-${ONNXRUNTIME_VERSION}*

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/image_embed_worker/ ./cmd/image_embed_worker/
COPY internal/ ./internal/
COPY ent/ ./ent/

# Build with CGO enabled and explicit paths
ENV CGO_CFLAGS="-I/opt/onnxruntime/include"
ENV CGO_LDFLAGS="-L/opt/onnxruntime/lib -lonnxruntime"
ENV LD_LIBRARY_PATH="/opt/onnxruntime/lib:/opt/libvips/lib"
RUN CGO_ENABLED=1 go build -tags="embeddings" -o /bin/image_embed_worker ./cmd/image_embed_worker

# ---------- runtime stage ----------
FROM debian:bookworm-slim

ARG ONNXRUNTIME_VERSION=1.22.0
ARG LIBVIPS_VERSION=8.17.2
ARG LIBVIPS_PACKAGE=libvips-${LIBVIPS_VERSION}-linux-x64.tar.gz

# Install runtime dependencies and download helpers
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    ffmpeg \
    libc6 \
    libgcc-s1 \
    libstdc++6 \
    wget \
 && rm -rf /var/lib/apt/lists/*

# Install libvips runtime libraries to match the builder stage
RUN wget -q https://github.com/kleisauke/libvips-packaging/releases/download/v${LIBVIPS_VERSION}/${LIBVIPS_PACKAGE} \
    && mkdir -p /opt \
    && tar -xzf ${LIBVIPS_PACKAGE} -C /opt \
    && mv /opt/libvips-${LIBVIPS_VERSION}-linux-x64 /opt/libvips \
    && rm ${LIBVIPS_PACKAGE}

# Install ONNX Runtime in runtime stage
RUN wget -q https://github.com/microsoft/onnxruntime/releases/download/v${ONNXRUNTIME_VERSION}/onnxruntime-linux-x64-${ONNXRUNTIME_VERSION}.tgz \
    && tar -xzf onnxruntime-linux-x64-${ONNXRUNTIME_VERSION}.tgz \
    && mkdir -p /opt/onnxruntime \
    && cp -r onnxruntime-linux-x64-${ONNXRUNTIME_VERSION}/* /opt/onnxruntime/ \
    && ln -sf /opt/onnxruntime/lib/libonnxruntime.so.${ONNXRUNTIME_VERSION} /opt/onnxruntime/lib/onnxruntime.so \
    && echo "/opt/onnxruntime/lib" > /etc/ld.so.conf.d/onnxruntime.conf \
    && ldconfig \
    && rm -rf onnxruntime-linux-x64-${ONNXRUNTIME_VERSION}*

ENV LD_LIBRARY_PATH="/opt/onnxruntime/lib:/opt/libvips/lib"

COPY --from=builder /bin/image_embed_worker /usr/local/bin/image_embed_worker

ENTRYPOINT ["image_embed_worker"]
