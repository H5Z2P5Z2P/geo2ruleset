// Package fetcher handles downloading and extracting the domain-list-community ZIP file.
package fetcher

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/xxxbrian/surge-geosite/internal/cache"
)

const (
	zipURL    = "https://github.com/v2fly/domain-list-community/archive/refs/heads/master.zip"
	userAgent = "Surge-Geosite-Go/1.0"
)

// Fetcher handles ZIP file operations
type Fetcher struct {
	client   *http.Client
	zipCache *cache.ZipCache
}

// NewFetcher creates a new Fetcher
func NewFetcher(zipCache *cache.ZipCache) *Fetcher {
	return &Fetcher{
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
		zipCache: zipCache,
	}
}

// GetETag fetches the ETag from GitHub without downloading the full file
func (f *Fetcher) GetETag() (string, error) {
	req, err := http.NewRequest(http.MethodHead, zipURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := f.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HEAD request failed: %s", resp.Status)
	}

	etag := resp.Header.Get("ETag")
	// Clean up ETag (remove quotes and W/ prefix if present)
	etag = strings.ReplaceAll(etag, "\"", "")
	etag = strings.TrimPrefix(etag, "W/")

	return etag, nil
}

// GetZipReader returns a cached or freshly downloaded zip.Reader
func (f *Fetcher) GetZipReader() (*zip.Reader, string, error) {
	// Try cache first
	reader, etag, ok := f.zipCache.Get()
	if ok {
		return reader, etag, nil
	}

	// Check if ETag changed
	newETag, err := f.GetETag()
	if err != nil {
		// If we have cached data, use it even if ETag check failed
		if reader != nil {
			return reader, etag, nil
		}
		return nil, "", fmt.Errorf("failed to get ETag: %w", err)
	}

	// If ETag hasn't changed and we have valid cached reader
	if etag == newETag && reader != nil {
		return reader, etag, nil
	}

	// Download new ZIP
	data, err := f.downloadZip()
	if err != nil {
		return nil, "", err
	}

	// Update cache
	if err := f.zipCache.Set(data, newETag); err != nil {
		return nil, "", fmt.Errorf("failed to set cache: %w", err)
	}

	reader, _, _ = f.zipCache.Get()
	return reader, newETag, nil
}

// RefreshZipReader checks upstream for updates regardless of TTL.
func (f *Fetcher) RefreshZipReader() (*zip.Reader, string, error) {
	reader, etag, _ := f.zipCache.GetAny()

	newETag, err := f.GetETag()
	if err != nil {
		if reader != nil {
			return reader, etag, nil
		}
		return nil, "", fmt.Errorf("failed to get ETag: %w", err)
	}

	if etag == newETag && reader != nil {
		return reader, etag, nil
	}

	data, err := f.downloadZip()
	if err != nil {
		return nil, "", err
	}

	if err := f.zipCache.Set(data, newETag); err != nil {
		return nil, "", fmt.Errorf("failed to set cache: %w", err)
	}

	reader, _, _ = f.zipCache.GetAny()
	return reader, newETag, nil
}

// downloadZip downloads the ZIP file from GitHub
func (f *Fetcher) downloadZip() ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, zipURL, nil)
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

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return data, nil
}

// GetFileContent reads a file from the ZIP archive
func (f *Fetcher) GetFileContent(reader *zip.Reader, name string) (string, error) {
	filePath := fmt.Sprintf("domain-list-community-master/data/%s", name)

	for _, file := range reader.File {
		if file.Name == filePath {
			rc, err := file.Open()
			if err != nil {
				return "", err
			}
			defer rc.Close()

			content, err := io.ReadAll(rc)
			if err != nil {
				return "", err
			}
			return string(content), nil
		}
	}

	return "", fmt.Errorf("file not found: %s", filePath)
}
