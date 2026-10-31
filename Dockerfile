# syntax=docker/dockerfile:1

FROM --platform=$BUILDPLATFORM node:24-alpine AS client
WORKDIR /app/web
RUN npm install -g pnpm@10.29.3
COPY web/package.json web/pnpm-lock.yaml web/pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile
COPY web/ ./
RUN pnpm build

FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=client /app/internal/web/dist ./internal/web/dist
ARG VERSION=dev
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /out/falafel ./cmd/falafel

FROM alpine:3.24
LABEL org.opencontainers.image.title="Falafel" \
      org.opencontainers.image.description="Browser to browser file transfer over WebRTC" \
      org.opencontainers.image.source="https://github.com/MahbodHastam/falafel" \
      org.opencontainers.image.licenses="MIT"
RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -H -u 10001 falafel \
    && mkdir -p /data \
    && chown falafel:falafel /data

WORKDIR /app
COPY --from=build /out/falafel /app/falafel
USER falafel

ENV FALAFEL_ADDR=:8080 \
    FALAFEL_DB_PATH=/data/falafel.db

EXPOSE 8080
VOLUME ["/data"]

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8080/healthz || exit 1

ENTRYPOINT ["/app/falafel"]
