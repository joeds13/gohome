# Build stage
# Always run on the native host architecture to avoid QEMU emulation
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder

# Build arguments
ARG VERSION=dev
ARG BUILD_TIME=unknown

# Injected by buildx for cross-compilation
ARG TARGETOS
ARG TARGETARCH

WORKDIR /app

# Install ca-certificates (needed for HTTPS in final image)
RUN apk add --no-cache ca-certificates

# Copy source code
COPY . .

# Cross-compile natively — no QEMU involved
# Cache mounts persist the Go build cache and module cache across runs
RUN --mount=type=cache,target=/root/.cache/go-build,id=go-build-${TARGETARCH} \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
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

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /app/main .
COPY --from=builder /app/templates ./templates
COPY --from=builder /app/static ./static

EXPOSE 8080

CMD ["./main"]
