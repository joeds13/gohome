package internal

import (
	"context"
	"html/template"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"tailscale.com/client/local"
)

// Server represents the HTTP server
type Server struct {
	k8sClient            *K8sClient
	bookmarkManager      *BookmarkManager
	templates            *template.Template
	port                 string
	mux                  *http.ServeMux
	handler              http.Handler // instrumented handler, built once, shared by all listeners
	tsLocalClient        *local.Client
	appsDisplayed        prometheus.Gauge
	servicesDisplayed    prometheus.Gauge
	uniqueVisitors       *prometheus.GaugeVec
	seenVisitors         map[string]struct{}
	seenVisitorsMu       sync.Mutex
	httpRequestsInFlight prometheus.Gauge
	httpRequestsTotal    *prometheus.CounterVec
	httpRequestDuration  *prometheus.HistogramVec
}

// PageData represents the data passed to templates
type PageData struct {
	Config        *Config
	Apps          []IngressInfo
	Services      []IngressInfo
	Error         string
	DemoMode      bool
	TailscaleUser string // email of the viewing tailnet peer, empty for local requests
}

// NewServer creates a new HTTP server
func NewServer(k8sClient *K8sClient, bookmarkManager *BookmarkManager) (*Server, error) {
	// Parse templates
	templates, err := template.ParseGlob("templates/*.html")
	if err != nil {
		return nil, err
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	httpRequestsInFlight := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "gohome_http_requests_in_flight",
		Help: "Current number of HTTP requests being served.",
	})
	prometheus.MustRegister(httpRequestsInFlight)

	httpRequestsTotal := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "gohome_http_requests_total",
		Help: "Total number of HTTP requests by status code and method.",
	}, []string{"code", "method"})
	prometheus.MustRegister(httpRequestsTotal)

	httpRequestDuration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "gohome_http_request_duration_seconds",
		Help:    "HTTP request duration in seconds by status code and method.",
		Buckets: prometheus.DefBuckets,
	}, []string{"code", "method"})
	prometheus.MustRegister(httpRequestDuration)

	appsDisplayed := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "gohome_apps_displayed",
		Help: "Number of apps currently displayed on the homepage.",
	})
	prometheus.MustRegister(appsDisplayed)

	servicesDisplayed := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "gohome_services_displayed",
		Help: "Number of services currently displayed on the homepage.",
	})
	prometheus.MustRegister(servicesDisplayed)

	uniqueVisitors := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "gohome_unique_visitors",
		Help: "Unique visitors that have loaded the homepage, labelled by their Tailscale email.",
	}, []string{"email"})
	prometheus.MustRegister(uniqueVisitors)

	mux := http.NewServeMux()

	s := &Server{
		k8sClient:            k8sClient,
		bookmarkManager:      bookmarkManager,
		templates:            templates,
		port:                 port,
		mux:                  mux,
		appsDisplayed:        appsDisplayed,
		servicesDisplayed:    servicesDisplayed,
		uniqueVisitors:       uniqueVisitors,
		seenVisitors:         make(map[string]struct{}),
		httpRequestsInFlight: httpRequestsInFlight,
		httpRequestsTotal:    httpRequestsTotal,
		httpRequestDuration:  httpRequestDuration,
	}

	s.mux.HandleFunc("/", s.handleHome)
	s.mux.HandleFunc("/health", s.handleHealth)
	s.mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static/"))))
	s.mux.Handle("/metrics", promhttp.Handler())

	// Build the instrumented handler once so that both the local TCP listener
	// and the tsnet listener share a single middleware chain and a single
	// in-flight gauge. Constructing it twice would still point at the same
	// metric objects, but would create two independent chain instances and
	// make the sharing implicit rather than guaranteed.
	s.handler = promhttp.InstrumentHandlerInFlight(s.httpRequestsInFlight,
		promhttp.InstrumentHandlerCounter(s.httpRequestsTotal,
			promhttp.InstrumentHandlerDuration(s.httpRequestDuration,
				s.mux,
			),
		),
	)

	return s, nil
}

// Handler returns the shared instrumented handler for the server, so it can
// be served over any listener (local TCP, tsnet, etc.) with all listeners
// contributing to the same set of metrics.
func (s *Server) Handler() http.Handler {
	return s.handler
}

// SetTailscaleClient wires up the tsnet LocalClient so that per-request
// WhoIs lookups can resolve the viewing user's identity.
func (s *Server) SetTailscaleClient(lc *local.Client) {
	s.tsLocalClient = lc
}

// Start starts the HTTP server on the configured local port.
func (s *Server) Start() error {
	log.Printf("Server starting on port %s", s.port)
	return http.ListenAndServe(":"+s.port, s.handler)
}

// ServeListener serves the HTTP handler over an already-established net.Listener.
// This is used to serve over a tsnet listener.
func (s *Server) ServeListener(l net.Listener) error {
	srv := &http.Server{Handler: s.handler}
	log.Printf("Serving over listener: %s", l.Addr())
	return srv.Serve(l)
}

// handleHome handles the main homepage
func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// Load configuration and bookmarks
	config, err := s.bookmarkManager.GetConfig(ctx)
	if err != nil {
		log.Printf("Warning: Error loading config: %v", err)
		// Use default config if ConfigMap is not available
		config = &Config{
			Title:     "Go Home",
			Bookmarks: []Bookmark{},
		}
	}

	// Resolve the Tailscale identity of the requesting peer, if available.
	tailscaleUser := s.resolveViewer(ctx, r)

	// Record unique visitors by email. Only set the gauge label the first time
	// we see each address so the series persists across scrapes rather than
	// fluctuating on every request.
	if tailscaleUser != "" {
		s.seenVisitorsMu.Lock()
		if _, seen := s.seenVisitors[tailscaleUser]; !seen {
			s.seenVisitors[tailscaleUser] = struct{}{}
			s.uniqueVisitors.With(prometheus.Labels{"email": tailscaleUser}).Set(1)
		}
		s.seenVisitorsMu.Unlock()
	}

	// Load ingresses
	apps, services, err := s.k8sClient.GetVisibleIngresses(ctx)
	if err != nil {
		log.Printf("Warning: Error loading ingresses: %v", err)
		// Continue with empty slices instead of failing
		apps = []IngressInfo{}
		services = []IngressInfo{}
	}

	// Update the displayed gauges.
	s.appsDisplayed.Set(float64(len(apps)))
	s.servicesDisplayed.Set(float64(len(services)))

	// Prepare page data
	data := PageData{
		Config:        config,
		Apps:          apps,
		Services:      services,
		DemoMode:      s.k8sClient == nil,
		TailscaleUser: tailscaleUser,
	}

	// Render template
	err = s.templates.ExecuteTemplate(w, "index.html", data)
	if err != nil {
		log.Printf("Error rendering template: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}

// resolveViewer returns the Tailscale login name (e.g. "alice@example.com") of
// the user viewing the page, or an empty string if it cannot be determined.
//
// Two paths are supported simultaneously:
//  1. Tailscale Serve / k8s ingress: Tailscale terminates TLS and proxies to
//     the local HTTP port, injecting a "Tailscale-User-Login" header. This is
//     checked first because it is always present on that path and requires no
//     extra round-trip.
//  2. tsnet listener: the app holds its own Tailscale node and the request
//     arrives over a raw net.Listener. No headers are injected, so we fall
//     back to a WhoIs lookup using the request's remote address.
//
// Header spoofing on path 1 is not a concern because Tailscale Serve strips
// any client-supplied Tailscale-* headers before re-adding them itself, and
// the server should only be reachable via localhost in that setup.
func (s *Server) resolveViewer(ctx context.Context, r *http.Request) string {
	// Path 1: Tailscale Serve injects this header.
	if login := r.Header.Get("Tailscale-User-Login"); login != "" {
		return login
	}

	// Path 2: tsnet — resolve by remote address via the local API.
	if s.tsLocalClient != nil {
		if who, err := s.tsLocalClient.WhoIs(ctx, r.RemoteAddr); err == nil {
			if who.UserProfile != nil {
				return who.UserProfile.LoginName
			}
		}
	}

	return ""
}

// handleHealth handles health checks
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// renderError renders an error page
func (s *Server) renderError(w http.ResponseWriter, message string) {
	data := PageData{
		Error: message,
		Config: &Config{
			Title:     "Go Home",
			Bookmarks: []Bookmark{},
		},
		Apps:     []IngressInfo{},
		Services: []IngressInfo{},
		DemoMode: s.k8sClient == nil,
	}

	err := s.templates.ExecuteTemplate(w, "index.html", data)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
