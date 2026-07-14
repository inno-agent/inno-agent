FROM golang:1.22-alpine

RUN apk add --no-cache \
    nodejs \
    npm \
    ripgrep \
    git \
    python3 \
    py3-pip \
    curl \
    bash

# Go linters
RUN go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Workspace
WORKDIR /workspace
RUN chmod 777 /workspace

EXPOSE 8080

CMD ["python3", "-m", "http.server", "8080", "--directory", "/workspace"]
