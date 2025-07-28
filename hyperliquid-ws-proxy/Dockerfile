# Build stage
FROM golang:1.21-alpine AS builder

# Install git and ca-certificates
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a -installsuffix cgo \
    -o hyperliquid-ws-proxy .

# Final stage - use busybox instead of scratch for better debugging
FROM busybox:musl

# Copy ca-certificates from builder
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy timezone data
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copy the binary
COPY --from=builder /app/hyperliquid-ws-proxy /hyperliquid-ws-proxy

# Install wget for health checks
RUN wget --version > /dev/null 2>&1 || echo "wget not available, health check may fail"

# Create necessary directories
RUN mkdir -p /var/lib/docker/volumes

# Run as root to access Docker volumes (necessary for local node access)
USER 0:0

# Expose port
EXPOSE 8080

# Health check using the /health endpoint
HEALTHCHECK --interval=30s --timeout=10s --start-period=30s --retries=3 \
    CMD wget --quiet --tries=1 --spider http://localhost:8080/health || exit 1

# Run the application
ENTRYPOINT ["/hyperliquid-ws-proxy"] 