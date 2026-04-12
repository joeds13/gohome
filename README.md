# GoHome - Kubernetes Personal Homepage

A modern, clean personal homepage for your home cluster that displays Kubernetes ingresses and bookmarks with a sleek, monospaced design.

## Features

- 🚀 **Apps Section**: Pin frequently-used ingresses to a prominent row above Services using an annotation
- 🔗 **Automatic Service Discovery**: Lists all Kubernetes ingresses in alphabetical order
- 🏷️ **Flexible Annotations**: Hide ingresses, override display names, or promote them to Apps
- 📚 **Custom Bookmarks**: Configure additional bookmarks via ConfigMaps
- 🎨 **Modern Design**: Clean, techy aesthetic with monospaced fonts
- ⚡ **Lightweight**: Minimal resource usage, perfect for home clusters
- 🔒 **Secure**: Runs with minimal RBAC permissions and security contexts
- 📱 **Responsive**: Works great on desktop and mobile devices

## Screenshots

The homepage features:

- A clean header with cluster status indicator
- **Apps section** (green accent) — pinned, frequently-used services shown prominently at the top
- **Services section** (cyan/purple accent) — all other visible ingresses listed alphabetically
- Bookmarks section organized by categories
- Real-time timestamp in the footer

## Prerequisites

This project uses [mise](https://mise.jdx.dev/) to install required tools:

```bash
mise install
```

## Quick Start

### 1. Deploy from GitHub (Recommended)

```bash
# Deploy directly using Kustomize from GitHub
kubectl apply -k https://github.com/joeds13/gohome/k8s
```

This will:

- Create the gohome namespace
- Deploy with pre-built multi-arch images from ghcr.io
- Set up RBAC, ConfigMap, Service, and Ingress
- Support: linux/amd64, linux/arm64, linux/arm/v7

### 2. Clone and Deploy Locally

```bash
# Clone the repository
git clone https://github.com/joeds13/gohome.git
cd gohome

# Deploy using Kustomize
kubectl apply -k k8s/

# Or apply manifests individually
kubectl apply -f k8s/rbac.yaml
kubectl apply -f k8s/configmap.yaml
kubectl apply -f k8s/deployment.yaml
kubectl apply -f k8s/service.yaml
kubectl apply -f k8s/ingress.yaml
```

### 3. Build and Deploy Custom Image

```bash
# Clone the repository
git clone https://github.com/joeds13/gohome.git
cd gohome

# Build the Docker image
mise docker-build

# Generate custom manifests with your image
REGISTRY=your-registry.com mise manifests

# Deploy with custom image
kubectl apply -f k8s/deployment-custom.yaml
kubectl apply -f k8s/rbac.yaml
kubectl apply -f k8s/configmap.yaml
kubectl apply -f k8s/service.yaml
kubectl apply -f k8s/ingress.yaml
```

### 4. Verify Deployment

Check that everything is running correctly:

```bash
# Check deployment status
kubectl get all -n gohome

# Watch pod startup
kubectl get pods -n gohome -w

# Check logs
kubectl logs -l app=gohome -n gohome

# Test connectivity (port forward)
kubectl port-forward svc/gohome 8080:80 -n gohome
# Then visit http://localhost:8080
```

### 5. Configure Your Domain

Edit the Kustomize configuration or ingress directly:

**Option A: Using Kustomize (Recommended)**
```bash
# Edit k8s/kustomization.yaml and update the host patch
# Then redeploy
kubectl apply -k k8s/
```

**Option B: Edit ingress directly**
```bash
# Edit the ingress after deployment
kubectl edit ingress gohome -n gohome

# Or edit k8s/ingress.yaml and reapply
kubectl apply -f k8s/ingress.yaml
```

## 🏗️ Multi-Architecture Support

GoHome provides pre-built container images for multiple architectures:

- **linux/amd64** - Standard x86_64 systems
- **linux/arm64** - ARM64 systems (Apple Silicon, newer ARM servers)
- **linux/arm/v7** - ARMv7 systems (Raspberry Pi, etc.)

Images are automatically built using GitHub Actions and published to GitHub Container Registry (`ghcr.io`).

### Using Pre-built Images

The deployment manifests automatically use the multi-arch images:

```yaml
# k8s/deployment.yaml
spec:
  template:
    spec:
      containers:
      - name: gohome
        image: ghcr.io/joeds13/gohome:latest  # Multi-arch image
```

### GitHub Actions CI/CD

The project includes comprehensive GitHub Actions workflows:

- **🔄 Continuous Integration** (`build.yml`)
  - Runs on every push and PR
  - Tests, linting, security scans
  - Multi-arch builds for amd64 and arm64
  - Automatic image publishing

- **🚀 Release Automation** (`release.yml`)
  - Triggered on version tags
  - Builds binaries for multiple platforms
  - Creates GitHub releases with assets
  - Publishes multi-arch container images
  - Generates SBOMs and security reports

- **✅ Pull Request Validation** (`pr.yml`)
  - Code quality checks
  - Kubernetes manifest validation
  - Security scanning
  - Documentation checks

### 6. Customize Bookmarks

Edit the ConfigMap to add your own bookmarks:

```bash
kubectl edit configmap gohome-config -n gohome
```

## Configuration

### Environment Variables

- `PORT`: Server port (default: 8080)
- `NAMESPACE`: Kubernetes namespace to watch (default: default)
- `CONFIG_MAP_NAME`: ConfigMap name for bookmarks (default: gohome-config)

### Ingress Annotations

GoHome supports three annotations on `Ingress` resources:

| Annotation | Value | Effect |
|---|---|---|
| `gohome.stringer.sh/app` | `"true"` | Promotes the ingress to the **Apps** section (shown above Services) |
| `gohome.stringer.sh/hide` | `"true"` | Hides the ingress from the homepage entirely |
| `gohome.stringer.sh/name` | any string | Overrides the display name shown on the card |

#### Promoting an ingress to the Apps section

Add `gohome.stringer.sh/app: "true"` to any ingress you click frequently (e.g. FreshRSS, Home Assistant). It will appear in the green-accented **Apps** row at the top of the page and will **not** be duplicated in the Services section below.

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: freshrss
  annotations:
    gohome.stringer.sh/app: "true"  # Promotes to the Apps section
spec:
  tls:
    - hosts:
        - rss.example.com
      secretName: freshrss-tls
  rules:
    - host: rss.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: freshrss
                port:
                  number: 80
```

#### Hiding an ingress

Add `gohome.stringer.sh/hide: "true"` to any ingress you want to hide from the homepage entirely:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: private-service
  annotations:
    gohome.stringer.sh/hide: "true"  # Hidden from the homepage
spec:
  # ... rest of ingress spec
```

#### Overriding the display name

By default GoHome uses the ingress `name` (stripping a trailing `-ingress` suffix). Override it with:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: my-app-ingress
  annotations:
    gohome.stringer.sh/name: "My App"  # Displayed as "My App" instead of "my-app"
spec:
  # ... rest of ingress spec
```

#### Combining annotations

Annotations can be combined freely. For example, promote an ingress to Apps *and* give it a friendly display name:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: freshrss-ingress
  annotations:
    gohome.stringer.sh/app: "true"
    gohome.stringer.sh/name: "FreshRSS"
spec:
  # ... rest of ingress spec
```

### Bookmark Configuration

Bookmarks are configured in the ConfigMap with the format:

```yaml
data:
  bookmark-<name>: "url|category"
```

Example:
```yaml
data:
  title: "Go Home"
  bookmark-grafana: "https://grafana.example.com|Infrastructure"
  bookmark-nextcloud: "https://cloud.example.com|Applications"
```

## Metrics

GoHome exposes Prometheus metrics at `/metrics`. The following application-specific metrics are available:

| Metric | Type | Description |
|---|---|---|
| `gohome_apps_displayed` | Gauge | Number of Apps currently shown in the Apps section |
| `gohome_services_displayed` | Gauge | Number of ingresses currently shown in the Services section |
| `gohome_unique_visitors` | Gauge (labelled `email`) | Unique Tailscale users who have loaded the homepage |
| `gohome_http_requests_total` | Counter | Total HTTP requests by `code` and `method` |
| `gohome_http_requests_in_flight` | Gauge | Current number of in-flight HTTP requests |
| `gohome_http_request_duration_seconds` | Histogram | Request duration by `code` and `method` |

A pre-built Grafana dashboard is included in `k8s/monitoring/grafana-dashboard.yaml`.

## Architecture

```
┌─────────────────┐    ┌─────────────────────────────────────────┐    ┌─────────────────┐
│   Web Browser   │────│         Kubernetes Ingresses            │────│   ConfigMap     │
│                 │    │  app=true → Apps  |  rest → Services    │    │   (Bookmarks)   │
└─────────────────┘    └─────────────────────────────────────────┘    └─────────────────┘
        │                                  │
        │                                  │
        ▼                                  ▼
┌─────────────────────────────────────────────────────────────────┐
│                        GoHome App                               │
│                                                                 │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐ │
│  │ K8s Client  │  │   Server    │  │   Template Engine       │ │
│  │  (k8s.go)   │  │ (server.go) │  │    (index.html)         │ │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

## Development

### Local Development

For local development outside of Kubernetes:

```bash
# Set up development environment
mise setup

# Install dependencies
mise deps

# Run locally (requires kubeconfig for your cluster)
export KUBECONFIG=/path/to/your/kubeconfig
mise go-run

# Run in demo mode (no Kubernetes required)
mise go-run-demo

# Or use Docker Compose for containerized development
mise dev-up

# Run Docker Compose in demo mode
mise dev-demo
```

### Building

```bash
# Build for current platform (with version info)
mise build

# Build Docker image (multi-arch capable)
mise docker-build

# Cross-compile for different platforms
GOOS=linux GOARCH=amd64 mise build
GOOS=linux GOARCH=arm64 mise build
GOOS=darwin GOARCH=arm64 mise build

# Show version information
mise version
```

### GitHub Actions Integration

The project uses GitHub Actions for automated CI/CD:

```bash
# Trigger builds on push to main
git push origin main

# Create a release (triggers full multi-arch build)
git tag v1.0.0
git push origin v1.0.0
```

The release workflow will:

- Build multi-arch containers (amd64, arm64, arm/v7)
- Create GitHub release with binaries
- Run security scans and generate SBOMs
- Update container registries

### Container Registries

Images are published to:
- **GitHub Container Registry**: `ghcr.io/joeds13/gohome:latest`
- **Docker Hub** (optional): `joeds13/gohome:latest`

All images support multiple architectures and are automatically selected based on your system.

## Security

### RBAC Permissions

GoHome requires minimal permissions:
- `get`, `list`, `watch` on `networking.k8s.io/ingresses`
- `get`, `list`, `watch` on `configmaps`

### Security Features

- Runs as non-root user (UID 1001)
- Read-only root filesystem where possible
- Drops all Linux capabilities
- Security contexts applied at pod and container level
- Minimal resource requests and limits

## Troubleshooting

### Common Issues

**App shows "Failed to load ingresses"**
- Check RBAC permissions: `kubectl auth can-i list ingresses --as=system:serviceaccount:default:gohome`
- Verify the service account exists: `kubectl get serviceaccount gohome`

**Bookmarks not appearing**
- Check ConfigMap exists: `kubectl get configmap gohome-config`
- Verify ConfigMap format matches the expected pattern
- Edit configuration: `mise config-edit`

**Page won't load**
- Check pod logs: `mise logs`
- Verify service is running: `mise status`
- Check ingress configuration: `kubectl describe ingress gohome`

### Debug Mode

Enable debug logging by setting environment variable:
```yaml
env:
- name: LOG_LEVEL
  value: "debug"
```

### Quick Debugging Commands

```bash
mise status           # Show deployment status
mise logs             # View application logs
mise port-forward     # Access app locally via port forward
mise test-demo        # Test demo mode setup locally
kubectl describe pod -l app=gohome -n gohome  # Describe pods
kubectl exec -it deployment/gohome -n gohome -- /bin/sh  # Shell access
```

## Customization

### Styling

The application uses CSS custom properties for easy theming. Main color variables:

```css
:root {
    --bg-primary: #0a0a0b;      /* Main background */
    --bg-secondary: #1a1a1e;    /* Card backgrounds */
    --text-primary: #e8e8ea;    /* Primary text */
    --accent-primary: #00d4ff;   /* Links and accents */
    --accent-secondary: #7c3aed; /* Secondary accent */
}
```

### Adding New Features

The codebase is organized for easy extension:
- `internal/k8s.go` - Kubernetes API interactions
- `internal/config.go` - Configuration and bookmark management
- `internal/server.go` - HTTP server and routing
- `templates/index.html` - HTML template
- `static/style.css` - Styling

## Contributing

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/amazing-feature`
3. Make your changes
4. Run quality checks: `mise test && mise vet && mise fmt`
5. Test the build: `mise build && mise docker-build`
6. Submit a pull request

### Development Workflow

The project uses GitHub Actions for quality assurance:

- **All PRs** trigger validation workflows
- **Code quality** checks (linting, formatting, security)
- **Multi-arch builds** ensure compatibility
- **Kubernetes manifest** validation
- **Automated security** scanning

### Release Process

1. Update version in code if needed
2. Create and push a git tag: `git tag v1.x.x && git push origin v1.x.x`
3. GitHub Actions automatically:
   - Builds multi-arch container images
   - Creates GitHub release with binaries
   - Publishes to container registries
   - Generates security reports

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Acknowledgments

- Built with Go and the official Kubernetes client library
- Uses JetBrains Mono font for the monospaced aesthetic
- Inspired by modern dashboard designs and terminal aesthetics

## Repository

- **GitHub**: https://github.com/joeds13/gohome
- **Container Images**: https://ghcr.io/joeds13/gohome
- **Issues**: https://github.com/joeds13/gohome/issues
- **Releases**: https://github.com/joeds13/gohome/releases
