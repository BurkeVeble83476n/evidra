// Package mode resolves the operating mode (online/offline/fallback) for
// evidra CLI and MCP server.
package mode

import (
	"fmt"
	"strings"
	"time"

	"samebits.com/evidra/pkg/client"
)

// Resolved holds the resolved mode and runtime config.
type Resolved struct {
	IsOnline bool
	Client   *client.Client
}

// Config holds all mode-resolution inputs.
type Config struct {
	URL            string        // from EVIDRA_URL or --url
	APIKey         string        // from EVIDRA_API_KEY or --api-key
	FallbackPolicy string        // from EVIDRA_FALLBACK: "closed" or "offline"
	ForceOffline   bool          // from --offline flag
	Timeout        time.Duration // from --timeout (0 = client default)
}

// Resolve determines the operating mode. Does NOT ping the API.
// Returns error only for invalid configuration (e.g. URL set but no API key).
func Resolve(cfg Config) (Resolved, error) {
	if cfg.ForceOffline || strings.TrimSpace(cfg.URL) == "" {
		return Resolved{IsOnline: false}, nil
	}

	if strings.TrimSpace(cfg.APIKey) == "" {
		return Resolved{}, fmt.Errorf("EVIDRA_API_KEY is required when EVIDRA_URL is set")
	}

	c := client.New(client.Config{
		URL:     strings.TrimSpace(cfg.URL),
		APIKey:  strings.TrimSpace(cfg.APIKey),
		Timeout: cfg.Timeout,
	})

	return Resolved{
		IsOnline: true,
		Client:   c,
	}, nil
}
