package service

import (
	"fmt"

	"runner/internal/store"
)

// PersonalRecordDisplay represents a formatted personal record for display
type PersonalRecordDisplay struct {
	Category       string
	CategoryLabel  string  // e.g., "5K", "1 Mile", "Best 400m"
	Time           string  // formatted duration "M:SS" or "H:MM:SS"
	Pace           string  // formatted pace "M:SS/mi"
	AvgHR          string  // formatted HR or "-"
	Date           string  // formatted date
	ActivityID     int64
	ActivityName   string
	IsEffort       bool    // true for best efforts, false for race distances
	DistanceMeters float64 // for display purposes
}

// PRsData contains all data needed for the PRs screen
type PRsData struct {
	RaceDistancePRs []PersonalRecordDisplay
	BestEffortPRs   []PersonalRecordDisplay
	OtherPRs        []PersonalRecordDisplay
}

// GetPersonalRecords retrieves all personal records formatted for display
func (q *QueryService) GetPersonalRecords() (*PRsData, error) {
	records, err := q.store.GetAllPersonalRecords()
	if err != nil {
		return nil, err
	}

	// Collect unique activity IDs for batch fetch
	activityIDSet := make(map[int64]struct{})
	for _, r := range records {
		activityIDSet[r.ActivityID] = struct{}{}
	}
	activityIDs := make([]int64, 0, len(activityIDSet))
	for id := range activityIDSet {
		activityIDs = append(activityIDs, id)
	}

	// Batch fetch all activities (fixes N+1 query)
	activities, err := q.store.GetActivitiesByIDs(activityIDs)
	if err != nil {
		activities = make(map[int64]*store.Activity) // Continue with empty map on error
	}

	// Build activity names map
	activityNames := make(map[int64]string)
	for id, activity := range activities {
		activityNames[id] = activity.Name
	}

	data := &PRsData{}

	for _, r := range records {
		display := PersonalRecordDisplay{
			Category:       r.Category,
			CategoryLabel:  formatCategoryLabel(r.Category),
			Time:           formatDuration(r.DurationSeconds),
			Date:           r.AchievedAt.Format("Jan 02, 2006"),
			ActivityID:     r.ActivityID,
			ActivityName:   activityNames[r.ActivityID],
			DistanceMeters: r.DistanceMeters,
		}

		if r.PacePerMile != nil {
			display.Pace = formatPace(int(*r.PacePerMile))
		} else {
			display.Pace = "-"
		}

		if r.AvgHeartrate != nil {
			display.AvgHR = fmt.Sprintf("%.0f", *r.AvgHeartrate)
		} else {
			display.AvgHR = "-"
		}

		// Categorize the record
		switch {
		case isRaceDistanceCategory(r.Category):
			data.RaceDistancePRs = append(data.RaceDistancePRs, display)
		case isEffortCategory(r.Category):
			display.IsEffort = true
			data.BestEffortPRs = append(data.BestEffortPRs, display)
		default:
			data.OtherPRs = append(data.OtherPRs, display)
		}
	}

	// Sort each category by distance
	sortPRsByDistance(data.RaceDistancePRs)
	sortPRsByDistance(data.BestEffortPRs)

	return data, nil
}

// GetActivityPRs retrieves personal records achieved during a specific activity
func (q *QueryService) GetActivityPRs(activityID int64) ([]PersonalRecordDisplay, error) {
	records, err := q.store.GetPersonalRecordsForActivity(activityID)
	if err != nil {
		return nil, err
	}

	var displays []PersonalRecordDisplay
	for _, r := range records {
		display := PersonalRecordDisplay{
			Category:       r.Category,
			CategoryLabel:  formatCategoryLabel(r.Category),
			Time:           formatDuration(r.DurationSeconds),
			Date:           r.AchievedAt.Format("Jan 02, 2006"),
			ActivityID:     r.ActivityID,
			DistanceMeters: r.DistanceMeters,
			IsEffort:       isEffortCategory(r.Category),
		}

		if r.PacePerMile != nil {
			display.Pace = formatPace(int(*r.PacePerMile))
		} else {
			display.Pace = "-"
		}

		if r.AvgHeartrate != nil {
			display.AvgHR = fmt.Sprintf("%.0f", *r.AvgHeartrate)
		} else {
			display.AvgHR = "-"
		}

		displays = append(displays, display)
	}

	return displays, nil
}

// formatCategoryLabel returns a human-readable label for a PR category
func formatCategoryLabel(category string) string {
	labels := map[string]string{
		"distance_1mi":      "1 Mile",
		"distance_5k":       "5K",
		"distance_10k":      "10K",
		"distance_half":     "Half Marathon",
		"distance_full":     "Marathon",
		"effort_400m":       "400m",
		"effort_1k":         "1K",
		"effort_1mi":        "1 Mile",
		"effort_5k":         "5K",
		"effort_10k":        "10K",
		"longest_run":       "Longest Run",
		"highest_elevation": "Most Elevation",
		"fastest_pace":      "Fastest Avg Pace",
	}

	if label, ok := labels[category]; ok {
		return label
	}
	return category
}

// isRaceDistanceCategory returns true if the category is a race distance PR
func isRaceDistanceCategory(category string) bool {
	return len(category) > 9 && category[:9] == "distance_"
}

// isEffortCategory returns true if the category is a best effort PR
func isEffortCategory(category string) bool {
	return len(category) > 7 && category[:7] == "effort_"
}

// sortPRsByDistance sorts PRs by their target distance (shortest first)
func sortPRsByDistance(prs []PersonalRecordDisplay) {
	// Define sort order
	order := map[string]int{
		"effort_400m":   1,
		"effort_1k":     2,
		"effort_1mi":    3,
		"distance_1mi":  3,
		"effort_5k":     4,
		"distance_5k":   4,
		"effort_10k":    5,
		"distance_10k":  5,
		"distance_half": 6,
		"distance_full": 7,
	}

	// Simple bubble sort for small slices
	for i := 0; i < len(prs); i++ {
		for j := i + 1; j < len(prs); j++ {
			orderI := order[prs[i].Category]
			orderJ := order[prs[j].Category]
			if orderI > orderJ {
				prs[i], prs[j] = prs[j], prs[i]
			}
		}
	}
}
