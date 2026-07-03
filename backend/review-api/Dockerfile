FROM golang:1.26-alpine AS builder

WORKDIR /app
COPY pkg/telemetry ./pkg/telemetry

WORKDIR /app/pkg/telemetry
COPY pkg/telemetry/go.mod pkg/telemetry/go.sum ./
RUN go mod download

WORKDIR /app/review-api
COPY review-api/go.mod review-api/go.sum ./
RUN go mod download

COPY review-api/cmd ./cmd
COPY review-api/internal ./internal
RUN CGO_ENABLED=0 GOOS=linux go build -o /review-api ./cmd/server


FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

WORKDIR /app

COPY --from=builder /review-api .

USER appuser

EXPOSE 8001

CMD ["./review-api"]
