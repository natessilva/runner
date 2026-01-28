package config

import (
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	// Test athlete defaults
	if cfg.Athlete.RestingHR != 50 {
		t.Errorf("Athlete.RestingHR = %v, want 50", cfg.Athlete.RestingHR)
	}
	if cfg.Athlete.MaxHR != 185 {
		t.Errorf("Athlete.MaxHR = %v, want 185", cfg.Athlete.MaxHR)
	}
	if cfg.Athlete.ThresholdHR != 165 {
		t.Errorf("Athlete.ThresholdHR = %v, want 165", cfg.Athlete.ThresholdHR)
	}

	// Test display defaults
	if cfg.Display.DistanceUnit != "km" {
		t.Errorf("Display.DistanceUnit = %q, want %q", cfg.Display.DistanceUnit, "km")
	}
	if cfg.Display.PaceUnit != "min/km" {
		t.Errorf("Display.PaceUnit = %q, want %q", cfg.Display.PaceUnit, "min/km")
	}

	// Strava config should be empty by default
	if cfg.Strava.ClientID != "" {
		t.Errorf("Strava.ClientID should be empty, got %q", cfg.Strava.ClientID)
	}
	if cfg.Strava.ClientSecret != "" {
		t.Errorf("Strava.ClientSecret should be empty, got %q", cfg.Strava.ClientSecret)
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		expectError bool
		errContains string
	}{
		{
			name: "valid config",
			config: Config{
				Strava: StravaConfig{
					ClientID:     "12345",
					ClientSecret: "abc123secret",
				},
			},
			expectError: false,
		},
		{
			name: "empty client ID",
			config: Config{
				Strava: StravaConfig{
					ClientID:     "",
					ClientSecret: "abc123secret",
				},
			},
			expectError: true,
			errContains: "client_id",
		},
		{
			name: "placeholder client ID",
			config: Config{
				Strava: StravaConfig{
					ClientID:     "YOUR_CLIENT_ID",
					ClientSecret: "abc123secret",
				},
			},
			expectError: true,
			errContains: "client_id",
		},
		{
			name: "empty client secret",
			config: Config{
				Strava: StravaConfig{
					ClientID:     "12345",
					ClientSecret: "",
				},
			},
			expectError: true,
			errContains: "client_secret",
		},
		{
			name: "placeholder client secret",
			config: Config{
				Strava: StravaConfig{
					ClientID:     "12345",
					ClientSecret: "YOUR_CLIENT_SECRET",
				},
			},
			expectError: true,
			errContains: "client_secret",
		},
		{
			name: "both placeholders",
			config: Config{
				Strava: StravaConfig{
					ClientID:     "YOUR_CLIENT_ID",
					ClientSecret: "YOUR_CLIENT_SECRET",
				},
			},
			expectError: true,
			errContains: "client_id", // first error wins
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errContains != "" && !containsString(err.Error(), tt.errContains) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStringHelper(s, substr))
}

func containsStringHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestConfigTypes(t *testing.T) {
	// Test that config structs can be properly instantiated
	cfg := Config{
		Strava: StravaConfig{
			ClientID:     "test-id",
			ClientSecret: "test-secret",
		},
		Athlete: AthleteConfig{
			RestingHR:   55,
			MaxHR:       190,
			ThresholdHR: 170,
		},
		Display: DisplayConfig{
			DistanceUnit: "mi",
			PaceUnit:     "min/mi",
		},
	}

	if cfg.Strava.ClientID != "test-id" {
		t.Error("StravaConfig.ClientID not set correctly")
	}
	if cfg.Athlete.RestingHR != 55 {
		t.Error("AthleteConfig.RestingHR not set correctly")
	}
	if cfg.Display.DistanceUnit != "mi" {
		t.Error("DisplayConfig.DistanceUnit not set correctly")
	}
}
