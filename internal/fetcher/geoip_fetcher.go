package fetcher

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/xxxbrian/surge-geosite/internal/cache"
)

const (
	DefaultGeoIPURL = "https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest/geoip-lite.db"
)

type GeoIPFetcher struct {
	client *http.Client
	url    string
	cache  *cache.ZipCache // Reusing ZipCache structure for binary storage
}

func NewGeoIPFetcher(url string) *GeoIPFetcher {
	if url == "" {
		url = DefaultGeoIPURL
	}
	return &GeoIPFetcher{
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
		url:   url,
		cache: cache.NewZipCache(24 * time.Hour), // 24h caching
	}
}

// GetDB returns the cached or freshly downloaded DB bytes
func (f *GeoIPFetcher) GetDB() ([]byte, error) {
	// Try cache first
	data, _, ok := f.cache.GetAny()
	if ok && data != nil {
		return nil, fmt.Errorf("not implemented")
	}
	return f.download()
}

func (f *GeoIPFetcher) download() ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, f.url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download failed: %s", resp.Status)
	}

	return io.ReadAll(resp.Body)
}
