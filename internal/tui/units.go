package tui

import (
	"fmt"

	"runner/internal/config"
)

const (
	metersPerMile = 1609.34
	metersPerKm   = 1000.0
)

// Units provides unit conversion and formatting based on user preferences
type Units struct {
	cfg config.DisplayConfig
}

// NewUnits creates a new Units helper with the given display config
func NewUnits(cfg config.DisplayConfig) Units {
	return Units{cfg: cfg}
}

// FormatDistance formats a distance in meters to the user's preferred unit
func (u Units) FormatDistance(meters float64) string {
	if u.cfg.DistanceUnit == "mi" {
		return fmt.Sprintf("%.1f mi", meters/metersPerMile)
	}
	return fmt.Sprintf("%.1f km", meters/metersPerKm)
}

// FormatDistanceValue returns just the numeric distance value (no unit label)
func (u Units) FormatDistanceValue(meters float64) string {
	if u.cfg.DistanceUnit == "mi" {
		return fmt.Sprintf("%.1f", meters/metersPerMile)
	}
	return fmt.Sprintf("%.1f", meters/metersPerKm)
}

// FormatPace formats pace from total seconds and meters to the user's preferred unit
func (u Units) FormatPace(seconds int, meters float64) string {
	if meters <= 0 || seconds <= 0 {
		return "-"
	}

	var paceSeconds float64
	if u.cfg.PaceUnit == "min/mi" {
		paceSeconds = float64(seconds) / (meters / metersPerMile)
	} else {
		paceSeconds = float64(seconds) / (meters / metersPerKm)
	}

	mins := int(paceSeconds) / 60
	secs := int(paceSeconds) % 60
	return fmt.Sprintf("%d:%02d", mins, secs)
}

// FormatPaceWithUnit formats pace with the unit label
func (u Units) FormatPaceWithUnit(seconds int, meters float64) string {
	pace := u.FormatPace(seconds, meters)
	if pace == "-" {
		return pace
	}
	return pace + "/" + u.DistanceLabel()
}

// DistanceLabel returns the short unit label ("mi" or "km")
func (u Units) DistanceLabel() string {
	if u.cfg.DistanceUnit == "mi" {
		return "mi"
	}
	return "km"
}

// DistanceLabelLong returns the long unit label ("miles" or "km")
func (u Units) DistanceLabelLong() string {
	if u.cfg.DistanceUnit == "mi" {
		return "miles"
	}
	return "km"
}

// PaceLabel returns the pace unit label ("min/mi" or "min/km")
func (u Units) PaceLabel() string {
	if u.cfg.PaceUnit == "min/mi" {
		return "min/mi"
	}
	return "min/km"
}

// ConvertPaceData converts pace data from min/mi to min/km if needed for charts
func (u Units) ConvertPaceData(paceMinPerMile []float64) []float64 {
	if u.cfg.PaceUnit == "min/mi" {
		return paceMinPerMile
	}
	// Convert from min/mi to min/km
	converted := make([]float64, len(paceMinPerMile))
	for i, p := range paceMinPerMile {
		if p > 0 {
			converted[i] = p * metersPerKm / metersPerMile
		}
	}
	return converted
}

// IsMiles returns true if distance unit is miles
func (u Units) IsMiles() bool {
	return u.cfg.DistanceUnit == "mi"
}
