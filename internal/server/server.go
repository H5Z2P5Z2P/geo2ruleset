// Package server provides the HTTP server and routing.
package server

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/xxxbrian/surge-geosite/internal/cache"
	"github.com/xxxbrian/surge-geosite/internal/converter"
	"github.com/xxxbrian/surge-geosite/internal/fetcher"
	"github.com/xxxbrian/surge-geosite/internal/komari"
)

// Server represents the HTTP server
type Server struct {
	fetcher      *fetcher.Fetcher
	resultCache  *cache.ResultCache
	httpClient   *http.Client
	komariClient *komari.Client
	indexPath    string
	baseURL      string
	repoURL      string
	miscBaseURL  string
	indexMu      sync.RWMutex
	indexETag    string
	indexBody    []byte
}

// Config contains server configuration.
type Config struct {
	IndexPath     string
	BaseURL       string
	RepoURL       string
	MiscBaseURL   string
	KomariAPIKey  string
	KomariBaseURL string
}

// NewServer creates a new Server
func NewServer(f *fetcher.Fetcher, rc *cache.ResultCache, cfg Config) *Server {
	var kc *komari.Client
	if cfg.KomariAPIKey != "" {
		kc = komari.NewClient(cfg.KomariAPIKey, cfg.KomariBaseURL)
	}
	return &Server{
		fetcher:      f,
		resultCache:  rc,
		komariClient: kc,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		indexPath:   strings.TrimSpace(cfg.IndexPath),
		baseURL:     strings.TrimSuffix(strings.TrimSpace(cfg.BaseURL), "/"),
		repoURL:     cfg.RepoURL,
		miscBaseURL: cfg.MiscBaseURL,
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
	// Komari IP CIDR 路由
	mux.HandleFunc("/komari/ipcidr", s.handleKomariIPCIDR)
	mux.HandleFunc("/komari/ipcidr/", s.handleKomariIPCIDR)
	mux.HandleFunc("/komari/surge/", s.handleKomariSurge)
	mux.HandleFunc("/komari/mihomo/", s.handleKomariMihomo)
	mux.HandleFunc("/komari/egern/", s.handleKomariEgern)
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
	// Priority 1: Read from indexPath file if exists
	if s.indexPath != "" {
		if body, err := os.ReadFile(s.indexPath); err == nil {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "public, max-age=1800")
			_, _ = w.Write(body)
			return
		}
	}

	// Priority 2: Use cached index
	if body, ok := s.getCachedIndex(); ok {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "public, max-age=1800")
		_, _ = w.Write(body)
		return
	}

	// Priority 3: Generate dynamically from request
	if err := s.writeIndexFromZip(w, r); err != nil {
		http.Error(w, fmt.Sprintf("Failed to generate index: %v", err), http.StatusInternalServerError)
	}
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

// handleKomariIPCIDR 处理 /komari/ipcidr 请求
// 支持的路径格式：
// - /komari/ipcidr 或 /komari/ipcidr@DIRECT 或 /komari/ipcidr@PROXY
func (s *Server) handleKomariIPCIDR(w http.ResponseWriter, r *http.Request) {
	s.handleKomariRuleset(w, r, "/komari/ipcidr", "surge")
}

// handleKomariSurge 处理 /komari/surge/{name} 请求
func (s *Server) handleKomariSurge(w http.ResponseWriter, r *http.Request) {
	s.handleKomariRuleset(w, r, "/komari/surge/", "surge")
}

// handleKomariMihomo 处理 /komari/mihomo/{name} 请求
func (s *Server) handleKomariMihomo(w http.ResponseWriter, r *http.Request) {
	s.handleKomariRuleset(w, r, "/komari/mihomo/", "mihomo")
}

// handleKomariEgern 处理 /komari/egern/{name} 请求
func (s *Server) handleKomariEgern(w http.ResponseWriter, r *http.Request) {
	s.handleKomariRuleset(w, r, "/komari/egern/", "egern")
}

// handleKomariRuleset 通用 Komari ruleset 处理函数
func (s *Server) handleKomariRuleset(w http.ResponseWriter, r *http.Request, prefix string, format string) {
	if s.komariClient == nil {
		http.Error(w, "Komari API not configured", http.StatusServiceUnavailable)
		return
	}

	// 解析路径和过滤器
	nameWithFilter := strings.TrimPrefix(r.URL.Path, prefix)
	nameWithFilter = strings.ToLower(strings.TrimSpace(nameWithFilter))

	// 目前只支持 ipcidr
	var name, filterStr string
	if strings.Contains(nameWithFilter, "@") {
		parts := strings.SplitN(nameWithFilter, "@", 2)
		name = parts[0]
		filterStr = strings.ToUpper(parts[1])
	} else {
		name = nameWithFilter
	}

	// 如果是 /komari/ipcidr 路径，name 可能为空或包含过滤器
	if prefix == "/komari/ipcidr" {
		if name == "" {
			name = "ipcidr"
		} else if name != "ipcidr" && !strings.HasPrefix(name, "@") {
			// /komari/ipcidr@DIRECT 的情况
			if strings.HasPrefix(nameWithFilter, "@") {
				filterStr = strings.ToUpper(strings.TrimPrefix(nameWithFilter, "@"))
				name = "ipcidr"
			}
		}
	}

	// 验证名称
	if name != "ipcidr" && name != "" {
		// 其他 ruleset 类型可以在此扩展
		http.Error(w, "Unknown ruleset: "+name+", available: ipcidr", http.StatusNotFound)
		return
	}

	var filter komari.FilterType
	if filterStr != "" {
		filter = komari.FilterType(filterStr)
		if filter != komari.FilterDirect && filter != komari.FilterProxy {
			http.Error(w, "Invalid filter, use @DIRECT or @PROXY", http.StatusBadRequest)
			return
		}
	}

	// 获取服务器列表
	clients, err := s.komariClient.GetClients()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get clients: %v", err), http.StatusInternalServerError)
		return
	}

	// 根据过滤器生成 IP CIDR 规则
	var getPing func(uuid string) int
	if filter != "" {
		getPing = s.komariClient.GetAveragePing
	}
	cidrs := komari.GenerateIPCIDR(clients, filter, getPing)

	// 根据格式渲染输出
	var output string
	var contentType string

	switch format {
	case "egern":
		output = komari.RenderEgern(cidrs)
		contentType = "text/yaml; charset=utf-8"
	case "mihomo":
		output = komari.RenderMihomo(cidrs)
		contentType = "text/plain; charset=utf-8"
	default: // surge
		output = komari.RenderSurge(cidrs)
		contentType = "text/plain; charset=utf-8"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.Write([]byte(output))
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

// RefreshIndex regenerates the index and saves to indexPath if configured.
// Called at startup and when ZIP is refreshed.
func (s *Server) RefreshIndex() error {
	if s.baseURL == "" {
		return nil
	}

	zipReader, etag, err := s.fetcher.GetZipReader()
	if err != nil {
		return err
	}

	// Check if we already have this version cached
	s.indexMu.RLock()
	currentETag := s.indexETag
	s.indexMu.RUnlock()
	if currentETag == etag && s.indexPath != "" {
		// Check if file exists
		if _, err := os.Stat(s.indexPath); err == nil {
			return nil
		}
	}

	// Build index with correct URL format: baseURL + /geosite/ + name
	body, err := s.buildIndexFromZip(zipReader, s.baseURL+"/geosite")
	if err != nil {
		return err
	}

	// Update in-memory cache
	s.setCachedIndex(etag, body)

	// Save to file if indexPath is configured
	if s.indexPath != "" {
		if err := s.saveIndexToFile(body); err != nil {
			log.Printf("Failed to save index to %s: %v", s.indexPath, err)
			return err
		}
		log.Printf("Index saved to %s", s.indexPath)
	}

	return nil
}

func (s *Server) writeIndexFromZip(w http.ResponseWriter, r *http.Request) error {
	zipReader, etag, err := s.fetcher.GetZipReader()
	if err != nil {
		return err
	}

	baseURL := buildBaseURL(r)

	// Check memory cache
	s.indexMu.RLock()
	if s.indexBody != nil && s.indexETag == etag {
		body := s.indexBody
		s.indexMu.RUnlock()
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "public, max-age=1800")
		_, _ = w.Write(body)
		return nil
	}
	s.indexMu.RUnlock()

	body, err := s.buildIndexFromZip(zipReader, baseURL)
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=1800")
	_, _ = w.Write(body)
	return nil
}

func buildBaseURL(r *http.Request) string {
	host := r.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}
	proto := r.Header.Get("X-Forwarded-Proto")
	if proto == "" {
		if r.TLS != nil {
			proto = "https"
		} else {
			proto = "http"
		}
	}
	return proto + "://" + host + "/geosite"
}

func (s *Server) buildIndexFromZip(zipReader *zip.Reader, geositeBaseURL string) ([]byte, error) {
	const dataPrefix = "domain-list-community-master/data/"
	index := make(map[string]string)

	for _, file := range zipReader.File {
		if !strings.HasPrefix(file.Name, dataPrefix) {
			continue
		}
		name := strings.TrimPrefix(file.Name, dataPrefix)
		if name == "" {
			continue
		}
		if strings.Contains(name, "/") {
			continue
		}
		// geositeBaseURL is already like "http://example.com/geosite"
		index[name] = strings.TrimRight(geositeBaseURL, "/") + "/" + name
	}

	ordered := make([]string, 0, len(index))
	for name := range index {
		ordered = append(ordered, name)
	}
	sort.Strings(ordered)

	orderedIndex := make(map[string]string, len(index))
	for _, name := range ordered {
		orderedIndex[name] = index[name]
	}

	return json.MarshalIndent(orderedIndex, "", "  ")
}

func (s *Server) getCachedIndex() ([]byte, bool) {
	s.indexMu.RLock()
	defer s.indexMu.RUnlock()

	if s.indexBody == nil {
		return nil, false
	}
	return s.indexBody, true
}

func (s *Server) setCachedIndex(etag string, body []byte) {
	s.indexMu.Lock()
	defer s.indexMu.Unlock()

	s.indexETag = etag
	s.indexBody = body
}

func (s *Server) saveIndexToFile(body []byte) error {
	if s.indexPath == "" {
		return nil
	}

	// Create directory if needed
	dir := filepath.Dir(s.indexPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Write to temp file first, then rename for atomicity
	tmpPath := s.indexPath + ".tmp"
	if err := os.WriteFile(tmpPath, body, 0644); err != nil {
		return err
	}

	return os.Rename(tmpPath, s.indexPath)
}
