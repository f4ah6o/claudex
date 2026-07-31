FROM golang:1.26.5-bookworm@sha256:1ecb7edf62a0408027bd5729dfd6b1b8766e578e8df93995b225dfd0944eb651 AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
ARG COMMIT=none
ARG BUILD_DATE=unknown

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -buildvcs=false \
    -ldflags="-s -w -X 'main.Version=${VERSION}' -X 'main.Commit=${COMMIT}' -X 'main.BuildDate=${BUILD_DATE}'" \
    -o /out/claudex ./cmd/claudex

FROM debian:bookworm-slim@sha256:7b140f374b289a7c2befc338f42ebe6441b7ea838a042bbd5acbfca6ec875818

RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates tzdata util-linux \
    && useradd --system --home-dir /home/claudex --create-home --shell /usr/sbin/nologin claudex \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /out/claudex /app/claudex
COPY claudex.example.yaml /app/claudex.example.yaml
COPY scripts/docker-entrypoint.sh /usr/local/bin/claudex-entrypoint
RUN chmod 0755 /usr/local/bin/claudex-entrypoint

EXPOSE 8317

ENTRYPOINT ["/usr/local/bin/claudex-entrypoint"]
CMD ["serve", "--config", "/app/claudex.yaml"]
