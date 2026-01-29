package main

import (
	"context"
	"errors"
	"fmt"
	"log"

	"golang.org/x/oauth2"

	tea "github.com/charmbracelet/bubbletea"

	"runner/internal/auth"
	"runner/internal/config"
	"runner/internal/service"
	"runner/internal/store"
	"runner/internal/strava"
	"runner/internal/tui"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx := context.Background()

	// Load configuration
	cfg, err := config.Load()
	if errors.Is(err, config.ErrNoConfig) {
		fmt.Println("No config file found. Creating example config...")
		if err := config.CreateExample(); err != nil {
			return fmt.Errorf("creating example config: %w", err)
		}
		configDir, _ := config.GetConfigDir()
		fmt.Printf("\nPlease edit the config file at:\n  %s/config.json\n\n", configDir)
		fmt.Println("You need to add your Strava API credentials.")
		fmt.Println("Get them from: https://www.strava.com/settings/api")
		return nil
	}
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Validate config
	if err := cfg.Validate(); err != nil {
		configDir, _ := config.GetConfigDir()
		fmt.Printf("Config validation failed: %v\n\n", err)
		fmt.Printf("Please edit the config file at:\n  %s/config.json\n", configDir)
		return nil
	}

	// Open database
	db, err := store.Open()
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()

	// Check for existing auth
	storedAuth, err := db.GetAuth()
	if errors.Is(err, store.ErrNoAuth) {
		// No auth stored, need to authenticate
		fmt.Println("No authentication found. Starting OAuth flow...")
		if err := authenticate(ctx, db, cfg); err != nil {
			return fmt.Errorf("authentication: %w", err)
		}
		// Re-fetch auth after successful authentication
		storedAuth, err = db.GetAuth()
		if err != nil {
			return fmt.Errorf("fetching auth after login: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("checking auth: %w", err)
	}

	// Create token source for API calls (with auto-refresh)
	oauthCfg := auth.NewOAuthConfig(auth.Config{
		ClientID:     cfg.Strava.ClientID,
		ClientSecret: cfg.Strava.ClientSecret,
		RedirectURL:  fmt.Sprintf("http://localhost:%d/callback", auth.CallbackPort),
	})

	token := &oauth2.Token{
		AccessToken:  storedAuth.AccessToken,
		RefreshToken: storedAuth.RefreshToken,
		Expiry:       storedAuth.ExpiresAt,
	}

	tokenSource := auth.NewTokenSource(oauthCfg, token, func(newToken *oauth2.Token) error {
		return db.UpdateTokens(newToken.AccessToken, newToken.RefreshToken, newToken.Expiry)
	})

	// Test token is valid by getting a fresh one
	if _, err := tokenSource.Token(); err != nil {
		fmt.Println("Stored token is invalid or expired. Re-authenticating...")
		if err := authenticate(ctx, db, cfg); err != nil {
			return fmt.Errorf("re-authentication: %w", err)
		}
	}

	// Create services
	stravaClient := strava.NewClient(tokenSource)
	syncSvc := service.NewSyncService(stravaClient, db, cfg.Athlete)
	querySvc := service.NewQueryService(db, cfg.Athlete)

	// Launch TUI
	app := tui.NewApp(db, stravaClient, syncSvc, querySvc, cfg.Display)
	p := tea.NewProgram(app, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("running TUI: %w", err)
	}

	return nil
}

func authenticate(ctx context.Context, db *store.DB, cfg *config.Config) error {
	oauthCfg := auth.NewOAuthConfig(auth.Config{
		ClientID:     cfg.Strava.ClientID,
		ClientSecret: cfg.Strava.ClientSecret,
		RedirectURL:  fmt.Sprintf("http://localhost:%d/callback", auth.CallbackPort),
	})

	result, err := auth.Authenticate(ctx, oauthCfg)
	if err != nil {
		return err
	}

	// Store the tokens
	storedAuth := &store.Auth{
		AthleteID:    result.AthleteID,
		AccessToken:  result.Token.AccessToken,
		RefreshToken: result.Token.RefreshToken,
		ExpiresAt:    result.Token.Expiry,
	}

	if err := db.SaveAuth(storedAuth); err != nil {
		return fmt.Errorf("saving auth: %w", err)
	}

	fmt.Println()
	fmt.Printf("Successfully authenticated as athlete %d!\n", result.AthleteID)
	return nil
}
