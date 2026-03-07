FROM alpine:3.23.3@sha256:25109184c71bdad752c8312a8623239686a9a2071e8825f20acb8f2198c3f659 AS mise
# Automatically set by BuildKit (e.g. amd64, arm64)
ARG TARGETARCH

RUN ARCH=$([ "$TARGETARCH" = "arm64" ] && echo "arm64" || echo "x64") && \
    wget -qO /usr/local/bin/mise \
      "https://github.com/jdx/mise/releases/download/v2026.2.23/mise-v2026.2.23-linux-${ARCH}-musl" && \
    chmod +x /usr/local/bin/mise

# ---

# ghcr.io/codize-dev/nsjail is based on debian:bookworm-slim
FROM ghcr.io/codize-dev/nsjail:87716b96b01ba350d9da7c672699189bac903db3@sha256:b31501a1b81d6e5b199f92e50834e238eb5dfc96bfc284c8007fe27275e84f6d AS base

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
      ca-certificates gpg gpg-agent && \
    rm -rf /var/lib/apt/lists/*

COPY --from=mise /usr/local/bin/mise /usr/local/bin/mise

ENV MISE_DATA_DIR="/mise"

ENV PATH="/mise/installs/node/24.14.0/bin:$PATH"
RUN mise use -g node@24.14.0

ENV PATH="/mise/installs/ruby/3.4.8/bin:$PATH"
RUN mise settings ruby.compile=false && mise use -g ruby@3.4.8

ENV PATH="/mise/installs/go/1.26.0/bin:$PATH"
RUN mise use -g go@1.26.0
RUN CGO_ENABLED=0 GOCACHE=/mise/go-cache go build std
COPY internal/sandbox/defaults/go/go.mod.tmpl /tmp/preinstall/go.mod
COPY internal/sandbox/defaults/go/go.sum.tmpl /tmp/preinstall/go.sum
RUN cd /tmp/preinstall && \
    GOMODCACHE=/mise/go-modcache go mod download && \
    rm -rf /tmp/preinstall

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
      curl wget mawk && \
    rm -rf /var/lib/apt/lists/*

# ---

FROM golang:1.25-bookworm@sha256:564e366a28ad1d70f460a2b97d1d299a562f08707eb0ecb24b659e5bd6c108e1 AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build \
      -trimpath \
      -ldflags="-w -s" \
      -o /out/gocacheprog \
      ./cmd/gocacheprog
RUN CGO_ENABLED=0 go build \
      -trimpath \
      -ldflags="-w -s" \
      -o /out/sandbox \
      .

# ---

FROM base

COPY internal/sandbox/configs/nsjail.cfg /etc/nsjail/nsjail.cfg
COPY internal/sandbox/configs/seccomp.kafel /etc/nsjail/seccomp.kafel
COPY --from=builder /out/gocacheprog /usr/local/bin/gocacheprog
COPY --from=builder /out/sandbox /usr/local/bin/sandbox
ENTRYPOINT ["/usr/local/bin/sandbox"]
