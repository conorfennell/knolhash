package worldcup

import (
	"log/slog"
	"sync"
	"time"
)

const refreshInterval = 1 * time.Minute

// Cache holds the most recently fetched TournamentData and refreshes it
// in the background every 15 minutes.
type Cache struct {
	mu        sync.RWMutex
	data      TournamentData
	lastFetch time.Time
}

// NewCache creates a Cache and performs an initial fetch synchronously.
// If the initial fetch fails, the cache starts empty and the background
// goroutine will retry.
func NewCache() *Cache {
	c := &Cache{}
	data, err := FetchTournamentData()
	if err != nil {
		slog.Error("worldcup: initial fetch failed", "error", err)
	} else {
		c.data = data
		c.lastFetch = data.FetchedAt
	}
	return c
}

// Get returns the cached TournamentData. If the data is stale (older than
// refreshInterval) it triggers a synchronous refresh before returning.
func (c *Cache) Get() TournamentData {
	c.mu.RLock()
	if time.Since(c.lastFetch) < refreshInterval {
		data := c.data
		c.mu.RUnlock()
		return data
	}
	c.mu.RUnlock()
	return c.refresh()
}

// StartBackgroundRefresh starts a goroutine that refreshes the cache every
// refreshInterval. The goroutine runs until the process exits.
func (c *Cache) StartBackgroundRefresh() {
	go func() {
		ticker := time.NewTicker(refreshInterval)
		defer ticker.Stop()
		for range ticker.C {
			c.refresh()
		}
	}()
	slog.Info("worldcup: background refresh started", "interval", refreshInterval)
}

func (c *Cache) refresh() TournamentData {
	data, err := FetchTournamentData()
	if err != nil {
		slog.Error("worldcup: refresh failed", "error", err)
		c.mu.RLock()
		stale := c.data
		c.mu.RUnlock()
		return stale
	}
	c.mu.Lock()
	c.data = data
	c.lastFetch = data.FetchedAt
	c.mu.Unlock()
	slog.Info("worldcup: cache refreshed", "teams", len(data.Teams))
	return data
}
