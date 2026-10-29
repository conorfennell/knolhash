package fpl

import (
	"log/slog"
	"sync"
	"time"
)

const refreshInterval = 5 * time.Minute

// Cache holds the most recently fetched LeagueData and refreshes it in the
// background every refreshInterval.
type Cache struct {
	client   *Client
	leagueID int

	refreshMu   sync.Mutex // one fetch at a time; a full fetch is ~10 API calls
	mu          sync.RWMutex
	data        LeagueData
	lastAttempt time.Time // set on failure too, so an FPL outage isn't retried per request
}

// NewCache creates a Cache and performs an initial fetch synchronously.
// If the initial fetch fails, the cache starts empty and the background
// goroutine will retry.
func NewCache(client *Client, leagueID int) *Cache {
	c := &Cache{client: client, leagueID: leagueID}
	c.refresh(true)
	return c
}

// Get returns the cached LeagueData. If the data is stale it triggers a
// synchronous refresh before returning.
func (c *Cache) Get() LeagueData {
	if c.stale() {
		return c.refresh(false)
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.data
}

// NextRefresh is when the cache will next fetch from FPL.
func (c *Cache) NextRefresh() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastAttempt.Add(refreshInterval)
}

// StartBackgroundRefresh starts a goroutine that refreshes the cache every
// refreshInterval. The goroutine runs until the process exits.
func (c *Cache) StartBackgroundRefresh() {
	go func() {
		ticker := time.NewTicker(refreshInterval)
		defer ticker.Stop()
		for range ticker.C {
			c.refresh(true)
		}
	}()
	slog.Info("fpl: background refresh started", "interval", refreshInterval)
}

func (c *Cache) stale() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return time.Since(c.lastAttempt) >= refreshInterval
}

// refresh fetches new data unless (when not forced) another caller already
// refreshed while this one waited for the lock. On failure the previous
// data is kept.
func (c *Cache) refresh(force bool) LeagueData {
	c.refreshMu.Lock()
	defer c.refreshMu.Unlock()
	if !force && !c.stale() {
		c.mu.RLock()
		defer c.mu.RUnlock()
		return c.data
	}

	data, err := c.client.FetchLeague(c.leagueID)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastAttempt = time.Now()
	if err != nil {
		slog.Error("fpl: refresh failed", "error", err)
		return c.data
	}
	c.data = data
	slog.Info("fpl: cache refreshed", "matches", len(data.Matches))
	return data
}
