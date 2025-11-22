# GoHome justfile for building and deployment

# Default recipe to display help
default:
    @just --list

# Variables
app_name := "gohome"
image_name := app_name
image_tag := "latest"
registry := env_var_or_default("REGISTRY", "")
namespace := env_var_or_default("NAMESPACE", "default")
goos := env_var_or_default("GOOS", "linux")
goarch := env_var_or_default("GOARCH", "amd64")

# Development
# Build the Go binary
build:
    @echo "Building {{app_name}}..."
    CGO_ENABLED=0 GOOS={{goos}} GOARCH={{goarch}} go build -a -installsuffix cgo \
        -ldflags="-s -w -X main.Version={{`git describe --tags --always --dirty 2>/dev/null || echo 'dev'`}} -X main.BuildTime={{`date -u +%Y-%m-%dT%H:%M:%SZ`}}" \
        -o bin/{{app_name}} ./cmd/main.go

# Run the application locally
run:
    @echo "Running {{app_name}} locally..."
    go run cmd/main.go

# Run the application in demo mode (without Kubernetes)
run-demo:
    @echo "Running {{app_name}} in demo mode..."
    KUBECONFIG= go run cmd/main.go

# Run tests
test:
    @echo "Running tests..."
    go test -v ./...

# Format Go code
fmt:
    go fmt ./...

# Run go vet
vet:
    go vet ./...

# Download dependencies
deps:
    go mod download
    go mod tidy

# Set up development environment
setup:
    @echo "Setting up development environment..."
    mise install
    go mod download
    @echo "Development environment ready!"

# Clean build artifacts
clean:
    @echo "Cleaning build artifacts..."
    rm -rf bin/
    docker rmi {{image_name}}:{{image_tag}} 2>/dev/null || true

# Docker
# Build Docker image
docker-build:
    @echo "Building Docker image {{image_name}}:{{image_tag}}..."
    docker build -t {{image_name}}:{{image_tag}} \
        --build-arg VERSION={{`git describe --tags --always --dirty 2>/dev/null || echo 'dev'`}} \
        --build-arg BUILD_TIME={{`date -u +%Y-%m-%dT%H:%M:%SZ`}} .

# Push Docker image to registry
docker-push: docker-build
    #!/usr/bin/env bash
    if [ -z "{{registry}}" ]; then
        echo "Error: REGISTRY environment variable not set."
        echo "Use: REGISTRY=your-registry.com just docker-push"
        exit 1
    fi
    echo "Pushing {{registry}}/{{image_name}}:{{image_tag}}..."
    docker tag {{image_name}}:{{image_tag}} {{registry}}/{{image_name}}:{{image_tag}}
    docker push {{registry}}/{{image_name}}:{{image_tag}}

# Kubernetes
# Show application logs
logs:
    kubectl logs -l app={{app_name}} -n {{namespace}} --tail=100 -f

# Show deployment status
status:
    @echo "Deployment status:"
    kubectl get deployment {{app_name}} -n {{namespace}}
    @echo "\nPods:"
    kubectl get pods -l app={{app_name}} -n {{namespace}}
    @echo "\nService:"
    kubectl get service {{app_name}} -n {{namespace}}
    @echo "\nIngress:"
    kubectl get ingress {{app_name}} -n {{namespace}}

# Port forward to local machine
port-forward:
    @echo "Port forwarding {{app_name}} to localhost:8080..."
    kubectl port-forward svc/{{app_name}} 8080:80 -n {{namespace}}

# Restart the deployment
restart:
    kubectl rollout restart deployment/{{app_name}} -n {{namespace}}
    kubectl rollout status deployment/{{app_name}} -n {{namespace}}

# Edit the configuration ConfigMap
config-edit:
    kubectl edit configmap gohome-config -n {{namespace}}

# Local Development with Docker Compose
# Start local development environment
dev-up:
    @echo "Starting local development environment..."
    docker-compose up -d --build
    @echo "\nRunning on: http://localhost:8080"

# Stop local development environment
dev-down:
    @echo "Stopping local development environment..."
    docker-compose down

# Show development container logs
dev-logs:
    docker-compose logs -f gohome

# Restart development environment
dev-restart:
    docker-compose restart gohome

# Start development environment in demo mode (no Kubernetes)
dev-demo:
    @echo "Starting development environment in demo mode..."
    KUBECONFIG= docker-compose up

# Generate manifests with custom image
manifests:
    #!/usr/bin/env bash
    if [ -z "{{registry}}" ]; then
        echo "Error: REGISTRY environment variable not set."
        echo "Use: REGISTRY=your-registry.com just manifests"
        exit 1
    fi
    echo "Generating manifests for {{registry}}/{{image_name}}:{{image_tag}}..."
    sed 's|image: ghcr.io/joeds13/gohome:latest|image: {{registry}}/{{image_name}}:{{image_tag}}|g' k8s/deployment.yaml > k8s/deployment-custom.yaml
    echo "Generated k8s/deployment-custom.yaml"

# Show version information
version:
    @echo "GoHome version: {{`git describe --tags --always --dirty 2>/dev/null || echo 'dev'`}}"
    @echo "Build time: {{`date -u +%Y-%m-%dT%H:%M:%SZ`}}"
    @echo "Git commit: {{`git rev-parse HEAD 2>/dev/null || echo 'unknown'`}}"
    @echo "Git branch: {{`git branch --show-current 2>/dev/null || echo 'unknown'`}}"

# Test local setup (demo mode)
test-demo:
    @echo "Testing demo mode setup..."
    @echo "Building binary..."
    just build
    @echo "Running health check..."
    KUBECONFIG= timeout 10s ./bin/{{app_name}} &
    sleep 3
    curl -f http://localhost:8080/health || echo "Health check failed"
    pkill {{app_name}} || true

# Update Github Action versions
github-action-update:
    npx actions-up -y
