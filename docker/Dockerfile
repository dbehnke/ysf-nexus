# Build stage
FROM golang:1.21-alpine AS builder

# Install dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN make build-linux

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -S ysf && adduser -S ysf -G ysf

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/bin/ysf-nexus-linux ./ysf-nexus

# Copy example config
COPY --from=builder /app/configs/config.example.yaml ./config.example.yaml

# Change ownership
RUN chown -R ysf:ysf /app

# Switch to non-root user
USER ysf

# Expose ports
EXPOSE 42000/udp 8080 9090

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run the application
CMD ["./ysf-nexus"]