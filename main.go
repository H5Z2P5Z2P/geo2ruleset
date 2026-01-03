// Surge-Geosite Go Server
// A Geosite Ruleset Converter for Surge
package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/xxxbrian/surge-geosite/internal/cache"
	"github.com/xxxbrian/surge-geosite/internal/fetcher"
	"github.com/xxxbrian/surge-geosite/internal/server"
)

func main() {
	// Parse command-line flags
	port := flag.String("port", "8080", "Port to listen on")
	zipTTL := flag.Duration("zip-ttl", 30*time.Minute, "ZIP cache TTL")
	resultTTL := flag.Duration("result-ttl", 24*time.Hour, "Result cache TTL")
	zipCachePath := flag.String("zip-cache-path", "", "ZIP cache persistence file path (optional)")
	refreshInterval := flag.Duration("zip-refresh-interval", 30*time.Minute, "Interval to refresh ZIP cache (0 to disable)")
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
	srv := server.NewServer(f, resultCache)

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
	if *zipCachePath != "" {
		log.Printf("ZIP cache persistence: %s", *zipCachePath)
	}
	if *refreshInterval > 0 {
		log.Printf("ZIP refresh interval: %v", *refreshInterval)
	}

	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
