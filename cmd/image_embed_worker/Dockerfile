# ---------- common build args ----------
ARG ONNXRUNTIME_VERSION=1.22.0
ARG LIBVIPS_VERSION=8.17.2
ARG GO_VERSION=1.25.0

# ---------- CPU builder ----------
FROM golang:1.25-bookworm AS cpu-builder
ARG TARGETARCH
ARG ONNXRUNTIME_VERSION
ARG LIBVIPS_VERSION

RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential ca-certificates pkg-config git meson ninja-build python3 \
    libglib2.0-dev libexpat1-dev zlib1g-dev libjpeg62-turbo-dev libpng-dev \
    libtiff-dev libwebp-dev libexif-dev liborc-0.4-dev wget \
 && rm -rf /var/lib/apt/lists/*
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
ENV PKG_CONFIG_PATH="/opt/libvips/lib/pkgconfig:/opt/libvips/lib/x86_64-linux-gnu/pkgconfig:/opt/libvips/lib/aarch64-linux-gnu/pkgconfig:${PKG_CONFIG_PATH}"
ENV LD_LIBRARY_PATH="/opt/libvips/lib:/opt/libvips/lib/x86_64-linux-gnu:/opt/libvips/lib/aarch64-linux-gnu:${LD_LIBRARY_PATH}"
RUN pkg-config --modversion vips && pkg-config --cflags --libs vips

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

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
    rm -rf "${onnx_dir}"*

COPY cmd/image_embed_worker/ ./cmd/image_embed_worker/
COPY internal/ ./internal/
COPY ent/ ./ent/

ENV CGO_CFLAGS="-I/opt/onnxruntime/include"
ENV CGO_LDFLAGS="-L/opt/onnxruntime/lib -lonnxruntime"
ENV LD_LIBRARY_PATH="/opt/onnxruntime/lib:/opt/libvips/lib:/opt/libvips/lib/x86_64-linux-gnu:/opt/libvips/lib/aarch64-linux-gnu:${LD_LIBRARY_PATH}"

RUN CGO_ENABLED=1 go build -tags="embeddings" -trimpath -ldflags="-s -w" -o /bin/image_embed_worker ./cmd/image_embed_worker \
 && strip /bin/image_embed_worker || true
RUN find /opt/libvips -type f -name "*.so*" -exec sh -c 'for f; do strip --strip-unneeded "$f" || true; done' sh {} +

# ---------- GPU builder ----------
FROM nvidia/cuda:12.4.1-cudnn-devel-ubuntu22.04 AS gpu-builder
ARG TARGETARCH
ARG ONNXRUNTIME_VERSION
ARG LIBVIPS_VERSION
ARG GO_VERSION

RUN if [ "${TARGETARCH:-amd64}" != "amd64" ]; then echo "gpu-cuda12 target requires linux/amd64" >&2; exit 1; fi

RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential ca-certificates pkg-config git meson ninja-build python3 \
    curl libglib2.0-dev libexpat1-dev zlib1g-dev libjpeg-turbo8-dev libpng-dev \
    libtiff-dev libwebp-dev libexif-dev liborc-0.4-dev wget \
 && rm -rf /var/lib/apt/lists/*

RUN curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" -o /tmp/go.tar.gz \
 && tar -C /usr/local -xzf /tmp/go.tar.gz \
 && rm /tmp/go.tar.gz
ENV GOROOT=/usr/local/go
ENV GOPATH=/go
ENV PATH="${GOROOT}/bin:${GOPATH}/bin:${PATH}"

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
ENV PKG_CONFIG_PATH="/opt/libvips/lib/pkgconfig:/opt/libvips/lib/x86_64-linux-gnu/pkgconfig:${PKG_CONFIG_PATH}"
ENV LD_LIBRARY_PATH="/opt/libvips/lib:/opt/libvips/lib/x86_64-linux-gnu:${LD_LIBRARY_PATH}"
RUN pkg-config --modversion vips && pkg-config --cflags --libs vips

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

RUN set -eux; \
    onnx_pkg="onnxruntime-linux-x64-gpu-${ONNXRUNTIME_VERSION}.tgz"; \
    wget -q "https://github.com/microsoft/onnxruntime/releases/download/v${ONNXRUNTIME_VERSION}/${onnx_pkg}"; \
    tar -xzf "${onnx_pkg}"; \
    onnx_dir="${onnx_pkg%.tgz}"; \
    mkdir -p /opt/onnxruntime; \
    cp -r "${onnx_dir}/"* /opt/onnxruntime/; \
    ln -sf /opt/onnxruntime/lib/libonnxruntime.so.${ONNXRUNTIME_VERSION} /opt/onnxruntime/lib/onnxruntime.so; \
    rm -rf "${onnx_dir}"*

COPY cmd/image_embed_worker/ ./cmd/image_embed_worker/
COPY internal/ ./internal/
COPY ent/ ./ent/

ENV CGO_CFLAGS="-I/opt/onnxruntime/include"
ENV CGO_LDFLAGS="-L/opt/onnxruntime/lib -lonnxruntime"
ENV LD_LIBRARY_PATH="/opt/onnxruntime/lib:/opt/libvips/lib:/opt/libvips/lib/x86_64-linux-gnu:${LD_LIBRARY_PATH}"

RUN CGO_ENABLED=1 go build -tags="embeddings" -trimpath -ldflags="-s -w" -o /bin/image_embed_worker ./cmd/image_embed_worker \
 && strip /bin/image_embed_worker || true
RUN find /opt/libvips -type f -name "*.so*" -exec sh -c 'for f; do strip --strip-unneeded "$f" || true; done' sh {} +

# ---------- CPU runtime ----------
FROM debian:bookworm-slim AS cpu
RUN set -eux; \
    apt-get update; \
    apt-get install -y --no-install-recommends \
        ca-certificates ffmpeg libc6 libgcc-s1 libstdc++6 \
        liborc-0.4-0 libjpeg62-turbo libpng16-16 libtiff6 \
        libwebp7 libwebpdemux2 libwebpmux3 libexif12 wget; \
    rm -rf /var/lib/apt/lists/*
COPY --from=cpu-builder /opt/libvips/lib /opt/libvips/lib
COPY --from=cpu-builder /opt/libvips/share /opt/libvips/share
COPY --from=cpu-builder /opt/onnxruntime /opt/onnxruntime
ENV LD_LIBRARY_PATH="/opt/onnxruntime/lib:/opt/libvips/lib:/opt/libvips/lib/x86_64-linux-gnu:/opt/libvips/lib/aarch64-linux-gnu:${LD_LIBRARY_PATH}"
COPY --from=cpu-builder /bin/image_embed_worker /usr/local/bin/image_embed_worker
ENTRYPOINT ["image_embed_worker"]

# ---------- GPU runtime ----------
FROM nvidia/cuda:12.4.1-cudnn-runtime-ubuntu22.04 AS gpu-cuda12
RUN set -eux; \
    apt-get update; \
    apt-get install -y --no-install-recommends \
        ca-certificates ffmpeg libgcc-s1 libstdc++6 \
        libjpeg-turbo8 libpng16-16 libtiff5 \
        libwebp7 libwebpdemux2 libwebpmux3 libexif12 liborc-0.4-0 wget; \
    rm -rf /var/lib/apt/lists/*
COPY --from=gpu-builder /opt/libvips/lib /opt/libvips/lib
COPY --from=gpu-builder /opt/libvips/share /opt/libvips/share
COPY --from=gpu-builder /opt/onnxruntime /opt/onnxruntime
ENV LD_LIBRARY_PATH="/opt/onnxruntime/lib:/opt/libvips/lib:/opt/libvips/lib/x86_64-linux-gnu:${LD_LIBRARY_PATH}"
COPY --from=gpu-builder /bin/image_embed_worker /usr/local/bin/image_embed_worker
ENTRYPOINT ["image_embed_worker"]
