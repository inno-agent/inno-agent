FROM golang:1.25-alpine AS builder
WORKDIR /src
COPY go.mod ./
COPY server.go ./
RUN CGO_ENABLED=0 go build -o /sandbox-server .

FROM golang:1.25-alpine AS runtime
RUN apk add --no-cache nodejs npm ripgrep git python3 bash \
 && adduser -D -u 10001 sandbox
# golangci-lint v2, pinned to the version this repo's CI uses. Note the /v2
# module path — v1 @latest now requires Go >= 1.23 and fails on older bases.
RUN GOBIN=/usr/local/bin GOTOOLCHAIN=auto CGO_ENABLED=0 \
    go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2
COPY --from=builder /sandbox-server /usr/local/bin/sandbox-server
RUN mkdir -p /workspace && chown sandbox:sandbox /workspace
USER sandbox
WORKDIR /workspace
EXPOSE 8080
CMD ["sandbox-server"]
