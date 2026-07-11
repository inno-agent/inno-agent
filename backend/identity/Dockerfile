FROM golang:1.26-alpine AS builder

WORKDIR /app
COPY pkg/telemetry ./pkg/telemetry
COPY pkg/tracing ./pkg/tracing
COPY pkg/logger ./pkg/logger

WORKDIR /app/pkg/telemetry
COPY pkg/telemetry/go.mod pkg/telemetry/go.sum ./
RUN go mod download

WORKDIR /app/pkg/tracing
COPY pkg/tracing/go.mod pkg/tracing/go.sum ./
RUN go mod download

WORKDIR /app/pkg/logger
COPY pkg/logger/go.mod pkg/logger/go.sum ./
RUN go mod download

WORKDIR /app/identity
COPY identity/go.mod identity/go.sum ./
RUN go mod download

COPY identity/ ./

RUN go build -o identity-server ./cmd/server

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /app/identity/identity-server .
EXPOSE 8081 9091
ENTRYPOINT ["./identity-server"]
