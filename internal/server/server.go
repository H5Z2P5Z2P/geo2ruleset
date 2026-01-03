// Package server provides the HTTP server and routing.
package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/xxxbrian/surge-geosite/internal/cache"
	"github.com/xxxbrian/surge-geosite/internal/converter"
	"github.com/xxxbrian/surge-geosite/internal/fetcher"
)

// Server represents the HTTP server
type Server struct {
	fetcher     *fetcher.Fetcher
	resultCache *cache.ResultCache
	httpClient  *http.Client
	indexURL    string
	repoURL     string
	miscBaseURL string
}

// NewServer creates a new Server
func NewServer(f *fetcher.Fetcher, rc *cache.ResultCache) *Server {
	return &Server{
		fetcher:     f,
		resultCache: rc,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		indexURL:    "https://raw.githubusercontent.com/xxxbrian/Surge-Geosite/main/index.json",
		repoURL:     "https://github.com/xxxbrian/Surge-Geosite",
		miscBaseURL: "https://raw.githubusercontent.com/xxxbrian/Surge-Geosite/refs/heads/main/misc",
	}
}

// SetupRoutes configures the HTTP routes
func (s *Server) SetupRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/", s.handleRoot)
	mux.HandleFunc("/geosite", s.handleGeositeIndex)
	mux.HandleFunc("/geosite/", s.handleGeosite)
	mux.HandleFunc("/geosite/surge", s.handleGeositeIndex)
	mux.HandleFunc("/geosite/surge/", s.handleSurge)
	mux.HandleFunc("/geosite/mihomo", s.handleGeositeIndex)
	mux.HandleFunc("/geosite/mihomo/", s.handleMihomo)
	mux.HandleFunc("/geosite/egern", s.handleGeositeIndex)
	mux.HandleFunc("/geosite/egern/", s.handleEgern)
	mux.HandleFunc("/misc/", s.handleMisc)
}

// handleRoot redirects to GitHub repository
func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, s.repoURL, http.StatusFound)
}

// handleGeositeIndex returns the JSON index of available geosites
func (s *Server) handleGeositeIndex(w http.ResponseWriter, r *http.Request) {
	resp, err := s.httpClient.Get(s.indexURL)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to fetch index: %v", err), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		http.Error(w, fmt.Sprintf("Failed to fetch index: %s", resp.Status), http.StatusInternalServerError)
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to read response: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=1800")
	w.Write(body)
}

// handleGeosite handles /geosite/:name_with_filter requests
func (s *Server) handleGeosite(w http.ResponseWriter, r *http.Request) {
	s.handleRuleset(w, r, "/geosite/", "geosite")
}

// handleSurge handles /geosite/surge/:name_with_filter requests
func (s *Server) handleSurge(w http.ResponseWriter, r *http.Request) {
	s.handleRuleset(w, r, "/geosite/surge/", "geosite")
}

// handleMihomo handles /mihomo/:name_with_filter requests
func (s *Server) handleMihomo(w http.ResponseWriter, r *http.Request) {
	s.handleRuleset(w, r, "/geosite/mihomo/", "mihomo")
}

// handleEgern handles /egern/:name_with_filter requests
func (s *Server) handleEgern(w http.ResponseWriter, r *http.Request) {
	s.handleRuleset(w, r, "/geosite/egern/", "egern")
}

func (s *Server) handleRuleset(w http.ResponseWriter, r *http.Request, prefix string, format string) {
	nameWithFilter := strings.TrimPrefix(r.URL.Path, prefix)
	nameWithFilter = strings.ToLower(strings.TrimSpace(nameWithFilter))

	if nameWithFilter == "" {
		http.Error(w, "Invalid name parameter", http.StatusBadRequest)
		return
	}

	var name, filter string
	if strings.Contains(nameWithFilter, "@") {
		parts := strings.SplitN(nameWithFilter, "@", 2)
		name = parts[0]
		filter = parts[1]
	} else {
		name = nameWithFilter
	}

	if name == "" {
		http.Error(w, "Invalid name parameter", http.StatusBadRequest)
		return
	}

	zipReader, etag, err := s.fetcher.GetZipReader()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to fetch upstream: %v", err), http.StatusInternalServerError)
		return
	}

	cacheKey := format + ":" + nameWithFilter
	if result, ok := s.resultCache.Get(cacheKey, etag); ok {
		log.Printf("Cache hit for %s (ETag %s)", cacheKey, truncateETag(etag))
		s.writeRulesetResponse(w, format, result)
		return
	}

	log.Printf("Cache miss for %s, generating...", cacheKey)

	upstreamContent, err := s.fetcher.GetFileContent(zipReader, name)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get upstream content: %v", err), http.StatusInternalServerError)
		return
	}

	conv := converter.NewConverter(zipReader, s.fetcher.GetFileContent)
	var output string
	switch format {
	case "mihomo":
		output, err = conv.ConvertMihomo(upstreamContent, filter)
	case "egern":
		output, err = conv.ConvertEgern(upstreamContent, filter)
	default:
		output, err = conv.Convert(upstreamContent, filter)
	}
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to convert: %v", err), http.StatusInternalServerError)
		return
	}

	s.resultCache.Set(cacheKey, output, etag)

	log.Printf("Generated and cached result for %s (ETag %s)", cacheKey, truncateETag(etag))

	s.writeRulesetResponse(w, format, output)
}

func (s *Server) writeRulesetResponse(w http.ResponseWriter, format string, body string) {
	contentType := "text/plain; charset=utf-8"
	if format == "egern" {
		contentType = "text/yaml; charset=utf-8"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=1800")
	w.Write([]byte(body))
}

// handleMisc handles /misc/:category/:name requests
func (s *Server) handleMisc(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/misc/")
	parts := strings.Split(path, "/")

	if len(parts) != 2 {
		http.Error(w, "Invalid path format, expected /misc/:category/:name", http.StatusBadRequest)
		return
	}

	category := strings.ToLower(parts[0])
	name := strings.ToLower(parts[1])

	url := fmt.Sprintf("%s/%s/%s.list", s.miscBaseURL, category, name)

	resp, err := s.httpClient.Get(url)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to fetch misc content: %v", err), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		http.Error(w, fmt.Sprintf("Failed to fetch misc content: %s", resp.Status), http.StatusInternalServerError)
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to read response: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=1800")
	w.Write(body)
}

// LoggingMiddleware logs all HTTP requests
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

// truncateETag truncates ETag for logging
func truncateETag(etag string) string {
	if len(etag) > 8 {
		return etag[:8]
	}
	return etag
}

// Compile-time check to ensure json is used (for index parsing)
var _ = json.Marshal
