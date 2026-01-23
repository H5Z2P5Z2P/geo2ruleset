package geoip

import (
	"fmt"
	"strings"
	"sync"

	"github.com/oschwald/maxminddb-golang"
)

type GeoIP struct {
	mu    sync.RWMutex
	cidrs map[string][]string
}

func NewGeoIP() *GeoIP {
	return &GeoIP{
		cidrs: make(map[string][]string),
	}
}

// Load parses the MMDB bytes and builds the in-memory index
func (g *GeoIP) Load(data []byte) error {
	db, err := maxminddb.FromBytes(data)
	if err != nil {
		return fmt.Errorf("failed to open mmdb: %w", err)
	}
	defer db.Close()

	fmt.Printf("GeoIP DB loaded. Size: %d bytes. Metadata: %+v\n", len(data), db.Metadata)

	newCIDRs := make(map[string][]string)

	networks := db.Networks(maxminddb.SkipAliasedNetworks)
	count := 0
	for networks.Next() {
		var record interface{}
		subnet, err := networks.Network(&record)
		if err != nil {
			continue
		}

		var code string
		switch v := record.(type) {
		case string:
			code = v
		case map[string]interface{}:
			if c, ok := v["country"].(map[string]interface{}); ok {
				if iso, ok := c["iso_code"].(string); ok {
					code = iso
				}
			} else if iso, ok := v["iso_code"].(string); ok {
				code = iso
			} else if v["code"] != nil { // Maybe 'code'?
				if s, ok := v["code"].(string); ok {
					code = s
				}
			}
		}

		if code == "" {
			continue
		}

		code = strings.ToUpper(code)
		newCIDRs[code] = append(newCIDRs[code], subnet.String())
		count++
	}
	fmt.Printf("Total GeoIP rules loaded: %d\n", count)

	g.mu.Lock()
	g.cidrs = newCIDRs
	g.mu.Unlock()

	return nil
}

// GetCIDRs returns the list of CIDRs for the given country code or category
func (g *GeoIP) GetCIDRs(code string) ([]string, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	cidrs, ok := g.cidrs[strings.ToUpper(code)]
	return cidrs, ok
}
