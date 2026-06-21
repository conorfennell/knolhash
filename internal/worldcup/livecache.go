package worldcup

import (
	"log/slog"
	"sync"
	"time"
)

// LiveCache polls ESPN for live match scores.
type LiveCache struct {
	mu      sync.RWMutex
	matches []LiveMatch
}

func NewLiveCache() *LiveCache {
	return &LiveCache{}
}

// Get returns the current live matches (empty slice when nothing is live).
func (c *LiveCache) Get() []LiveMatch {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]LiveMatch, len(c.matches))
	copy(out, c.matches)
	return out
}

// Start launches the background polling goroutine.
func (c *LiveCache) Start() {
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
	matches, _, err := FetchLiveMatchesESPN()
	if err != nil {
		slog.Warn("worldcup: ESPN live fetch failed", "error", err)
		return 2 * time.Minute
	}

	c.mu.Lock()
	c.matches = matches
	c.mu.Unlock()

	slog.Info("worldcup: live poll", "live", len(matches), "source", "espn")

	if len(matches) > 0 {
		return 10 * time.Second
	}
	return 3 * time.Minute
}
