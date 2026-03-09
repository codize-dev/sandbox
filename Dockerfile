FROM alpine:3.23.3@sha256:25109184c71bdad752c8312a8623239686a9a2071e8825f20acb8f2198c3f659 AS mise
# Automatically set by BuildKit (e.g. amd64, arm64)
ARG TARGETARCH

RUN ARCH=$([ "$TARGETARCH" = "arm64" ] && echo "arm64" || echo "x64") && \
    wget -qO /usr/local/bin/mise \
      "https://github.com/jdx/mise/releases/download/v2026.2.23/mise-v2026.2.23-linux-${ARCH}" && \
    chmod +x /usr/local/bin/mise

# ---

# ghcr.io/codize-dev/nsjail is based on debian:bookworm-slim
FROM ghcr.io/codize-dev/nsjail:222f2fa36125b31f734b039c647c44a9b42c1b6f@sha256:c200a59c915faafa1856b43fe28f4d190b7bef5486473222e07b4d2ab223edb2 AS base

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
      ca-certificates gpg gpg-agent && \
    rm -rf /var/lib/apt/lists/*

COPY --from=mise /usr/local/bin/mise /usr/local/bin/mise

ENV MISE_DATA_DIR="/mise"

# Install tools for sandbox
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
      curl wget mawk gcc libc-dev && \
    rm -rf /var/lib/apt/lists/*

# Node.js
ENV PATH="/mise/installs/node/24.14.0/bin:$PATH"
RUN mise use -g node@24.14.0
COPY internal/sandbox/defaults/node-typescript/package.json internal/sandbox/defaults/node-typescript/package-lock.json /mise/ts-node-modules/
RUN cd /mise/ts-node-modules && npm ci

# Ruby
ENV PATH="/mise/installs/ruby/3.4.8/bin:$PATH"
RUN mise settings ruby.compile=false && mise use -g ruby@3.4.8

# Go
ENV PATH="/mise/installs/go/1.26.0/bin:$PATH"
RUN mise use -g go@1.26.0
RUN CGO_ENABLED=0 GOCACHE=/mise/go-cache go build std
COPY internal/sandbox/defaults/go/go.mod.tmpl /tmp/preinstall/go.mod
COPY internal/sandbox/defaults/go/go.sum.tmpl /tmp/preinstall/go.sum
RUN cd /tmp/preinstall && \
    GOMODCACHE=/mise/go-modcache go mod download && \
    rm -rf /tmp/preinstall

# Python
ENV PATH="/mise/installs/python/3.13.12/bin:$PATH"
RUN mise use -g python@3.13.12

# Rust
ENV RUSTUP_HOME="/mise/rustup" \
    CARGO_HOME="/mise/cargo"
ENV PATH="/mise/cargo/bin:$PATH"
RUN mise use -g rust@1.94.0

# ---

FROM golang:1.26.1-bookworm@sha256:c7a82e9e2df2fea5d8cb62a16aa6f796d2b2ed81ccad4ddd2bc9f0d22936c3f2 AS builder
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
