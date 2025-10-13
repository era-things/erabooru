# ---------- build stage ----------
FROM golang:1.25-bookworm AS builder

ARG TARGETARCH
ARG ONNXRUNTIME_VERSION=1.22.0
ARG LIBVIPS_VERSION=8.17.2

# CGO + libvips build deps (no pip/gettext)
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential ca-certificates pkg-config git meson ninja-build python3 \
    libglib2.0-dev libexpat1-dev zlib1g-dev libjpeg62-turbo-dev libpng-dev \
    libtiff-dev libwebp-dev libexif-dev liborc-0.4-dev wget \
 && rm -rf /var/lib/apt/lists/*

# ---- Build libvips 8.17.x into /opt/libvips ----
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
      -Djpeg-xl=disabled \
      -Db_lto=true -Db_ndebug=true; \
    meson compile -C build; \
    meson install -C build; \
    cd /; rm -rf /tmp/libvips

# Make pkg-config and linker see our libvips
ENV PKG_CONFIG_PATH="/opt/libvips/lib/pkgconfig:/opt/libvips/lib/x86_64-linux-gnu/pkgconfig:${PKG_CONFIG_PATH}"
ENV LD_LIBRARY_PATH="/opt/libvips/lib:/opt/libvips/lib/x86_64-linux-gnu:${LD_LIBRARY_PATH}"
RUN pkg-config --modversion vips && pkg-config --cflags --libs vips

# ---- Multi-arch ONNX Runtime with GPU support for amd64 ----
RUN set -eux; \
    case "${TARGETARCH}" in \
        amd64) onnx_pkg="onnxruntime-linux-x64-gpu-${ONNXRUNTIME_VERSION}.tgz" ;; \
        arm64) onnx_pkg="onnxruntime-linux-aarch64-${ONNXRUNTIME_VERSION}.tgz" ;; \
        *) echo "Unsupported architecture: ${TARGETARCH}" >&2; exit 1 ;; \
    esac; \
    wget -q "https://github.com/microsoft/onnxruntime/releases/download/v${ONNXRUNTIME_VERSION}/${onnx_pkg}"; \
    tar -xzf "${onnx_pkg}"; \
    onnx_dir="${onnx_pkg%.tgz}"; \
    mkdir -p /opt/onnxruntime; \
    cp -r "${onnx_dir}/"* /opt/onnxruntime/; \
    ln -sf /opt/onnxruntime/lib/libonnxruntime.so.${ONNXRUNTIME_VERSION} /opt/onnxruntime/lib/onnxruntime.so; \
    rm -rf "${onnx_dir}"*
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

# Build (smaller binary)
RUN CGO_ENABLED=1 go build -tags="embeddings" -trimpath -ldflags="-s -w" -o /bin/image_embed_worker ./cmd/image_embed_worker \
 && strip /bin/image_embed_worker || true

# Optional: strip libvips *.so to shave a few MB
RUN find /opt/libvips -type f -name "*.so*" -exec sh -c 'for f; do strip --strip-unneeded "$f" || true; done' sh {} +

# ---------- runtime stage ----------
FROM debian:bookworm-slim

# Runtime libs (match codecs enabled above) + CUDA runtime for ONNX Runtime GPU
RUN set -eux; \
    apt-get update; \
    apt-get install -y --no-install-recommends \
        ca-certificates ffmpeg libc6 libgcc-s1 libstdc++6 \
        liborc-0.4-0 libjpeg62-turbo libpng16-16 libtiff6 \
        libwebp7 libwebpdemux2 libwebpmux3 libexif12 wget; \
    wget -q https://developer.download.nvidia.com/compute/cuda/repos/debian12/x86_64/cuda-keyring_1.1-1_all.deb; \
    dpkg -i cuda-keyring_1.1-1_all.deb; \
    rm cuda-keyring_1.1-1_all.deb; \
    apt-get update; \
    apt-get install -y --no-install-recommends \
        cuda-compat-12-4 \
        cuda-libraries-12-4 \
        libcudnn9-cuda-12; \
    rm -rf /var/lib/apt/lists/*

# Copy only libvips runtime bits (no headers)
COPY --from=builder /opt/libvips/lib /opt/libvips/lib
COPY --from=builder /opt/libvips/share /opt/libvips/share

# Copy ONNX Runtime from builder (includes the onnxruntime.so alias)
COPY --from=builder /opt/onnxruntime /opt/onnxruntime

# Runtime linker paths
ENV LD_LIBRARY_PATH="/opt/onnxruntime/lib:/opt/libvips/lib:/opt/libvips/lib/x86_64-linux-gnu:/usr/local/cuda/lib64:/usr/local/cuda-12.4/lib64:${LD_LIBRARY_PATH}"

# App binary
COPY --from=builder /bin/image_embed_worker /usr/local/bin/image_embed_worker

ENTRYPOINT ["image_embed_worker"]
