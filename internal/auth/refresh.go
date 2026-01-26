package auth

import (
	"context"
	"sync"
	"time"

	"golang.org/x/oauth2"
)

// TokenSource wraps oauth2.TokenSource with persistence
// It automatically refreshes tokens and calls onRefresh when a new token is obtained
type TokenSource struct {
	config    *oauth2.Config
	token     *oauth2.Token
	onRefresh func(*oauth2.Token) error
	mu        sync.Mutex
}

// NewTokenSource creates a new TokenSource that will refresh tokens as needed
// and call onRefresh to persist new tokens
func NewTokenSource(cfg *oauth2.Config, token *oauth2.Token, onRefresh func(*oauth2.Token) error) *TokenSource {
	return &TokenSource{
		config:    cfg,
		token:     token,
		onRefresh: onRefresh,
	}
}

// Token returns a valid token, refreshing if necessary
func (ts *TokenSource) Token() (*oauth2.Token, error) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	// Check if token needs refresh (with 60s buffer)
	if time.Until(ts.token.Expiry) > 60*time.Second {
		return ts.token, nil
	}

	// Refresh the token
	ctx := context.Background()
	src := ts.config.TokenSource(ctx, ts.token)
	newToken, err := src.Token()
	if err != nil {
		return nil, err
	}

	// Persist the new token if callback is set
	if ts.onRefresh != nil {
		if err := ts.onRefresh(newToken); err != nil {
			return nil, err
		}
	}

	ts.token = newToken
	return newToken, nil
}

// IsExpired checks if the current token is expired or will expire within the buffer
func (ts *TokenSource) IsExpired() bool {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return time.Until(ts.token.Expiry) <= 60*time.Second
}

// CurrentToken returns the current token without refreshing
func (ts *TokenSource) CurrentToken() *oauth2.Token {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return ts.token
}
