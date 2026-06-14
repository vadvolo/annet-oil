# Build stage
FROM golang:1.26-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git openssh-keygen

# Set working directory
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o annet-oil ./cmd/annet-oil

# Final stage
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache \
    openssh-keygen \
    docker-cli \
    ca-certificates \
    tzdata

# Create app user
RUN addgroup -g 1001 appgroup && \
    adduser -u 1001 -G appgroup -s /bin/sh -D appuser

# Create necessary directories
RUN mkdir -p /app/configs /app/storage /app/keys /tmp && \
    chown -R appuser:appgroup /app /tmp

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/annet-oil .

# Copy configuration files
COPY --chown=appuser:appgroup configs/ ./configs/
COPY --chown=appuser:appgroup storage/ ./storage/

# Make binary executable
RUN chmod +x annet-oil

# Switch to non-root user
USER appuser

# Expose ports
EXPOSE 22 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/api/v0/health || exit 1

# Default command
CMD ["./annet-oil", "server", "start"]