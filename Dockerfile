# ==========================
# Stage 1: Builder
# ==========================
FROM golang:1.24 AS builder

WORKDIR /workspace

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
      unzip curl protobuf-compiler && \
    rm -rf /var/lib/apt/lists/*

RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN mkdir -p gen/v1 && \
    protoc -I proto \
      --go_out=gen --go_opt=paths=source_relative \
      --go-grpc_out=gen --go-grpc_opt=paths=source_relative \
      proto/v1/*.proto

RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/server ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/client ./cmd/client

# ==========================
# Stage 2: Runtime
# ==========================
FROM gcr.io/distroless/base-debian12 AS runtime

COPY --from=builder /bin/server /bin/server
COPY --from=builder /bin/client /bin/client

CMD ["/bin/server"]
