# syntax=docker/dockerfile:1

# ----- build UI -----
FROM node:20-bookworm AS ui-build
WORKDIR /src/web
COPY web/package*.json ./
RUN npm ci
COPY web .
RUN npm run build

# ----- build Go server with embedded assets -----
FROM golang:1.24-bookworm AS server-build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN rm -rf internal/assets/build
COPY --from=ui-build /src/web/build ./internal/assets/build
RUN CGO_ENABLED=0 go build -o /erabooru ./cmd/server
RUN CGO_ENABLED=0 go build -o /healthcheck ./cmd/healthcheck

# Create the data directory structure in build stage
RUN mkdir -p /tmp/data/bleve && chown -R 65532:65532 /tmp/data

# ----- dev stage with Vite HMR -----
FROM golang:1.24-bookworm AS dev
RUN apt-get update && apt-get install -y curl && \
    curl -fsSL https://deb.nodesource.com/setup_20.x | bash - && \
    apt-get install -y nodejs && rm -rf /var/lib/apt/lists/* && \
    go install github.com/air-verse/air@latest
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN npm install --prefix web
RUN CGO_ENABLED=0 go build -o /healthcheck ./cmd/healthcheck
COPY scripts ./scripts
RUN chmod +x scripts/dev.sh
EXPOSE 8080 5173
CMD ["go", "run", "./cmd/server"]

# ----- final minimal image -----
FROM gcr.io/distroless/base-debian12 AS prod
WORKDIR /
COPY --from=server-build /erabooru /erabooru
COPY --from=server-build /healthcheck /healthcheck
COPY --from=server-build /tmp/data /data
ENV BLEVE_PATH=/data/bleve
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/erabooru"]