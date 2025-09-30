# ---------- build stage ----------
FROM golang:1.25-bookworm AS builder

ARG ONNXRUNTIME_VERSION=1.22.0
ARG LIBVIPS_VERSION=8.17.2

# CGO + libvips build deps
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    ca-certificates \
    pkg-config \
    git \
    meson \
    ninja-build \
    python3 \
    python3-pip \
    gettext \
    # vips deps
    libglib2.0-dev \
    libexpat1-dev \
    zlib1g-dev \
    libjpeg62-turbo-dev \
    libpng-dev \
    libtiff-dev \
    libwebp-dev \
    libexif-dev \
    liborc-0.4-dev \
    wget \
 && rm -rf /var/lib/apt/lists/*

# ---- Build libvips 8.17.x from source into /opt/libvips ----
RUN set -eux; \
    git clone --depth 1 --branch v${LIBVIPS_VERSION} https://github.com/libvips/libvips.git /tmp/libvips; \
    cd /tmp/libvips; \
    meson setup build \
      --prefix=/opt/libvips \
      -Dexamples=false \
      -Dintrospection=disabled \
      -Ddeprecated=true \
      -Dmodules=enabled \
      -Dmagick=disabled \
      -Dopenslide=disabled \
      -Dpdfium=disabled \
      -Dpoppler=disabled \
      -Dcfitsio=disabled \
      -Djpeg-xl=disabled; \
    meson compile -C build; \
    meson install -C build; \
    cd /; rm -rf /tmp/libvips

# Make pkg-config and linker see our libvips
ENV PKG_CONFIG_PATH="/opt/libvips/lib/pkgconfig:/opt/libvips/lib/x86_64-linux-gnu/pkgconfig:${PKG_CONFIG_PATH}"
ENV LD_LIBRARY_PATH="/opt/libvips/lib:/opt/libvips/lib/x86_64-linux-gnu:${LD_LIBRARY_PATH}"

RUN pkg-config --modversion vips && pkg-config --cflags --libs vips

# ---- ONNX Runtime (CPU) ----
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

# Copy sources
COPY cmd/image_embed_worker/ ./cmd/image_embed_worker/
COPY internal/ ./internal/
COPY ent/ ./ent/

# CGO flags for ONNX Runtime
ENV CGO_CFLAGS="-I/opt/onnxruntime/include"
ENV CGO_LDFLAGS="-L/opt/onnxruntime/lib -lonnxruntime"
ENV LD_LIBRARY_PATH="/opt/onnxruntime/lib:/opt/libvips/lib:/opt/libvips/lib/x86_64-linux-gnu:${LD_LIBRARY_PATH}"

# Build
RUN CGO_ENABLED=1 go build -tags="embeddings" -o /bin/image_embed_worker ./cmd/image_embed_worker

# ---------- runtime stage ----------
FROM debian:bookworm-slim

ARG ONNXRUNTIME_VERSION=1.22.0

# Runtime libs (match codecs enabled above)
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    ffmpeg \
    libc6 \
    libgcc-s1 \
    libstdc++6 \
    liborc-0.4-0 \
    libjpeg62-turbo \
    libpng16-16 \
    libtiff6 \
    libwebp7 \
    libwebpdemux2 \
    libwebpmux3 \
    libexif12 \
    wget \
 && rm -rf /var/lib/apt/lists/*

# Bring the libvips we built
COPY --from=builder /opt/libvips /opt/libvips

# ---- ONNX Runtime runtime ----
RUN wget -q https://github.com/microsoft/onnxruntime/releases/download/v${ONNXRUNTIME_VERSION}/onnxruntime-linux-x64-${ONNXRUNTIME_VERSION}.tgz \
    && tar -xzf onnxruntime-linux-x64-${ONNXRUNTIME_VERSION}.tgz \
    && mkdir -p /opt/onnxruntime \
    && cp -r onnxruntime-linux-x64-${ONNXRUNTIME_VERSION}/* /opt/onnxruntime/ \
    && ln -sf /opt/onnxruntime/lib/libonnxruntime.so.${ONNXRUNTIME_VERSION} /opt/onnxruntime/lib/onnxruntime.so \
    && echo "/opt/onnxruntime/lib" > /etc/ld.so.conf.d/onnxruntime.conf \
    && ldconfig \
    && rm -rf onnxruntime-linux-x64-${ONNXRUNTIME_VERSION}*

ENV LD_LIBRARY_PATH="/opt/onnxruntime/lib:/opt/libvips/lib:/opt/libvips/lib/x86_64-linux-gnu:${LD_LIBRARY_PATH}"

COPY --from=builder /bin/image_embed_worker /usr/local/bin/image_embed_worker
ENTRYPOINT ["image_embed_worker"]
