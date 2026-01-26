package strava

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Strava rate limits:
// - 100 requests per 15 minutes
// - 1000 requests per day

// RateLimiter manages Strava API rate limits
type RateLimiter struct {
	mu sync.Mutex

	// 15-minute window
	shortLimit    int
	shortUsage    int
	shortResetsAt time.Time

	// Daily window
	dailyLimit    int
	dailyUsage    int
	dailyResetsAt time.Time

	// Minimum interval between requests
	minInterval time.Duration
	lastRequest time.Time
}

// NewRateLimiter creates a new rate limiter with Strava's limits
func NewRateLimiter() *RateLimiter {
	now := time.Now()
	return &RateLimiter{
		shortLimit:    100,
		shortResetsAt: now.Add(15 * time.Minute),
		dailyLimit:    1000,
		dailyResetsAt: now.Truncate(24 * time.Hour).Add(24 * time.Hour),
		minInterval:   150 * time.Millisecond, // ~6.6 req/s max
	}
}

// Wait blocks until a request can be made without exceeding rate limits
func (r *RateLimiter) Wait(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()

	// Reset windows if expired
	if now.After(r.shortResetsAt) {
		r.shortUsage = 0
		r.shortResetsAt = now.Add(15 * time.Minute)
	}
	if now.After(r.dailyResetsAt) {
		r.dailyUsage = 0
		r.dailyResetsAt = now.Truncate(24 * time.Hour).Add(24 * time.Hour)
	}

	// Check 15-minute limit
	if r.shortUsage >= r.shortLimit {
		waitTime := time.Until(r.shortResetsAt)
		r.mu.Unlock()
		select {
		case <-time.After(waitTime):
		case <-ctx.Done():
			return ctx.Err()
		}
		r.mu.Lock()
		r.shortUsage = 0
		r.shortResetsAt = time.Now().Add(15 * time.Minute)
	}

	// Check daily limit
	if r.dailyUsage >= r.dailyLimit {
		waitTime := time.Until(r.dailyResetsAt)
		r.mu.Unlock()
		select {
		case <-time.After(waitTime):
		case <-ctx.Done():
			return ctx.Err()
		}
		r.mu.Lock()
		r.dailyUsage = 0
		r.dailyResetsAt = time.Now().Truncate(24 * time.Hour).Add(24 * time.Hour)
	}

	// Enforce minimum interval between requests
	elapsed := time.Since(r.lastRequest)
	if elapsed < r.minInterval {
		waitTime := r.minInterval - elapsed
		r.mu.Unlock()
		select {
		case <-time.After(waitTime):
		case <-ctx.Done():
			return ctx.Err()
		}
		r.mu.Lock()
	}

	r.shortUsage++
	r.dailyUsage++
	r.lastRequest = time.Now()

	return nil
}

// UpdateFromHeaders updates rate limit state from Strava response headers
func (r *RateLimiter) UpdateFromHeaders(h http.Header) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Strava returns: X-RateLimit-Limit: "100,1000" and X-RateLimit-Usage: "34,512"
	if usage := h.Get("X-RateLimit-Usage"); usage != "" {
		parts := strings.Split(usage, ",")
		if len(parts) >= 2 {
			if short, err := strconv.Atoi(parts[0]); err == nil {
				r.shortUsage = short
			}
			if daily, err := strconv.Atoi(parts[1]); err == nil {
				r.dailyUsage = daily
			}
		}
	}

	if limit := h.Get("X-RateLimit-Limit"); limit != "" {
		parts := strings.Split(limit, ",")
		if len(parts) >= 2 {
			if short, err := strconv.Atoi(parts[0]); err == nil {
				r.shortLimit = short
			}
			if daily, err := strconv.Atoi(parts[1]); err == nil {
				r.dailyLimit = daily
			}
		}
	}
}

// Status returns current rate limit status
func (r *RateLimiter) Status() (shortRemaining, dailyRemaining int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.shortLimit - r.shortUsage, r.dailyLimit - r.dailyUsage
}

// Usage returns current usage counts
func (r *RateLimiter) Usage() (shortUsage, dailyUsage int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.shortUsage, r.dailyUsage
}
