# Use the official Go image as the build stage
FROM golang:1.25-alpine AS builder

# Install ca-certificates and git for secure connections and go modules
RUN apk --no-cache add ca-certificates git

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o server cmd/server/main.go

# Use a minimal alpine image for the final stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

# Create non-root user
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

WORKDIR /root/

# Copy the binary from the builder stage
COPY --from=builder /app/server .

# Copy any necessary config files (if you have them)
# COPY --from=builder /app/configs ./configs

# Change ownership to non-root user
RUN chown -R appuser:appgroup /root/
USER appuser

# Expose the gRPC port
EXPOSE 50051

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD nc -z localhost 50051 || exit 1

# Command to run the application
CMD ["./server"]