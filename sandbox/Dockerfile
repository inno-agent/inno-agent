FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY go.mod ./
COPY server.go ./
RUN CGO_ENABLED=0 go build -o /sandbox-server .

FROM golang:1.22-alpine AS runtime
RUN apk add --no-cache nodejs npm ripgrep git python3 bash \
 && adduser -D -u 10001 sandbox
RUN go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest \
 && mv /root/go/bin/golangci-lint /usr/local/bin/
COPY --from=builder /sandbox-server /usr/local/bin/sandbox-server
RUN mkdir -p /workspace && chown sandbox:sandbox /workspace
USER sandbox
WORKDIR /workspace
EXPOSE 8080
CMD ["sandbox-server"]
