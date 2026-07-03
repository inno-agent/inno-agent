FROM golang:1.26-alpine AS builder

WORKDIR /app
COPY pkg/telemetry ./pkg/telemetry

WORKDIR /app/pkg/telemetry
COPY pkg/telemetry/go.mod pkg/telemetry/go.sum ./
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
