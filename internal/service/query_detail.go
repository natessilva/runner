package service

import (
	"fmt"

	"runner/internal/store"
)

// MileSplit represents stats for a single mile
type MileSplit struct {
	Mile     int
	Duration int     // seconds
	Pace     string  // "M:SS" format
	AvgHR    float64
	AvgCad   float64
}

// HRZoneTime represents time spent in an HR zone
type HRZoneTime struct {
	Zone    int
	Name    string
	Seconds int
	Percent float64
}

// ActivityDetail contains detailed info for a single activity
type ActivityDetail struct {
	Activity      ActivityWithMetrics
	Splits        []MileSplit
	HRZones       []HRZoneTime
	PaceData      []float64 // pace per minute for charting (min/mile)
	HRData        []float64 // HR per minute for charting
	TimeLabels    []string  // time labels for chart
	AvgHR         float64
	AvgCadence    float64
	MaxHR         int // Observed max HR during this activity
	ConfiguredMax int // Configured max HR used for zone calculations
	ThresholdHR   int // Configured threshold HR (0 if using %maxHR zones)
}

// GetActivityDetailByID returns detailed analysis for a single activity
func (q *QueryService) GetActivityDetailByID(id int64) (*ActivityDetail, error) {
	activity, err := q.store.GetActivity(id)
	if err != nil {
		return nil, err
	}

	metrics, _ := q.store.GetActivityMetrics(id)
	streams, err := q.store.GetStreams(id)
	if err != nil {
		return nil, err
	}

	detail := &ActivityDetail{
		Activity: ActivityWithMetrics{
			Activity: *activity,
		},
		ConfiguredMax: int(q.athleteCfg.MaxHR),
		ThresholdHR:   int(q.athleteCfg.ThresholdHR),
	}
	if metrics != nil {
		detail.Activity.Metrics = *metrics
	}

	if len(streams) == 0 {
		return detail, nil
	}

	// Calculate splits, HR zones, and chart data from streams
	detail.calculateFromStreams(streams, activity.Distance, int(q.athleteCfg.MaxHR), int(q.athleteCfg.ThresholdHR))

	return detail, nil
}

func (d *ActivityDetail) calculateFromStreams(streams []store.StreamPoint, totalDistance float64, configuredMaxHR int, thresholdHR int) {
	// Mile splits
	currentMile := 1
	mileStartIdx := 0
	var lastDistance float64

	for i, p := range streams {
		if p.Distance == nil {
			continue
		}

		dist := *p.Distance
		mileThreshold := float64(currentMile) * MetersPerMile

		if dist >= mileThreshold && lastDistance < mileThreshold {
			// Completed a mile
			split := d.calculateSplit(streams, mileStartIdx, i, currentMile)
			d.Splits = append(d.Splits, split)
			currentMile++
			mileStartIdx = i
		}
		lastDistance = dist
	}

	// Add final partial mile if significant (> 0.1 mile)
	remainingDist := totalDistance - float64(currentMile-1)*MetersPerMile
	if remainingDist > PartialMileThreshold && mileStartIdx < len(streams)-1 {
		split := d.calculateSplit(streams, mileStartIdx, len(streams)-1, currentMile)
		// Adjust pace for partial mile
		if remainingDist > 0 {
			partialMiles := remainingDist / MetersPerMile
			split.Duration = int(float64(split.Duration) / partialMiles)
			split.Pace = formatPace(split.Duration)
		}
		d.Splits = append(d.Splits, split)
	}

	// HR zones (using 5-zone model based on configured max HR)
	// Also record observed max HR during this activity
	d.MaxHR = findMaxHeartrate(streams)

	// Use configured max HR for zone calculations (not the activity's max)
	if configuredMaxHR > 0 {
		d.HRZones = d.calculateHRZones(streams, configuredMaxHR, thresholdHR)
	}

	// Calculate averages using helper
	stats := AggregateStreamStats(streams)
	d.AvgHR = stats.AvgHR()
	d.AvgCadence = stats.AvgCadence()

	// Build chart data (minute-by-minute aggregation)
	d.buildChartData(streams)
}

// findMaxHeartrate returns the highest heart rate in the stream
func findMaxHeartrate(streams []store.StreamPoint) int {
	maxHR := 0
	for _, p := range streams {
		if p.Heartrate != nil && *p.Heartrate > maxHR {
			maxHR = *p.Heartrate
		}
	}
	return maxHR
}

// buildChartData aggregates stream data into minute-by-minute chart arrays
func (d *ActivityDetail) buildChartData(streams []store.StreamPoint) {
	minuteData := make(map[int]struct {
		paceSum   float64
		paceCount int
		hrSum     float64
		hrCount   int
	})

	var prevDist float64
	var prevTime int
	for _, p := range streams {
		minute := p.TimeOffset / SecondsPerMinute

		// Pace calculation
		if p.Distance != nil && p.TimeOffset > prevTime {
			distDelta := *p.Distance - prevDist
			timeDelta := float64(p.TimeOffset - prevTime)
			if distDelta > 0 && timeDelta > 0 {
				speedMPS := distDelta / timeDelta
				if speedMPS > MinSpeedForPace {
					paceMinPerMile := (MetersPerMile / speedMPS) / SecondsPerMinute
					entry := minuteData[minute]
					entry.paceSum += paceMinPerMile
					entry.paceCount++
					minuteData[minute] = entry
				}
			}
			prevDist = *p.Distance
			prevTime = p.TimeOffset
		}

		// HR for chart (slightly different threshold than validation - just > 50)
		if p.Heartrate != nil && *p.Heartrate > MinValidHeartrate {
			entry := minuteData[minute]
			entry.hrSum += float64(*p.Heartrate)
			entry.hrCount++
			minuteData[minute] = entry
		}
	}

	// Find max minute
	maxMinute := 0
	for m := range minuteData {
		if m > maxMinute {
			maxMinute = m
		}
	}

	// Build chart arrays
	for m := 0; m <= maxMinute; m++ {
		entry := minuteData[m]
		if entry.paceCount > 0 {
			d.PaceData = append(d.PaceData, entry.paceSum/float64(entry.paceCount))
		} else if len(d.PaceData) > 0 {
			d.PaceData = append(d.PaceData, d.PaceData[len(d.PaceData)-1]) // carry forward
		} else {
			d.PaceData = append(d.PaceData, 0)
		}

		if entry.hrCount > 0 {
			d.HRData = append(d.HRData, entry.hrSum/float64(entry.hrCount))
		} else if len(d.HRData) > 0 {
			d.HRData = append(d.HRData, d.HRData[len(d.HRData)-1])
		} else {
			d.HRData = append(d.HRData, 0)
		}

		d.TimeLabels = append(d.TimeLabels, formatMinutes(m))
	}
}

func (d *ActivityDetail) calculateSplit(streams []store.StreamPoint, startIdx, endIdx int, mile int) MileSplit {
	split := MileSplit{Mile: mile}

	if endIdx <= startIdx || endIdx >= len(streams) {
		return split
	}

	startTime := streams[startIdx].TimeOffset
	endTime := streams[endIdx].TimeOffset
	split.Duration = endTime - startTime
	split.Pace = formatPace(split.Duration)

	// Calculate averages for this split using the slice
	splitStreams := streams[startIdx : endIdx+1]
	stats := AggregateStreamStats(splitStreams)
	split.AvgHR = stats.AvgHR()
	split.AvgCad = stats.AvgCadence()

	return split
}

func (d *ActivityDetail) calculateHRZones(streams []store.StreamPoint, maxHR int, thresholdHR int) []HRZoneTime {
	// Use threshold-based zones if thresholdHR is set, otherwise use %maxHR zones
	var zones []HRZoneTime
	var thresholds []float64

	if thresholdHR > 0 {
		// Threshold-based zones (based on % of threshold HR)
		// Zone 1: <75% LTHR, Zone 2: 75-84% LTHR, Zone 3: 85-94% LTHR, Zone 4: 95-100% LTHR, Zone 5: >100% LTHR
		zones = []HRZoneTime{
			{Zone: 1, Name: "Warm Up (<75% LTHR)"},
			{Zone: 2, Name: "Easy (75-84% LTHR)"},
			{Zone: 3, Name: "Aerobic (85-94% LTHR)"},
			{Zone: 4, Name: "Threshold (95-100% LTHR)"},
			{Zone: 5, Name: "Maximum (>100% LTHR)"},
		}
		// Convert zone thresholds to actual HR values then to % of max for comparison
		// Zone boundaries match labels: Z2 75-84%, Z3 85-94%, Z4 95-100%
		// Using exclusive upper bounds so Z3 includes up to 94.99% and Z4 starts at 95%
		lthr := float64(thresholdHR)
		maxF := float64(maxHR)
		thresholds = []float64{
			(0.75 * lthr) / maxF, // Zone 1 upper bound: <75% LTHR
			(0.85 * lthr) / maxF, // Zone 2 upper bound: <85% LTHR
			(0.95 * lthr) / maxF, // Zone 3 upper bound: <95% LTHR
			lthr / maxF,          // Zone 4 upper bound: <=100% LTHR
			1.0,                  // Zone 5 upper bound: >100% LTHR
		}
	} else {
		// Traditional %maxHR zones
		zones = []HRZoneTime{
			{Zone: 1, Name: "Warm Up (<60%)"},
			{Zone: 2, Name: "Easy (60-70%)"},
			{Zone: 3, Name: "Aerobic (70-80%)"},
			{Zone: 4, Name: "Threshold (80-90%)"},
			{Zone: 5, Name: "Maximum (>90%)"},
		}
		thresholds = HRZoneThresholds
	}

	totalSeconds := 0

	for _, p := range streams {
		if p.Heartrate == nil || *p.Heartrate < MinValidHeartrate {
			continue
		}

		pct := float64(*p.Heartrate) / float64(maxHR)
		totalSeconds++

		for i, thresh := range thresholds {
			if pct <= thresh {
				zones[i].Seconds++
				break
			}
		}
	}

	// Calculate percentages
	if totalSeconds > 0 {
		for i := range zones {
			zones[i].Percent = float64(zones[i].Seconds) / float64(totalSeconds) * 100
		}
	}

	return zones
}

func formatPace(seconds int) string {
	mins := seconds / SecondsPerMinute
	secs := seconds % SecondsPerMinute
	return fmt.Sprintf("%d:%02d", mins, secs)
}

func formatMinutes(m int) string {
	return fmt.Sprintf("%d:00", m)
}
