package auth

import (
	"golang.org/x/oauth2"
)

const (
	// Strava OAuth endpoints
	AuthURL  = "https://www.strava.com/oauth/authorize"
	TokenURL = "https://www.strava.com/oauth/token"
)

// Scopes required for our app (Strava uses comma-separated scopes)
var Scopes = []string{
	"read,activity:read_all",
}

// Config holds the OAuth client credentials
type Config struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string // e.g., "http://localhost:8089/callback"
}

// NewOAuthConfig creates an oauth2.Config from our Config
func NewOAuthConfig(cfg Config) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  AuthURL,
			TokenURL: TokenURL,
		},
		RedirectURL: cfg.RedirectURL,
		Scopes:      Scopes,
	}
}

// AuthResult contains the token and athlete info from successful auth
type AuthResult struct {
	Token     *oauth2.Token
	AthleteID int64
}

// ExtractAthleteID extracts the athlete ID from the token extras
// Strava includes athlete info in the token response
func ExtractAthleteID(token *oauth2.Token) int64 {
	if athlete, ok := token.Extra("athlete").(map[string]interface{}); ok {
		if id, ok := athlete["id"].(float64); ok {
			return int64(id)
		}
	}
	return 0
}
