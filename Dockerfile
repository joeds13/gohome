# Build stage
FROM golang:1.25-alpine AS builder

# Build arguments
ARG VERSION=dev
ARG BUILD_TIME=unknown

WORKDIR /app

# Install git for go mod download
RUN apk add --no-cache git ca-certificates

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application with version information
RUN CGO_ENABLED=0 go build \
    -a -installsuffix cgo \
    -ldflags="-s -w -X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME}" \
    -o main ./cmd/main.go

# Final stage
FROM gcr.io/distroless/static:nonroot

# Build arguments for metadata
ARG VERSION=dev
ARG BUILD_TIME=unknown

# Add metadata labels
LABEL org.opencontainers.image.title="GoHome" \
    org.opencontainers.image.description="Kubernetes personal homepage for home clusters" \
    org.opencontainers.image.version="${VERSION}" \
    org.opencontainers.image.created="${BUILD_TIME}" \
    org.opencontainers.image.source="https://github.com/joeds13/gohome" \
    org.opencontainers.image.licenses="MIT" \
    org.opencontainers.image.authors="GoHome Contributors" \
    org.opencontainers.image.documentation="https://github.com/joeds13/gohome/blob/main/README.md"

WORKDIR /app

# Copy ca-certificates from builder stage for HTTPS requests
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy the binary from builder stage
COPY --from=builder /app/main .

# Copy static files and templates
COPY --from=builder /app/templates ./templates
COPY --from=builder /app/static ./static

EXPOSE 8080

# Note: HEALTHCHECK removed as distroless images don't include wget/curl
# Health checks should be configured at the orchestration level (e.g., Kubernetes)

CMD ["./main"]
