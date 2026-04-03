package internal

import (
	"context"
	"html/template"
	"log"
	"net"
	"net/http"
	"os"
	"time"
)

// Server represents the HTTP server
type Server struct {
	k8sClient       *K8sClient
	bookmarkManager *BookmarkManager
	templates       *template.Template
	port            string
	mux             *http.ServeMux
}

// PageData represents the data passed to templates
type PageData struct {
	Config    *Config
	Ingresses []IngressInfo
	Error     string
	DemoMode  bool
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

	s := &Server{
		k8sClient:       k8sClient,
		bookmarkManager: bookmarkManager,
		templates:       templates,
		port:            port,
		mux:             http.NewServeMux(),
	}

	s.mux.HandleFunc("/", s.handleHome)
	s.mux.HandleFunc("/health", s.handleHealth)
	s.mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static/"))))

	return s, nil
}

// Handler returns the HTTP handler for the server, so it can be served
// over any listener (local TCP, tsnet, etc.).
func (s *Server) Handler() http.Handler {
	return s.mux
}

// Start starts the HTTP server on the configured local port.
func (s *Server) Start() error {
	log.Printf("Server starting on port %s", s.port)
	return http.ListenAndServe(":"+s.port, s.mux)
}

// ServeListener serves the HTTP handler over an already-established net.Listener.
// This is used to serve over a tsnet listener.
func (s *Server) ServeListener(l net.Listener) error {
	srv := &http.Server{Handler: s.mux}
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

	// Load ingresses
	ingresses, err := s.k8sClient.GetVisibleIngresses(ctx)
	if err != nil {
		log.Printf("Warning: Error loading ingresses: %v", err)
		// Continue with empty ingresses list instead of failing
		ingresses = []IngressInfo{}
	}

	// Prepare page data
	data := PageData{
		Config:    config,
		Ingresses: ingresses,
		DemoMode:  s.k8sClient == nil,
	}

	// Render template
	err = s.templates.ExecuteTemplate(w, "index.html", data)
	if err != nil {
		log.Printf("Error rendering template: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
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
		Ingresses: []IngressInfo{},
		DemoMode:  s.k8sClient == nil,
	}

	err := s.templates.ExecuteTemplate(w, "index.html", data)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
