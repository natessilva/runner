package service

import (
	"testing"

	"runner/internal/config"
)

func TestFormatPace(t *testing.T) {
	tests := []struct {
		seconds  int
		expected string
	}{
		{0, "0:00"},
		{30, "0:30"},
		{60, "1:00"},
		{90, "1:30"},
		{300, "5:00"},
		{359, "5:59"},
		{360, "6:00"},
		{420, "7:00"},
		{450, "7:30"},
		{599, "9:59"},
		{600, "10:00"},
		{3600, "60:00"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatPace(tt.seconds)
			if result != tt.expected {
				t.Errorf("formatPace(%d) = %q, want %q", tt.seconds, result, tt.expected)
			}
		})
	}
}

func TestFormatMinutes(t *testing.T) {
	tests := []struct {
		minutes  int
		expected string
	}{
		{0, "0:00"},
		{1, "1:00"},
		{5, "5:00"},
		{10, "10:00"},
		{30, "30:00"},
		{60, "60:00"},
		{90, "90:00"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatMinutes(tt.minutes)
			if result != tt.expected {
				t.Errorf("formatMinutes(%d) = %q, want %q", tt.minutes, result, tt.expected)
			}
		})
	}
}

func TestNewQueryService(t *testing.T) {
	tests := []struct {
		name       string
		athleteCfg config.AthleteConfig
		expected   float64
	}{
		{
			name:       "uses provided maxHR",
			athleteCfg: config.AthleteConfig{MaxHR: 190, RestingHR: 50, ThresholdHR: 165},
			expected:   190,
		},
		{
			name:       "defaults to 185 when maxHR is 0",
			athleteCfg: config.AthleteConfig{MaxHR: 0, RestingHR: 50, ThresholdHR: 0},
			expected:   185,
		},
		{
			name:       "accepts custom maxHR",
			athleteCfg: config.AthleteConfig{MaxHR: 200, RestingHR: 55, ThresholdHR: 175},
			expected:   200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Pass nil for store since we're only testing athleteCfg handling
			svc := NewQueryService(nil, tt.athleteCfg)
			if svc.athleteCfg.MaxHR != tt.expected {
				t.Errorf("MaxHR = %v, want %v", svc.athleteCfg.MaxHR, tt.expected)
			}
		})
	}
}

func TestHRZoneTimeStructure(t *testing.T) {
	// Test that HRZoneTime struct can be properly used
	zone := HRZoneTime{
		Zone:    1,
		Name:    "Recovery (<60%)",
		Seconds: 600,
		Percent: 25.5,
	}

	if zone.Zone != 1 {
		t.Error("Zone not set correctly")
	}
	if zone.Name != "Recovery (<60%)" {
		t.Error("Name not set correctly")
	}
	if zone.Seconds != 600 {
		t.Error("Seconds not set correctly")
	}
	if zone.Percent != 25.5 {
		t.Error("Percent not set correctly")
	}
}

func TestMileSplitStructure(t *testing.T) {
	// Test that MileSplit struct can be properly used
	split := MileSplit{
		Mile:     1,
		Duration: 420,
		Pace:     "7:00",
		AvgHR:    155.5,
		AvgCad:   180.0,
	}

	if split.Mile != 1 {
		t.Error("Mile not set correctly")
	}
	if split.Duration != 420 {
		t.Error("Duration not set correctly")
	}
	if split.Pace != "7:00" {
		t.Error("Pace not set correctly")
	}
	if split.AvgHR != 155.5 {
		t.Error("AvgHR not set correctly")
	}
	if split.AvgCad != 180.0 {
		t.Error("AvgCad not set correctly")
	}
}

func TestPeriodStatsStructure(t *testing.T) {
	// Test that PeriodStats struct can be properly used
	stats := PeriodStats{
		PeriodLabel: "Jan 06",
		RunCount:    5,
		TotalMiles:  25.5,
		AvgHR:       145.0,
		AvgSPM:      175.0,
		AvgEF:       1.25,
	}

	if stats.PeriodLabel != "Jan 06" {
		t.Error("PeriodLabel not set correctly")
	}
	if stats.RunCount != 5 {
		t.Error("RunCount not set correctly")
	}
	if stats.TotalMiles != 25.5 {
		t.Error("TotalMiles not set correctly")
	}
	if stats.AvgHR != 145.0 {
		t.Error("AvgHR not set correctly")
	}
	if stats.AvgSPM != 175.0 {
		t.Error("AvgSPM not set correctly")
	}
	if stats.AvgEF != 1.25 {
		t.Error("AvgEF not set correctly")
	}
}

func TestComparisonStatsStructure(t *testing.T) {
	// Test that ComparisonStats struct can be properly used
	current := PeriodStats{
		PeriodLabel: "This Week",
		RunCount:    5,
		TotalMiles:  25.5,
		AvgHR:       145.0,
		AvgSPM:      175.0,
		AvgEF:       1.25,
	}
	previous := PeriodStats{
		PeriodLabel: "Last Week",
		RunCount:    4,
		TotalMiles:  20.0,
		AvgHR:       148.0,
		AvgSPM:      172.0,
		AvgEF:       1.20,
	}
	comp := ComparisonStats{
		Label:      "This Week vs Last Week",
		Current:    current,
		Previous:   previous,
		DeltaRuns:  1,
		DeltaMiles: 5.5,
		DeltaHR:    -3.0,
		DeltaSPM:   3.0,
		DeltaEF:    0.05,
	}

	if comp.Label != "This Week vs Last Week" {
		t.Error("Label not set correctly")
	}
	if comp.Current.RunCount != 5 {
		t.Error("Current.RunCount not set correctly")
	}
	if comp.Previous.RunCount != 4 {
		t.Error("Previous.RunCount not set correctly")
	}
	if comp.DeltaRuns != 1 {
		t.Error("DeltaRuns not set correctly")
	}
	if comp.DeltaMiles != 5.5 {
		t.Error("DeltaMiles not set correctly")
	}
	if comp.DeltaHR != -3.0 {
		t.Error("DeltaHR not set correctly")
	}
	if comp.DeltaSPM != 3.0 {
		t.Error("DeltaSPM not set correctly")
	}
	if comp.DeltaEF != 0.05 {
		t.Error("DeltaEF not set correctly")
	}
}

func TestDashboardDataStructure(t *testing.T) {
	// Test that DashboardData struct can be properly used
	data := DashboardData{
		CurrentEF:       1.2,
		EFTrend:         "↑",
		CurrentFitness:  45.0,
		CurrentFatigue:  55.0,
		CurrentForm:     -10.0,
		FormDescription: "Slightly fatigued",
		WeekRunCount:    3,
		WeekDistance:    15.5,
		WeekTime:        5400,
		WeekAvgEF:       1.15,
	}

	if data.CurrentEF != 1.2 {
		t.Error("CurrentEF not set correctly")
	}
	if data.EFTrend != "↑" {
		t.Error("EFTrend not set correctly")
	}
	if data.CurrentFitness != 45.0 {
		t.Error("CurrentFitness not set correctly")
	}
	if data.CurrentForm != -10.0 {
		t.Error("CurrentForm not set correctly")
	}
	if data.FormDescription != "Slightly fatigued" {
		t.Error("FormDescription not set correctly")
	}
	if data.WeekRunCount != 3 {
		t.Error("WeekRunCount not set correctly")
	}
}

func TestActivityDetailStructure(t *testing.T) {
	// Test that ActivityDetail struct can be properly used
	detail := ActivityDetail{
		AvgHR:         150.0,
		AvgCadence:    175.0,
		MaxHR:         180,
		ConfiguredMax: 185,
	}

	if detail.AvgHR != 150.0 {
		t.Error("AvgHR not set correctly")
	}
	if detail.AvgCadence != 175.0 {
		t.Error("AvgCadence not set correctly")
	}
	if detail.MaxHR != 180 {
		t.Error("MaxHR not set correctly")
	}
	if detail.ConfiguredMax != 185 {
		t.Error("ConfiguredMax not set correctly")
	}
}
