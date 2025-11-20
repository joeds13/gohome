package internal

import (
	"context"
	"fmt"
	"log"
	"sort"

	"os"
	"path/filepath"

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
	Name      string
	Namespace string
	Host      string
	Path      string
	URL       string
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

// extractIngressInfo converts a Kubernetes ingress to our simplified structure
func (k *K8sClient) extractIngressInfo(ingress *networkingv1.Ingress) IngressInfo {
	info := IngressInfo{
		Name:      ingress.Name,
		Namespace: ingress.Namespace,
	}

	// Extract the first rule and host
	if len(ingress.Spec.Rules) > 0 {
		rule := ingress.Spec.Rules[0]
		info.Host = rule.Host

		// Extract the first path if available
		if rule.HTTP != nil && len(rule.HTTP.Paths) > 0 {
			info.Path = rule.HTTP.Paths[0].Path
		}

		// Determine the protocol (check for TLS)
		protocol := "http"
		if len(ingress.Spec.TLS) > 0 {
			for _, tls := range ingress.Spec.TLS {
				for _, host := range tls.Hosts {
					if host == info.Host {
						protocol = "https"
						break
					}
				}
			}
		}

		// Construct the URL
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
			Name:      "grafana",
			Namespace: "monitoring",
			Host:      "grafana.example.com",
			Path:      "/",
			URL:       "https://grafana.example.com/",
		},
		{
			Name:      "home-assistant",
			Namespace: "home-automation",
			Host:      "hass.example.com",
			Path:      "/",
			URL:       "https://hass.example.com/",
		},
		{
			Name:      "jellyfin",
			Namespace: "media",
			Host:      "media.example.com",
			Path:      "/",
			URL:       "https://media.example.com/",
		},
		{
			Name:      "nextcloud",
			Namespace: "productivity",
			Host:      "cloud.example.com",
			Path:      "/",
			URL:       "https://cloud.example.com/",
		},
		{
			Name:      "portainer",
			Namespace: "management",
			Host:      "portainer.example.com",
			Path:      "/",
			URL:       "https://portainer.example.com/",
		},
	}
}
