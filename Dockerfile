# Multi-stage Containerfile for Escocar Autos web server
# Stage 1: Build binary with Go
FROM docker.io/library/golang:1.24-alpine AS builder

WORKDIR /src

# Install CA certificates and git
RUN apk add --no-cache ca-certificates git

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy application source
COPY . .

# Compile static binary
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s" \
    -trimpath \
    -o /app/escocar-server ./cmd/server

# Stage 2: Minimal runtime image
FROM docker.io/library/alpine:3.20

WORKDIR /app

# Install security updates and timezone data
RUN apk add --no-cache ca-certificates tzdata

# Create application directories
RUN mkdir -p /app/uploads /app/web/static /app/web/templates

# Copy binary and assets
COPY --from=builder /app/escocar-server /app/escocar-server
COPY web/ /app/web/

# Set ownership
RUN chmod +x /app/escocar-server

# Volume for persisted vehicle pictures
VOLUME ["/app/uploads"]

# Expose default HTTP port
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8080/health || exit 1

ENTRYPOINT ["/app/escocar-server"]
