FROM alpine:3.23.3@sha256:25109184c71bdad752c8312a8623239686a9a2071e8825f20acb8f2198c3f659 AS mise
# Automatically set by BuildKit (e.g. amd64, arm64)
ARG TARGETARCH

# renovate: datasource=github-releases depName=jdx/mise extractVersion=^v(?<version>.+)$
ARG MISE_VERSION=2026.4.11
RUN ARCH=$([ "$TARGETARCH" = "arm64" ] && echo "arm64" || echo "x64") && \
    wget -qO /usr/local/bin/mise \
      "https://github.com/jdx/mise/releases/download/v${MISE_VERSION}/mise-v${MISE_VERSION}-linux-${ARCH}" && \
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
# renovate: datasource=node-version depName=node
ARG NODE_VERSION=24.14.0
ENV PATH="/mise/installs/node/${NODE_VERSION}/bin:$PATH"
RUN mise use -g node@${NODE_VERSION} && \
    ln -s /mise/installs/node/${NODE_VERSION} /mise/installs/node/current
COPY internal/sandbox/defaults/node-typescript/package.json internal/sandbox/defaults/node-typescript/package-lock.json /mise/ts-node-modules/
RUN cd /mise/ts-node-modules && npm ci

# Ruby
# renovate: datasource=ruby-version depName=ruby
ARG RUBY_VERSION=3.4.9
ENV PATH="/mise/installs/ruby/${RUBY_VERSION}/bin:$PATH"
RUN mise settings ruby.compile=false && mise use -g ruby@${RUBY_VERSION} && \
    ln -s /mise/installs/ruby/${RUBY_VERSION} /mise/installs/ruby/current

# Go
# renovate: datasource=golang-version depName=go
ARG GO_VERSION=1.26.2
ENV PATH="/mise/installs/go/${GO_VERSION}/bin:$PATH"
RUN mise use -g go@${GO_VERSION} && \
    ln -s /mise/installs/go/${GO_VERSION} /mise/installs/go/current
RUN CGO_ENABLED=0 GOCACHE=/mise/go-cache go build std
COPY internal/sandbox/defaults/go/go.mod.tmpl /tmp/preinstall/go.mod
COPY internal/sandbox/defaults/go/go.sum.tmpl /tmp/preinstall/go.sum
RUN cd /tmp/preinstall && \
    GOMODCACHE=/mise/go-modcache go mod download && \
    rm -rf /tmp/preinstall

# Python
# renovate: datasource=python-version depName=python
ARG PYTHON_VERSION=3.13.12
ENV PATH="/mise/installs/python/${PYTHON_VERSION}/bin:$PATH"
RUN mise use -g python@${PYTHON_VERSION} && \
    ln -s /mise/installs/python/${PYTHON_VERSION} /mise/installs/python/current

# Rust
# renovate: datasource=github-tags depName=rust packageName=rust-lang/rust
ARG RUST_VERSION=1.94.1
ENV RUSTUP_HOME="/mise/rustup" \
    CARGO_HOME="/mise/cargo"
ENV PATH="/mise/cargo/bin:$PATH"
RUN mise use -g rust@${RUST_VERSION}

# ---

FROM golang:1.26.2-bookworm@sha256:4f4ab2c90005e7e63cb631f0b4427f05422f241622ee3ec4727cc5febbf83e34 AS builder
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
