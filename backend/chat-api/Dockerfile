FROM golang:1.26-alpine AS builder

WORKDIR /app
COPY metrics ./metrics

WORKDIR /app/metrics
COPY metrics/go.mod metrics/go.sum ./
RUN go mod download

WORKDIR /app/chat-api
COPY chat-api/go.mod chat-api/go.sum ./
RUN go mod download

COPY chat-api/cmd ./cmd
COPY chat-api/internal ./internal
RUN go mod tidy && CGO_ENABLED=0 GOOS=linux go build -o /chat-api ./cmd/server


FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

WORKDIR /app

COPY --from=builder /chat-api .

USER appuser

EXPOSE 8000

CMD ["./chat-api"]
