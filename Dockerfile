FROM golang:1.25-bookworm@sha256:564e366a28ad1d70f460a2b97d1d299a562f08707eb0ecb24b659e5bd6c108e1 AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build \
      -trimpath \
      -ldflags="-w -s" \
      -o /out/server \
      .

# ---

FROM ghcr.io/codize-dev/nsjail:83d63e1fc0bddd5cff3b077a4ece89515cb8a482@sha256:536e7c0d8b591bb3a12b86fdbbaee617e503d7058606a48a10b189d20a5cfb09

COPY --from=builder /out/server /usr/local/bin/server

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/server"]
