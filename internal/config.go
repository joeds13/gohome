package internal

import (
	"context"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Bookmark represents a bookmark entry
type Bookmark struct {
	Name     string
	URL      string
	Category string
}

// Config holds the application configuration
type Config struct {
	Bookmarks []Bookmark
	Title     string
}

// BookmarkManager handles bookmark configuration from ConfigMaps
type BookmarkManager struct {
	clientset     *kubernetes.Clientset
	namespace     string
	configMapName string
}

// NewBookmarkManager creates a new bookmark manager
func NewBookmarkManager(clientset *kubernetes.Clientset, namespace, configMapName string) *BookmarkManager {
	return &BookmarkManager{
		clientset:     clientset,
		namespace:     namespace,
		configMapName: configMapName,
	}
}

// LoadBookmarks loads bookmarks from a ConfigMap
func (bm *BookmarkManager) LoadBookmarks(ctx context.Context) ([]Bookmark, error) {
	if bm.clientset == nil {
		log.Printf("Warning: Kubernetes client not available, using default bookmarks")
		return bm.getDefaultBookmarks(), nil
	}

	configMap, err := bm.clientset.CoreV1().ConfigMaps(bm.namespace).Get(ctx, bm.configMapName, metav1.GetOptions{})
	if err != nil {
		log.Printf("Warning: Could not load bookmarks ConfigMap %s/%s: %v", bm.namespace, bm.configMapName, err)
		return bm.getDefaultBookmarks(), nil
	}

	return bm.parseBookmarks(configMap), nil
}

// parseBookmarks parses bookmarks from ConfigMap data
func (bm *BookmarkManager) parseBookmarks(configMap *corev1.ConfigMap) []Bookmark {
	var bookmarks []Bookmark

	// Parse bookmarks from ConfigMap data
	// Expected format: bookmark-name: "url|category"
	for name, value := range configMap.Data {
		if strings.HasPrefix(name, "bookmark-") {
			bookmark := bm.parseBookmarkEntry(name, value)
			if bookmark.URL != "" {
				bookmarks = append(bookmarks, bookmark)
			}
		}
	}

	// Sort bookmarks by category, then by name
	sort.Slice(bookmarks, func(i, j int) bool {
		if bookmarks[i].Category == bookmarks[j].Category {
			return bookmarks[i].Name < bookmarks[j].Name
		}
		return bookmarks[i].Category < bookmarks[j].Category
	})

	return bookmarks
}

// parseBookmarkEntry parses a single bookmark entry
func (bm *BookmarkManager) parseBookmarkEntry(key, value string) Bookmark {
	// Remove "bookmark-" prefix from key to get the name
	name := strings.TrimPrefix(key, "bookmark-")
	name = strings.ReplaceAll(name, "-", " ")
	name = cases.Title(language.English).String(name)

	parts := strings.Split(value, "|")
	bookmark := Bookmark{
		Name: name,
	}

	if len(parts) >= 1 {
		bookmark.URL = strings.TrimSpace(parts[0])
	}
	if len(parts) >= 2 {
		bookmark.Category = strings.TrimSpace(parts[1])
	}

	// Default category if not specified
	if bookmark.Category == "" {
		bookmark.Category = "General"
	}

	return bookmark
}

// GetConfig loads the complete application configuration
func (bm *BookmarkManager) GetConfig(ctx context.Context) (*Config, error) {
	bookmarks, err := bm.LoadBookmarks(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load bookmarks: %w", err)
	}

	// Load title from ConfigMap if available
	title := "Go Home"

	if bm.clientset != nil {
		configMap, err := bm.clientset.CoreV1().ConfigMaps(bm.namespace).Get(ctx, bm.configMapName, metav1.GetOptions{})
		if err == nil {
			if t, exists := configMap.Data["title"]; exists && t != "" {
				title = t
			}
		} else {
			log.Printf("Warning: Could not load ConfigMap for title: %v", err)
		}
	} else {
		log.Printf("Info: Using default title (demo mode)")
	}

	// PAGE_TITLE env var takes highest priority, allowing local overrides
	// (e.g. via mise.toml) without touching the ConfigMap.
	if t := os.Getenv("PAGE_TITLE"); t != "" {
		title = t
	}

	return &Config{
		Bookmarks: bookmarks,
		Title:     title,
	}, nil
}

// getDefaultBookmarks returns a set of example bookmarks when ConfigMap is not available
func (bm *BookmarkManager) getDefaultBookmarks() []Bookmark {
	return []Bookmark{
		{
			Name:     "Hacker News",
			URL:      "https://news.ycombinator.com",
			Category: "News",
		},
		{
			Name:     "Bracket City",
			URL:      "https://www.theatlantic.com/games/bracket-city/",
			Category: "Games",
		},
	}
}

// redactAuthKey returns a redacted but identifiable representation of a
// Tailscale auth key, showing the first 6 and last 4 characters separated by
// "...", e.g. "tskey-...abcd". If the key is too short to redact meaningfully
// it returns a fixed placeholder instead.
func RedactAuthKey(key string) string {
	if key == "" {
		return "(not set)"
	}
	const prefixLen, suffixLen = 6, 4
	if len(key) <= prefixLen+suffixLen {
		return "(set, too short to redact)"
	}
	return key[:prefixLen] + "..." + key[len(key)-suffixLen:]
}
