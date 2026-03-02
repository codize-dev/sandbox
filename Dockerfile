FROM alpine:3.21 AS mise
# Automatically set by BuildKit (e.g. amd64, arm64)
ARG TARGETARCH

RUN ARCH=$([ "$TARGETARCH" = "arm64" ] && echo "arm64" || echo "x64") && \
    wget -qO /usr/local/bin/mise \
      "https://github.com/jdx/mise/releases/download/v2026.2.23/mise-v2026.2.23-linux-${ARCH}-musl" && \
    chmod +x /usr/local/bin/mise

# ---

FROM ghcr.io/codize-dev/nsjail:83d63e1fc0bddd5cff3b077a4ece89515cb8a482@sha256:536e7c0d8b591bb3a12b86fdbbaee617e503d7058606a48a10b189d20a5cfb09 AS base

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

COPY --from=builder /out/gocacheprog /usr/local/bin/gocacheprog
COPY --from=builder /out/sandbox /usr/local/bin/sandbox
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/sandbox", "serve"]
