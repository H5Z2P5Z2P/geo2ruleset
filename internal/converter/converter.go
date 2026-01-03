// Package converter handles the conversion of v2fly domain list format to Surge ruleset format.
package converter

import (
	"archive/zip"
)

// Converter handles rule conversion
type Converter struct {
	zipReader  *zip.Reader
	fileGetter func(reader *zip.Reader, name string) (string, error)
}

// NewConverter creates a new Converter
func NewConverter(zipReader *zip.Reader, fileGetter func(reader *zip.Reader, name string) (string, error)) *Converter {
	return &Converter{
		zipReader:  zipReader,
		fileGetter: fileGetter,
	}
}

// Convert converts upstream content to Surge ruleset format
func (c *Converter) Convert(upstreamContent string, filter string) (string, error) {
	items, err := c.Parse(upstreamContent, filter)
	if err != nil {
		return "", err
	}
	return RenderSurge(items), nil
}

// ConvertMihomo converts upstream content to Mihomo ruleset format.
func (c *Converter) ConvertMihomo(upstreamContent string, filter string) (string, error) {
	items, err := c.Parse(upstreamContent, filter)
	if err != nil {
		return "", err
	}
	return RenderMihomo(items), nil
}

// ConvertEgern converts upstream content to Egern ruleset YAML.
func (c *Converter) ConvertEgern(upstreamContent string, filter string) (string, error) {
	items, err := c.Parse(upstreamContent, filter)
	if err != nil {
		return "", err
	}
	return RenderEgern(items), nil
}
