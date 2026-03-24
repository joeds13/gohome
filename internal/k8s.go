package internal

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	// HideAnnotation is the annotation key to hide ingresses from the homepage
	HideAnnotation = "gohome.stringer.sh/hide"
)

// IngressInfo represents a simplified ingress for display
type IngressInfo struct {
	Name            string
	Host            string
	Path            string
	URL             string
	Tailscale       bool
	TailscaleFunnel bool
}

// K8sClient wraps the Kubernetes client
type K8sClient struct {
	clientset *kubernetes.Clientset
}

// NewK8sClient creates a new Kubernetes client, trying in-cluster config first, then kubeconfig
func NewK8sClient() (*K8sClient, error) {
	var config *rest.Config
	var err error

	// Try in-cluster config first (for when running in Kubernetes)
	config, err = rest.InClusterConfig()
	if err != nil {
		log.Printf("In-cluster config not available, trying kubeconfig: %v", err)

		// Try to load kubeconfig for local development
		config, err = loadKubeConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to load kubeconfig: %w", err)
		}
		log.Println("Using kubeconfig for Kubernetes client")
	} else {
		log.Println("Using in-cluster config for Kubernetes client")
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	return &K8sClient{
		clientset: clientset,
	}, nil
}

// loadKubeConfig loads the kubeconfig from default locations
func loadKubeConfig() (*rest.Config, error) {
	// Try KUBECONFIG environment variable first
	kubeconfigPath := os.Getenv("KUBECONFIG")
	if kubeconfigPath == "" {
		// Fall back to default location
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("unable to find home directory: %w", err)
		}
		kubeconfigPath = filepath.Join(homeDir, ".kube", "config")
	}

	// Check if kubeconfig file exists
	if _, err := os.Stat(kubeconfigPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("kubeconfig file not found at %s", kubeconfigPath)
	}

	// Load the kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("error building kubeconfig: %w", err)
	}

	return config, nil
}

// GetClientset returns the underlying Kubernetes clientset
func (k *K8sClient) GetClientset() *kubernetes.Clientset {
	return k.clientset
}

// GetVisibleIngresses returns all ingresses that should be displayed on the homepage
func (k *K8sClient) GetVisibleIngresses(ctx context.Context) ([]IngressInfo, error) {
	if k == nil || k.clientset == nil {
		log.Printf("Info: Kubernetes client not available, returning demo ingresses")
		return k.getDemoIngresses(), nil
	}

	ingresses, err := k.clientset.NetworkingV1().Ingresses("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list ingresses: %w", err)
	}

	var visibleIngresses []IngressInfo
	for _, ingress := range ingresses.Items {
		// Skip ingresses with hide annotation
		if shouldHide := ingress.Annotations[HideAnnotation]; shouldHide == "true" {
			log.Printf("Hiding ingress %s/%s due to annotation", ingress.Namespace, ingress.Name)
			continue
		}

		// Extract ingress information
		info := k.extractIngressInfo(&ingress)
		if info.URL != "" {
			visibleIngresses = append(visibleIngresses, info)
		}
	}

	// Sort alphabetically by name
	sort.Slice(visibleIngresses, func(i, j int) bool {
		return visibleIngresses[i].Name < visibleIngresses[j].Name
	})

	return visibleIngresses, nil
}

// isTailscaleIngress returns true when the ingress is managed by the Tailscale operator.
// The operator sets ingressClassName to "tailscale" and uses a wildcard host in spec.rules,
// publishing the real hostname via status.loadBalancer.ingress[].hostname.
func isTailscaleIngress(ingress *networkingv1.Ingress) bool {
	if ingress.Spec.IngressClassName != nil && *ingress.Spec.IngressClassName == "tailscale" {
		return true
	}
	// Also check the legacy annotation used by some versions of the operator
	if ingress.Annotations["kubernetes.io/ingress.class"] == "tailscale" {
		return true
	}
	return false
}

// extractIngressInfo converts a Kubernetes ingress to our simplified structure
func (k *K8sClient) extractIngressInfo(ingress *networkingv1.Ingress) IngressInfo {
	name := ingress.Name
	if strings.HasSuffix(name, "-ingress") {
		name = strings.TrimSuffix(name, "-ingress")
	}

	info := IngressInfo{
		Name:            name,
		Tailscale:       isTailscaleIngress(ingress),
		TailscaleFunnel: isTailscaleIngress(ingress) && ingress.Annotations["tailscale.com/funnel"] == "true",
	}

	// Extract the first path from spec rules if available
	if len(ingress.Spec.Rules) > 0 {
		rule := ingress.Spec.Rules[0]
		if rule.HTTP != nil && len(rule.HTTP.Paths) > 0 {
			info.Path = rule.HTTP.Paths[0].Path
		}
	}

	if info.Tailscale {
		// Tailscale ingresses use a wildcard host in spec.rules; the real hostname is
		// assigned by the operator and published in the load balancer status.
		for _, lb := range ingress.Status.LoadBalancer.Ingress {
			if lb.Hostname != "" {
				info.Host = lb.Hostname
				break
			}
		}
		// Tailscale always terminates TLS for both VPN-only and Funnel ingresses.
		if info.Host != "" {
			info.URL = fmt.Sprintf("https://%s%s", info.Host, info.Path)
		}
	} else {
		// Standard ingress: host comes from spec.rules
		if len(ingress.Spec.Rules) > 0 {
			info.Host = ingress.Spec.Rules[0].Host
		}

		// Determine the protocol by checking for a matching TLS entry
		protocol := "http"
		for _, tls := range ingress.Spec.TLS {
			for _, host := range tls.Hosts {
				if host == info.Host {
					protocol = "https"
					break
				}
			}
		}

		if info.Host != "" {
			info.URL = fmt.Sprintf("%s://%s%s", protocol, info.Host, info.Path)
		}
	}

	return info
}

// getDemoIngresses returns example ingresses for demo mode
func (k *K8sClient) getDemoIngresses() []IngressInfo {
	return []IngressInfo{
		{
			Name: "grafana",
			Host: "grafana.example.com",
			Path: "/",
			URL:  "https://grafana.example.com/",
		},
		{
			Name: "home-assistant",
			Host: "hass.example.com",
			Path: "/",
			URL:  "https://hass.example.com/",
		},
		{
			Name: "jellyfin",
			Host: "media.example.com",
			Path: "/",
			URL:  "https://media.example.com/",
		},
		{
			Name: "nextcloud",
			Host: "cloud.example.com",
			Path: "/",
			URL:  "https://cloud.example.com/",
		},
		{
			Name:      "open-webui",
			Host:      "ai.example-tailnet.ts.net",
			Path:      "/",
			URL:       "https://ai.example-tailnet.ts.net/",
			Tailscale: true,
		},
		{
			Name:            "open-webui-funnel",
			Host:            "ai.snowy-galaxy.ts.net",
			Path:            "/",
			URL:             "https://ai.snowy-galaxy.ts.net/",
			Tailscale:       true,
			TailscaleFunnel: true,
		},
		{
			Name: "portainer",
			Host: "portainer.example.com",
			Path: "/",
			URL:  "https://portainer.example.com/",
		},
	}
}
