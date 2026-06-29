FROM golang:1.26-alpine AS builder

WORKDIR /app
COPY metrics ./metrics

WORKDIR /app/metrics
COPY metrics/go.mod metrics/go.sum ./
RUN go mod download

WORKDIR /app/innoagent
RUN apk add --no-cache git ca-certificates tzdata

COPY innoagent/go.mod .
RUN go mod download || true

COPY innoagent/ .

RUN go mod tidy && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o server \
    ./cmd/server

FROM alpine:3.19

RUN apk add --no-cache ca-certificates curl tzdata && \
    addgroup -S appgroup && \
    adduser -S appuser -G appgroup

WORKDIR /app

COPY --from=builder /app/innoagent/server .

RUN chown -R appuser:appgroup /app

USER appuser

EXPOSE 8080

HEALTHCHECK --interval=15s --timeout=5s --start-period=10s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

CMD ["./server"]
