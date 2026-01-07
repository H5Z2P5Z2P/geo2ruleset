// Surge-Geosite Go Server
// A Geosite Ruleset Converter for Surge
package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/xxxbrian/surge-geosite/internal/cache"
	"github.com/xxxbrian/surge-geosite/internal/fetcher"
	"github.com/xxxbrian/surge-geosite/internal/server"
)

func main() {
	// Parse command-line flags
	port := flag.String("port", envOrDefault("GEO_PORT", "8080"), "Port to listen on")
	baseURL := flag.String("base-url", envOrDefault("GEO_BASE_URL", ""), "Base URL for generated index (optional)")
	indexPath := flag.String("index-path", envOrDefault("GEO_INDEX_PATH", ""), "Local index.json file path (optional)")
	repoURL := flag.String("repo-url", envOrDefault("GEO_REPO_URL", "https://github.com/xxxbrian/Surge-Geosite"), "Repository URL for root redirect")
	miscBaseURL := flag.String("misc-base-url", envOrDefault("GEO_MISC_BASE_URL", "https://raw.githubusercontent.com/xxxbrian/Surge-Geosite/refs/heads/main/misc"), "Base URL for misc lists")
	zipTTL := flag.Duration("zip-ttl", 30*time.Minute, "ZIP cache TTL")
	resultTTL := flag.Duration("result-ttl", 24*time.Hour, "Result cache TTL")
	zipCachePath := flag.String("zip-cache-path", "", "ZIP cache persistence file path (optional)")
	refreshInterval := flag.Duration("zip-refresh-interval", 30*time.Minute, "Interval to refresh ZIP cache (0 to disable)")
	komariAPIKey := flag.String("komari-api-key", envOrDefault("KOMARI_API_KEY", ""), "Komari API key for IP CIDR ruleset")
	komariBaseURL := flag.String("komari-base-url", envOrDefault("KOMARI_BASE_URL", ""), "Komari API base URL (e.g. https://komari.example.com)")
	flag.Parse()

	// Initialize caches
	zipCache := cache.NewZipCache(*zipTTL)
	resultCache := cache.NewResultCache(*resultTTL)
	if *zipCachePath != "" {
		zipCache.SetPersistPath(*zipCachePath)
		if err := zipCache.LoadFromFile(*zipCachePath); err != nil {
			if !os.IsNotExist(err) {
				log.Printf("Failed to load ZIP cache from %s: %v", *zipCachePath, err)
			}
		} else {
			log.Printf("Loaded ZIP cache from %s", *zipCachePath)
		}
	}

	// Initialize fetcher
	f := fetcher.NewFetcher(zipCache)

	// Initialize server
	srv := server.NewServer(f, resultCache, server.Config{
		IndexPath:     *indexPath,
		BaseURL:       *baseURL,
		RepoURL:       *repoURL,
		MiscBaseURL:   *miscBaseURL,
		KomariAPIKey:  *komariAPIKey,
		KomariBaseURL: *komariBaseURL,
	})
	if err := srv.RefreshIndex(); err != nil {
		log.Printf("Index refresh failed: %v", err)
	}

	// Setup routes
	mux := http.NewServeMux()
	srv.SetupRoutes(mux)

	// Apply logging middleware
	handler := server.LoggingMiddleware(mux)

	// Start cache cleanup goroutine
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			resultCache.Cleanup()
		}
	}()

	// Start ZIP refresh goroutine
	if *refreshInterval > 0 {
		go func() {
			refresh := func() {
				beforeETag := zipCache.GetETag()
				_, afterETag, err := f.RefreshZipReader()
				if err != nil {
					log.Printf("ZIP refresh failed: %v", err)
					return
				}
				if afterETag != "" && afterETag != beforeETag {
					log.Printf("ZIP cache refreshed (etag %s)", afterETag)
				}
				if err := srv.RefreshIndex(); err != nil {
					log.Printf("Index refresh failed: %v", err)
				}
			}
			refresh()
			ticker := time.NewTicker(*refreshInterval)
			defer ticker.Stop()
			for range ticker.C {
				refresh()
			}
		}()
	}

	// Start server
	addr := ":" + *port
	log.Printf("Starting Surge-Geosite server on %s", addr)
	log.Printf("ZIP cache TTL: %v, Result cache TTL: %v", *zipTTL, *resultTTL)
	if *indexPath != "" {
		log.Printf("Index path: %s", *indexPath)
	}
	if *baseURL != "" {
		log.Printf("Base URL: %s", *baseURL)
	}
	if *zipCachePath != "" {
		log.Printf("ZIP cache persistence: %s", *zipCachePath)
	}
	if *refreshInterval > 0 {
		log.Printf("ZIP refresh interval: %v", *refreshInterval)
	}
	if *komariAPIKey != "" {
		log.Printf("Komari API enabled for IP CIDR ruleset")
	}

	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func envOrDefault(key string, def string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return def
	}
	return value
}
