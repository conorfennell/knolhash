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
	matches, remaining, err := FetchLiveMatches(c.apiKey)
	if err != nil {
		slog.Warn("worldcup: live fetch failed", "error", err)
		return 2 * time.Minute
	}

	c.mu.Lock()
	c.matches = matches
	c.mu.Unlock()

	slog.Info("worldcup: live poll", "live", len(matches), "quota_remaining", remaining)

	if remaining < 3 {
		slog.Warn("worldcup: live feed quota low, backing off", "remaining", remaining)
		return 90 * time.Second
	}
	if len(matches) > 0 {
		return 60 * time.Second
	}
	return 5 * time.Minute
}
