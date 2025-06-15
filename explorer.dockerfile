# ──────────────────────────────────────────────────────────────
#  Stage 1 – build the bleve-explorer binary from source
# ──────────────────────────────────────────────────────────────
FROM golang:1.24-alpine AS build
RUN apk add --no-cache git
RUN go install github.com/blevesearch/bleve-explorer@latest

# ──────────────────────────────────────────────────────────────
#  Stage 2 – minimal runtime image (≈ 8 MB)
# ──────────────────────────────────────────────────────────────
FROM alpine:3.20
COPY --from=build /go/bin/bleve-explorer /usr/local/bin/bleve-explorer
EXPOSE 8095
WORKDIR /
ENTRYPOINT ["bleve-explorer", "-dataDir", "/data", "-addr", ":8095"]
