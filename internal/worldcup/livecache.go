package worldcup

import (
	"log/slog"
	"sync"
	"time"
)

// LiveCache polls football-data.org for live match scores.
// It polls every 60 seconds when matches are in progress and backs off
// to every 5 minutes when nothing is live, to conserve API quota.
type LiveCache struct {
	mu      sync.RWMutex
	matches []LiveMatch
	apiKey  string
}

func NewLiveCache(apiKey string) *LiveCache {
	return &LiveCache{apiKey: apiKey}
}

// Get returns the current live matches (empty slice when nothing is live).
func (c *LiveCache) Get() []LiveMatch {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]LiveMatch, len(c.matches))
	copy(out, c.matches)
	return out
}

// Start launches the background polling goroutine. No-ops if no API key is set.
func (c *LiveCache) Start() {
	if c.apiKey == "" {
		slog.Info("worldcup: live feed disabled (no football_data_api_key in config)")
		return
	}
	go c.run()
	slog.Info("worldcup: live feed started")
}

func (c *LiveCache) run() {
	interval := c.poll()
	for {
		time.Sleep(interval)
		interval = c.poll()
	}
}

func (c *LiveCache) poll() time.Duration {
	// Try ESPN first (no quota, faster).
	matches, _, err := FetchLiveMatchesESPN()
	source := "espn"
	if err != nil {
		slog.Warn("worldcup: ESPN live fetch failed, falling back to football-data.org", "error", err)
		if c.apiKey != "" {
			matches, _, err = FetchLiveMatches(c.apiKey)
			source = "football-data.org"
			if err != nil {
				slog.Warn("worldcup: live fetch failed", "error", err)
				return 2 * time.Minute
			}
		} else {
			return 2 * time.Minute
		}
	}

	c.mu.Lock()
	c.matches = matches
	c.mu.Unlock()

	slog.Info("worldcup: live poll", "live", len(matches), "source", source)

	if len(matches) > 0 {
		return 10 * time.Second
	}
	return 3 * time.Minute
}
