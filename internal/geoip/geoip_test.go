package geoip_test

import (
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/xxxbrian/surge-geosite/internal/geoip"
)

const GeoIPURL = "https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest/geoip-lite.db"

func TestGeoIPIntegration(t *testing.T) {
	// 1. Download real DB
	fmt.Println("Downloading GeoIP DB for testing...")
	resp, err := http.Get(GeoIPURL)
	if err != nil {
		t.Skipf("Skipping test due to network error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Skipf("Skipping test due to download failure: %s", resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read DB: %v", err)
	}

	// 2. Load DB
	g := geoip.NewGeoIP()
	start := time.Now()
	if err := g.Load(data); err != nil {
		t.Fatalf("Failed to load DB: %v", err)
	}
	fmt.Printf("DB loaded in %v\n", time.Since(start))

	// 3. Test lookups
	testCases := []struct {
		code      string
		expectHit bool
	}{
		{"CN", true},
		{"JP", true},
		{"GOOGLE", true},
		{"CLOUDFLARE", true},
		{"NETFLIX", true},
		{"INVALIDXXXX", false},
	}

	for _, tc := range testCases {
		cidrs, found := g.GetCIDRs(tc.code)
		if found != tc.expectHit {
			t.Errorf("GetCIDRs(%s) = %v, want %v", tc.code, found, tc.expectHit)
		}
		if found {
			if len(cidrs) == 0 {
				t.Errorf("GetCIDRs(%s) returned empty list", tc.code)
			} else {
				fmt.Printf("found %d CIDRs for %s (e.g. %s)\n", len(cidrs), tc.code, cidrs[0])
			}
		}
	}
}
