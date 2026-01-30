package analysis

import (
	"math"
)

// VDOTEntry represents a row in the VDOT lookup table
// Times are in seconds for each distance
type VDOTEntry struct {
	VDOT     float64
	Time1500 float64 // 1500m time in seconds
	Time1Mi  float64 // 1 mile time in seconds
	Time5K   float64 // 5K time in seconds
	Time10K  float64 // 10K time in seconds
	TimeHalf float64 // Half marathon time in seconds
	TimeFull float64 // Marathon time in seconds
}

// VDOTTable contains the Jack Daniels VDOT lookup values
// Covers recreational to elite runners (VDOT 30-85)
var VDOTTable = []VDOTEntry{
	{30, 510, 552, 1860, 3876, 8388, 17496},
	{31, 496, 536, 1806, 3762, 8136, 16980},
	{32, 482, 521, 1752, 3654, 7896, 16488},
	{33, 469, 507, 1704, 3552, 7674, 16020},
	{34, 457, 494, 1656, 3450, 7458, 15570},
	{35, 445, 481, 1614, 3360, 7254, 15138},
	{36, 434, 469, 1572, 3270, 7062, 14730},
	{37, 423, 457, 1530, 3186, 6876, 14334},
	{38, 413, 446, 1494, 3102, 6702, 13956},
	{39, 403, 435, 1458, 3024, 6534, 13596},
	{40, 394, 425, 1422, 2952, 6372, 13248},
	{41, 385, 416, 1392, 2880, 6222, 12918},
	{42, 376, 406, 1356, 2814, 6078, 12600},
	{43, 368, 398, 1326, 2748, 5940, 12300},
	{44, 360, 389, 1296, 2688, 5802, 12006},
	{45, 352, 381, 1266, 2628, 5676, 11730},
	{46, 345, 373, 1242, 2568, 5550, 11460},
	{47, 338, 365, 1212, 2514, 5430, 11202},
	{48, 331, 358, 1188, 2460, 5316, 10956},
	{49, 324, 351, 1164, 2412, 5208, 10722},
	{50, 318, 344, 1140, 2364, 5100, 10494},
	{51, 312, 337, 1116, 2316, 4998, 10278},
	{52, 306, 331, 1098, 2274, 4902, 10068},
	{53, 300, 325, 1074, 2232, 4806, 9870},
	{54, 295, 319, 1056, 2190, 4716, 9678},
	{55, 290, 313, 1038, 2154, 4632, 9492},
	{56, 285, 308, 1020, 2112, 4548, 9312},
	{57, 280, 302, 1002, 2076, 4470, 9144},
	{58, 275, 297, 984, 2040, 4392, 8976},
	{59, 270, 292, 972, 2010, 4320, 8820},
	{60, 266, 288, 954, 1974, 4248, 8664},
	{61, 262, 283, 942, 1944, 4182, 8520},
	{62, 258, 279, 924, 1914, 4116, 8376},
	{63, 254, 274, 912, 1884, 4050, 8238},
	{64, 250, 270, 900, 1860, 3990, 8106},
	{65, 246, 266, 888, 1830, 3930, 7980},
	{66, 242, 262, 876, 1806, 3876, 7860},
	{67, 239, 258, 864, 1782, 3822, 7740},
	{68, 235, 254, 852, 1758, 3768, 7626},
	{69, 232, 251, 840, 1734, 3720, 7518},
	{70, 229, 247, 834, 1716, 3672, 7410},
	{71, 226, 244, 822, 1692, 3624, 7308},
	{72, 223, 241, 810, 1674, 3582, 7212},
	{73, 220, 238, 804, 1656, 3540, 7116},
	{74, 217, 235, 792, 1632, 3498, 7026},
	{75, 214, 232, 786, 1614, 3456, 6936},
	{76, 212, 229, 774, 1596, 3420, 6852},
	{77, 209, 226, 768, 1578, 3384, 6768},
	{78, 206, 223, 756, 1560, 3348, 6690},
	{79, 204, 221, 750, 1548, 3312, 6612},
	{80, 201, 218, 744, 1530, 3282, 6540},
	{81, 199, 215, 738, 1518, 3246, 6468},
	{82, 197, 213, 726, 1500, 3216, 6396},
	{83, 194, 210, 720, 1488, 3186, 6330},
	{84, 192, 208, 714, 1470, 3156, 6264},
	{85, 190, 206, 708, 1458, 3126, 6198},
}

// CalculateVDOT derives VDOT from a race result
// distanceMeters: the race distance in meters
// durationSeconds: the finish time in seconds
// Returns the estimated VDOT value
func CalculateVDOT(distanceMeters float64, durationSeconds int) float64 {
	if durationSeconds <= 0 {
		return 0
	}

	// Find which distance column to use
	getTimeForDistance := func(entry VDOTEntry) float64 {
		switch {
		case matchesDistance(distanceMeters, 1500):
			return entry.Time1500
		case matchesDistance(distanceMeters, Distance1Mile):
			return entry.Time1Mi
		case matchesDistance(distanceMeters, Distance5K):
			return entry.Time5K
		case matchesDistance(distanceMeters, Distance10K):
			return entry.Time10K
		case matchesDistance(distanceMeters, DistanceHalfMara):
			return entry.TimeHalf
		case matchesDistance(distanceMeters, DistanceMarathon):
			return entry.TimeFull
		default:
			// For non-standard distances, interpolate based on closest standard distance
			return interpolateTimeForDistance(entry, distanceMeters)
		}
	}

	duration := float64(durationSeconds)

	// Binary search to find the VDOT range
	low, high := 0, len(VDOTTable)-1

	// Handle edge cases
	if duration >= getTimeForDistance(VDOTTable[0]) {
		return VDOTTable[0].VDOT
	}
	if duration <= getTimeForDistance(VDOTTable[high]) {
		return VDOTTable[high].VDOT
	}

	// Binary search for the bracketing entries
	for high-low > 1 {
		mid := (low + high) / 2
		midTime := getTimeForDistance(VDOTTable[mid])
		if duration <= midTime {
			low = mid
		} else {
			high = mid
		}
	}

	// Interpolate between low and high entries
	lowEntry := VDOTTable[low]
	highEntry := VDOTTable[high]
	lowTime := getTimeForDistance(lowEntry)
	highTime := getTimeForDistance(highEntry)

	if lowTime == highTime {
		return lowEntry.VDOT
	}

	// Linear interpolation
	fraction := (lowTime - duration) / (lowTime - highTime)
	vdot := lowEntry.VDOT + fraction*(highEntry.VDOT-lowEntry.VDOT)

	return math.Round(vdot*10) / 10 // Round to 1 decimal place
}

// PredictTime predicts race time for a target distance given a VDOT
// Returns predicted time in seconds
func PredictTime(vdot float64, targetDistanceMeters float64) int {
	if vdot <= 0 {
		return 0
	}

	// Find the bracketing VDOT entries
	low, high := 0, len(VDOTTable)-1

	if vdot <= VDOTTable[0].VDOT {
		low, high = 0, 0
	} else if vdot >= VDOTTable[len(VDOTTable)-1].VDOT {
		low, high = len(VDOTTable)-1, len(VDOTTable)-1
	} else {
		for high-low > 1 {
			mid := (low + high) / 2
			if VDOTTable[mid].VDOT <= vdot {
				low = mid
			} else {
				high = mid
			}
		}
	}

	getTimeForDistance := func(entry VDOTEntry) float64 {
		switch {
		case matchesDistance(targetDistanceMeters, 1500):
			return entry.Time1500
		case matchesDistance(targetDistanceMeters, Distance1Mile):
			return entry.Time1Mi
		case matchesDistance(targetDistanceMeters, Distance5K):
			return entry.Time5K
		case matchesDistance(targetDistanceMeters, Distance10K):
			return entry.Time10K
		case matchesDistance(targetDistanceMeters, DistanceHalfMara):
			return entry.TimeHalf
		case matchesDistance(targetDistanceMeters, DistanceMarathon):
			return entry.TimeFull
		default:
			return interpolateTimeForDistance(entry, targetDistanceMeters)
		}
	}

	if low == high {
		return int(math.Round(getTimeForDistance(VDOTTable[low])))
	}

	// Interpolate
	lowEntry := VDOTTable[low]
	highEntry := VDOTTable[high]
	fraction := (vdot - lowEntry.VDOT) / (highEntry.VDOT - lowEntry.VDOT)

	lowTime := getTimeForDistance(lowEntry)
	highTime := getTimeForDistance(highEntry)

	predictedTime := lowTime + fraction*(highTime-lowTime)
	return int(math.Round(predictedTime))
}

// GetVDOTLabel returns a human-readable fitness level for a VDOT value
func GetVDOTLabel(vdot float64) string {
	switch {
	case vdot >= 75:
		return "Elite"
	case vdot >= 65:
		return "Highly Competitive"
	case vdot >= 55:
		return "Competitive"
	case vdot >= 45:
		return "Advanced Recreational"
	case vdot >= 38:
		return "Intermediate"
	case vdot >= 30:
		return "Beginner"
	default:
		return "Novice"
	}
}

// matchesDistance checks if a distance is within 5% of a target
func matchesDistance(distance, target float64) bool {
	tolerance := target * 0.05
	return math.Abs(distance-target) <= tolerance
}

// interpolateTimeForDistance estimates time for non-standard distances
// Uses a power-law relationship between distance and time
func interpolateTimeForDistance(entry VDOTEntry, distance float64) float64 {
	// Find two closest standard distances to interpolate between
	type distTime struct {
		dist float64
		time float64
	}

	standards := []distTime{
		{1500, entry.Time1500},
		{Distance1Mile, entry.Time1Mi},
		{Distance5K, entry.Time5K},
		{Distance10K, entry.Time10K},
		{DistanceHalfMara, entry.TimeHalf},
		{DistanceMarathon, entry.TimeFull},
	}

	// Find bracketing distances
	var lower, upper distTime
	for i, s := range standards {
		if distance <= s.dist {
			if i == 0 {
				lower = s
				upper = standards[1]
			} else {
				lower = standards[i-1]
				upper = s
			}
			break
		}
		if i == len(standards)-1 {
			lower = standards[len(standards)-2]
			upper = s
		}
	}

	// Use logarithmic interpolation (approximates the power-law relationship)
	logDistRatio := math.Log(distance/lower.dist) / math.Log(upper.dist/lower.dist)
	logTimeRatio := math.Log(upper.time) - math.Log(lower.time)

	return math.Exp(math.Log(lower.time) + logDistRatio*logTimeRatio)
}
