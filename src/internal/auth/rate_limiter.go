package auth

import (
	"errors"
	"sync"
	"time"
)

// RateLimiter provides rate limiting functionality
type RateLimiter interface {
	Check(key string) (bool, error)
}

var (
	ErrRateLimitExceeded = errors.New("rate limit exceeded")
)

type rateLimiter struct {
	mu              sync.RWMutex
	records         map[string]*rateRecord
	maxSize         int
	window          time.Duration
	maxRequests     int
	cleanupInterval time.Duration
	stopCleanup     chan struct{}
}

type rateRecord struct {
	requests   []time.Time
	lastAccess time.Time
}

// RateLimiterConfig holds configuration for rate limiter
type RateLimiterConfig struct {
	Window          time.Duration // time window (e.g., 1 minute)
	MaxRequests     int           // max requests in window
	MaxSize         int           // max entries in map (default: 10000)
	CleanupInterval time.Duration // cleanup interval (default: 5 minutes)
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(config RateLimiterConfig) RateLimiter {
	if config.MaxSize == 0 {
		config.MaxSize = 10000
	}
	if config.CleanupInterval == 0 {
		config.CleanupInterval = 5 * time.Minute
	}

	rl := &rateLimiter{
		records:         make(map[string]*rateRecord),
		maxSize:         config.MaxSize,
		window:          config.Window,
		maxRequests:     config.MaxRequests,
		cleanupInterval: config.CleanupInterval,
		stopCleanup:     make(chan struct{}),
	}

	// Start background cleanup
	go rl.cleanupLoop()

	return rl
}

func (rl *rateLimiter) Check(key string) (bool, error) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	// Get or create record
	record, exists := rl.records[key]
	if !exists {
		// Check map size before adding
		if len(rl.records) >= rl.maxSize {
			rl.evictLRU()
		}

		record = &rateRecord{
			requests:   make([]time.Time, 0, rl.maxRequests),
			lastAccess: now,
		}
		rl.records[key] = record
	}

	// Clean old requests (outside window)
	record.cleanup(now, rl.window)

	// Check limit
	if len(record.requests) >= rl.maxRequests {
		record.lastAccess = now
		return false, ErrRateLimitExceeded
	}

	// Add new request
	record.requests = append(record.requests, now)
	record.lastAccess = now

	return true, nil
}

func (rl *rateLimiter) evictLRU() {
	if len(rl.records) == 0 {
		return
	}

	var oldestKey string
	var oldestTime time.Time = time.Now()

	for key, record := range rl.records {
		if record.lastAccess.Before(oldestTime) {
			oldestTime = record.lastAccess
			oldestKey = key
		}
	}

	if oldestKey != "" {
		delete(rl.records, oldestKey)
	}
}

func (rl *rateLimiter) cleanupLoop() {
	ticker := time.NewTicker(rl.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.cleanup()
		case <-rl.stopCleanup:
			return
		}
	}
}

func (rl *rateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	expiredKeys := make([]string, 0)

	for key, record := range rl.records {
		// Clean old requests
		record.cleanup(now, rl.window)

		// If record is empty and old (not used for more than 2 windows), delete it
		if len(record.requests) == 0 && now.Sub(record.lastAccess) > 2*rl.window {
			expiredKeys = append(expiredKeys, key)
		}
	}

	// Delete expired records
	for _, key := range expiredKeys {
		delete(rl.records, key)
	}
}

func (r *rateRecord) cleanup(now time.Time, window time.Duration) {
	cutoff := now.Add(-window)

	// Remove all requests older than window
	valid := r.requests[:0]
	for _, t := range r.requests {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	r.requests = valid
}
