package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"gohome/internal"

	"tailscale.com/tsnet"
)

var (
	// Version information (set via ldflags during build)
	Version   = "2026.19"
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

	tsnetAddr := os.Getenv("TSNET_ADDR")
	if tsnetAddr == "" {
		tsnetAddr = ":443"
	}

	tsnetHostname := os.Getenv("TSNET_HOSTNAME")
	if tsnetHostname == "" {
		tsnetHostname = "gohome"
	}

	// TS_STATE_DIR is the directory where tsnet persists its node identity
	// (private key, node ID, etc.). Mounting a persistent volume at this path
	// and setting TS_STATE_DIR ensures the node keeps the same Tailscale
	// identity across restarts, so the hostname never gets a numeric suffix
	// like "gohome-1". If unset, tsnet uses a temp dir and the node is
	// effectively stateless — Tailscale will append a number each redeploy.
	tsnetStateDir := os.Getenv("TS_STATE_DIR")

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

	// Create the server
	server, err := internal.NewServer(k8sClient, bookmarkManager)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Log the auth key in redacted form so it's visible in logs without
	// exposing the full secret. tsnet reads TS_AUTHKEY automatically when
	// AuthKey is not set on the struct.
	log.Printf("TS_AUTHKEY: %s", internal.RedactAuthKey(os.Getenv("TS_AUTHKEY")))

	// Create and configure the tsnet server.
	// Dir must point at a persistent volume so the node retains its identity
	// (private key + node ID) across restarts. Without this, every restart
	// looks like a brand-new node to Tailscale and it appends a numeric
	// suffix (e.g. "gohome-1") to avoid collisions with the previous ghost.
	tsnetServer := &tsnet.Server{
		Hostname: tsnetHostname,
		Dir:      tsnetStateDir,
	}
	defer tsnetServer.Close()

	// Obtain a TLS listener before starting goroutines so we can fail fast
	// if tailscale is unavailable. ListenTLS automatically fetches and renews
	// a Let's Encrypt certificate for the node's *.ts.net domain via
	// Tailscale's control plane — no cert management required.
	tsListener, err := tsnetServer.ListenTLS("tcp", tsnetAddr)
	if err != nil {
		log.Fatalf("Failed to create tsnet TLS listener on %s: %v", tsnetAddr, err)
	}

	// Wire up the tsnet LocalClient so the server can resolve per-request
	// Tailscale identity via WhoIs.
	lc, err := tsnetServer.LocalClient()
	if err != nil {
		log.Fatalf("Failed to get tsnet local client: %v", err)
	}
	server.SetTailscaleClient(lc)

	log.Printf("Starting GoHome %s (built %s)...", Version, BuildTime)

	errCh := make(chan error, 2)

	// Serve on the local HTTP port
	go func() {
		if err := server.Start(); err != nil {
			errCh <- fmt.Errorf("local server error: %w", err)
		}
	}()

	// Serve the same handler over the tailscale (tsnet) listener
	go func() {
		log.Printf("Serving over tailscale as %q on https://...ts.net%s", tsnetHostname, tsnetAddr)
		if err := server.ServeListener(tsListener); err != nil {
			errCh <- fmt.Errorf("tsnet server error: %w", err)
		}
	}()

	if err := <-errCh; err != nil {
		log.Fatalf("Fatal server error: %v", err)
	}
}
