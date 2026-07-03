FROM golang:1.26-alpine AS builder

WORKDIR /app
COPY pkg/telemetry ./pkg/telemetry

WORKDIR /app/pkg/telemetry
COPY pkg/telemetry/go.mod pkg/telemetry/go.sum ./
RUN go mod download

WORKDIR /app/review-consumer
COPY review-consumer/go.mod review-consumer/go.sum ./
RUN go mod download

COPY review-consumer/cmd ./cmd
COPY review-consumer/internal ./internal
RUN CGO_ENABLED=0 GOOS=linux go build -o /review-consumer ./cmd/server


FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

WORKDIR /app

COPY --from=builder /review-consumer .

USER appuser

EXPOSE 9090

CMD ["./review-consumer"]
