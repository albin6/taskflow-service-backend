# Build stage
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git gcc musl-dev

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN mkdir -p /build/output
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /build/output/taskflow-server ./cmd/server

# Runtime stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates postgresql-client

# Create non-root user
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

WORKDIR /app

# Copy binary and migrations from builder
COPY --from=builder /build/output/taskflow-server /app/taskflow-server
COPY --from=builder /build/migrations /app/migrations

# Change ownership
RUN chown -R appuser:appuser /app && \
    chmod +x /app/taskflow-server

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 8080

# Run the application
CMD ["/app/taskflow-server"]
