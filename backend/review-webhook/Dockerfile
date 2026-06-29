FROM golang:1.26-alpine AS builder

WORKDIR /app
COPY metrics ./metrics

WORKDIR /app/metrics
COPY metrics/go.mod metrics/go.sum ./
RUN go mod download

WORKDIR /app/review-webhook
COPY review-webhook/go.mod review-webhook/go.sum ./
RUN go mod download

COPY review-webhook/cmd ./cmd
COPY review-webhook/internal ./internal
RUN go mod tidy && CGO_ENABLED=0 GOOS=linux go build -o /review-webhook ./cmd/server


FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

WORKDIR /app

COPY --from=builder /review-webhook .

USER appuser

EXPOSE 8002

CMD ["./review-webhook"]
