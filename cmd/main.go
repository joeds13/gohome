package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"gohome/internal"
)

var (
	// Version information (set via ldflags during build)
	Version   = "dev"
	BuildTime = "unknown"
)

func main() {
	// Parse command line flags
	var showVersion = flag.Bool("version", false, "Show version information")
	var showHelp = flag.Bool("help", false, "Show help information")
	flag.Parse()

	if *showVersion {
		fmt.Printf("GoHome %s (built %s)\n", Version, BuildTime)
		os.Exit(0)
	}

	if *showHelp {
		fmt.Printf("GoHome %s - Kubernetes Personal Homepage\n\n", Version)
		fmt.Println("Usage:")
		fmt.Println("  gohome [flags]")
		fmt.Println()
		fmt.Println("Flags:")
		flag.PrintDefaults()
		fmt.Println()
		fmt.Println("Environment Variables:")
		fmt.Println("  PORT              Server port (default: 8080)")
		fmt.Println("  NAMESPACE         Kubernetes namespace (default: default)")
		fmt.Println("  CONFIG_MAP_NAME   ConfigMap name for bookmarks (default: gohome-config)")
		fmt.Println()
		fmt.Println("For more information, visit: https://github.com/joeds13/gohome")
		os.Exit(0)
	}
	// Get configuration from environment
	namespace := os.Getenv("NAMESPACE")
	if namespace == "" {
		namespace = "default"
	}

	configMapName := os.Getenv("CONFIG_MAP_NAME")
	if configMapName == "" {
		configMapName = "gohome-config"
	}

	// Initialize Kubernetes client
	k8sClient, err := internal.NewK8sClient()
	if err != nil {
		log.Printf("Warning: Failed to initialize Kubernetes client: %v", err)
		log.Println("Running in demo mode without Kubernetes integration")
		k8sClient = nil
	}

	// Initialize bookmark manager
	var bookmarkManager *internal.BookmarkManager
	if k8sClient != nil {
		bookmarkManager = internal.NewBookmarkManager(k8sClient.GetClientset(), namespace, configMapName)
	} else {
		// Create a nil bookmark manager for demo mode
		bookmarkManager = internal.NewBookmarkManager(nil, namespace, configMapName)
	}

	// Create and start server
	server, err := internal.NewServer(k8sClient, bookmarkManager)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	log.Printf("Starting GoHome %s (built %s)...", Version, BuildTime)
	if err := server.Start(); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
