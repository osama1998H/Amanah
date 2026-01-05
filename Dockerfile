# Amanah Payment Gateway - Production Dockerfile
# Multi-stage build for optimized container size

# ============================================
# Stage 1: Build
# ============================================
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build arguments
ARG VERSION=dev
ARG COMMIT_SHA=unknown
ARG BUILD_TIME=unknown

# Build the application
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.Version=${VERSION} -X main.CommitSHA=${COMMIT_SHA} -X main.BuildTime=${BUILD_TIME}" \
    -o /app/bin/gateway ./cmd/gateway

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.Version=${VERSION} -X main.CommitSHA=${COMMIT_SHA} -X main.BuildTime=${BUILD_TIME}" \
    -o /app/bin/api ./cmd/api

# ============================================
# Stage 2: Production Runtime
# ============================================
FROM alpine:3.19 AS production

# Security: Run as non-root user
RUN addgroup -g 1000 amanah && \
    adduser -u 1000 -G amanah -s /bin/sh -D amanah

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata curl

# Set timezone
ENV TZ=UTC

# Working directory
WORKDIR /app

# Copy built binaries
COPY --from=builder /app/bin/ ./bin/

# Copy configuration templates
COPY --from=builder /app/configs/ ./configs/

# Set ownership
RUN chown -R amanah:amanah /app

# Switch to non-root user
USER amanah

# Expose ports
EXPOSE 8080 8443 9090

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/health/live || exit 1

# Default command (can be overridden)
CMD ["./bin/gateway"]

# ============================================
# Stage 3: Development
# ============================================
FROM golang:1.23-alpine AS development

# Install development tools
RUN apk add --no-cache git make curl bash

# Install Go tools
RUN go install github.com/cosmtrek/air@latest && \
    go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Expose ports
EXPOSE 8080 8443 9090

# Development command with hot reload
CMD ["air", "-c", ".air.toml"]
