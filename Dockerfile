# Stage 1: Build the static Go binary
FROM golang:1.25-alpine AS builder

WORKDIR /build

# Install build prerequisites
RUN apk add --no-cache git ca-certificates tzdata

# Cache Go dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code and documentation
COPY . .

# Build statically compiled binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w -extldflags '-static'" \
    -o /build/jobboard \
    ./cmd/server

# Stage 2: Final lightweight runtime container
FROM alpine:3.20

# Install runtime dependencies for health checks and timezone support
RUN apk --no-cache add ca-certificates tzdata curl

# Create non-root user and group
RUN addgroup -g 10001 -S appgroup && \
    adduser -u 10001 -S appuser -G appgroup

WORKDIR /app

# Copy binary from builder stage
COPY --from=builder --chown=appuser:appgroup /build/jobboard /app/jobboard

# Switch to non-root user
USER appuser:appgroup

# Expose HTTP service port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
  CMD curl -f http://localhost:8080/healthz || exit 1

# Entrypoint & default subcommand
ENTRYPOINT ["/app/jobboard"]
CMD ["server"]
