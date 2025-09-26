# Build stage
FROM golang:1.25.1-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git libwebp-dev gcc musl-dev

WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o thumbla .

# Final stage
FROM alpine:3.20

# Install CA certificates for HTTPS requests, wget for health checks, and add non-root user
RUN apk --no-cache add ca-certificates wget libwebp && \
    addgroup -g 1001 -S thumbla && \
    adduser -u 1001 -S thumbla -G thumbla

# Copy binary from builder stage with proper ownership
COPY --from=builder --chown=thumbla:thumbla /app/thumbla /usr/local/bin/thumbla

# Switch to non-root user
USER thumbla

# Health check endpoint
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:1323/health || exit 1

EXPOSE 1323

ENTRYPOINT ["/usr/local/bin/thumbla"]
