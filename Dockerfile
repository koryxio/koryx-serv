# Build stage
FROM golang:1.24.7-alpine3.20 AS builder

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w -X main.version=docker" -o koryx-serv

# Runtime stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 koryx-serv && \
    adduser -D -u 1000 -G koryx-serv koryx-serv

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/koryx-serv /app/koryx-serv

# Copy example config
COPY --from=builder /app/config.example.json /app/config.example.json

# Create directory for serving files
RUN mkdir -p /app/public && \
    chown -R koryx-serv:koryx-serv /app

# Switch to non-root user
USER koryx-serv

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/ || exit 1

# Run the application
ENTRYPOINT ["/app/koryx-serv"]
CMD ["-dir", "/app/public"]
