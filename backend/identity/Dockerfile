FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY metrics ./metrics
WORKDIR /app/metrics
COPY metrics/go.mod metrics/go.sum ./
RUN go mod download

WORKDIR /app/identity
COPY identity/go.mod identity/go.sum ./
RUN go mod download
COPY identity/ .
RUN go mod tidy && go build -o identity-server ./cmd/server

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /app/identity/identity-server .
EXPOSE 8081 9091
ENTRYPOINT ["./identity-server"]
