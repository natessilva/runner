package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Config represents the application configuration
type Config struct {
	Strava  StravaConfig  `json:"strava"`
	Athlete AthleteConfig `json:"athlete"`
	Display DisplayConfig `json:"display"`
}

// StravaConfig holds Strava API credentials
type StravaConfig struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

// AthleteConfig holds athlete-specific settings
type AthleteConfig struct {
	RestingHR   float64 `json:"resting_hr"`
	MaxHR       float64 `json:"max_hr"`
	ThresholdHR float64 `json:"threshold_hr"`
}

// DisplayConfig holds display preferences
type DisplayConfig struct {
	DistanceUnit string `json:"distance_unit"`
	PaceUnit     string `json:"pace_unit"`
}

// ErrNoConfig is returned when the config file doesn't exist
var ErrNoConfig = errors.New("config file not found")

// DefaultConfig returns the default configuration
func DefaultConfig() Config {
	return Config{
		Athlete: AthleteConfig{
			RestingHR:   50,
			MaxHR:       185,
			ThresholdHR: 165,
		},
		Display: DisplayConfig{
			DistanceUnit: "km",
			PaceUnit:     "min/km",
		},
	}
}

// Load reads the configuration from ~/.runner/config.json
func Load() (*Config, error) {
	path, err := getConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, ErrNoConfig
	}
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	// Apply defaults for missing values
	defaults := DefaultConfig()
	if cfg.Athlete.RestingHR == 0 {
		cfg.Athlete.RestingHR = defaults.Athlete.RestingHR
	}
	if cfg.Athlete.MaxHR == 0 {
		cfg.Athlete.MaxHR = defaults.Athlete.MaxHR
	}
	if cfg.Athlete.ThresholdHR == 0 {
		cfg.Athlete.ThresholdHR = defaults.Athlete.ThresholdHR
	}
	if cfg.Display.DistanceUnit == "" {
		cfg.Display.DistanceUnit = defaults.Display.DistanceUnit
	}
	if cfg.Display.PaceUnit == "" {
		cfg.Display.PaceUnit = defaults.Display.PaceUnit
	}

	return &cfg, nil
}

// Save writes the configuration to ~/.runner/config.json
func Save(cfg *Config) error {
	path, err := getConfigPath()
	if err != nil {
		return err
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}

	return nil
}

// CreateExample creates an example config file if none exists
func CreateExample() error {
	path, err := getConfigPath()
	if err != nil {
		return err
	}

	// Check if config already exists
	if _, err := os.Stat(path); err == nil {
		return nil // Config exists, don't overwrite
	}

	example := Config{
		Strava: StravaConfig{
			ClientID:     "YOUR_CLIENT_ID",
			ClientSecret: "YOUR_CLIENT_SECRET",
		},
		Athlete: AthleteConfig{
			RestingHR:   50,
			MaxHR:       185,
			ThresholdHR: 165,
		},
		Display: DisplayConfig{
			DistanceUnit: "km",
			PaceUnit:     "min/km",
		},
	}

	return Save(&example)
}

// Validate checks if the config has required fields
func (c *Config) Validate() error {
	if c.Strava.ClientID == "" || c.Strava.ClientID == "YOUR_CLIENT_ID" {
		return errors.New("strava.client_id is required - get it from https://www.strava.com/settings/api")
	}
	if c.Strava.ClientSecret == "" || c.Strava.ClientSecret == "YOUR_CLIENT_SECRET" {
		return errors.New("strava.client_secret is required - get it from https://www.strava.com/settings/api")
	}

	// Validate display units
	if c.Display.DistanceUnit != "" && c.Display.DistanceUnit != "km" && c.Display.DistanceUnit != "mi" {
		return fmt.Errorf("display.distance_unit must be \"km\" or \"mi\", got %q", c.Display.DistanceUnit)
	}
	if c.Display.PaceUnit != "" && c.Display.PaceUnit != "min/km" && c.Display.PaceUnit != "min/mi" {
		return fmt.Errorf("display.pace_unit must be \"min/km\" or \"min/mi\", got %q", c.Display.PaceUnit)
	}

	// Validate threshold_hr < max_hr when both are set
	if c.Athlete.ThresholdHR > 0 && c.Athlete.MaxHR > 0 && c.Athlete.ThresholdHR >= c.Athlete.MaxHR {
		return fmt.Errorf("athlete.threshold_hr (%v) must be less than athlete.max_hr (%v)", c.Athlete.ThresholdHR, c.Athlete.MaxHR)
	}

	return nil
}

// getConfigPath returns the path to the config file
func getConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}
	return filepath.Join(home, ".runner", "config.json"), nil
}

// GetConfigDir returns the path to the config directory
func GetConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}
	return filepath.Join(home, ".runner"), nil
}
