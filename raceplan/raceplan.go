package raceplan

import (
	"bytes"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/tormoder/fit"
)

const (
	earthRadiusM           = 6371000.0
	minClimbLengthM        = 600.0
	minClimbGainM          = 30.0
	minClimbAvgGradePct    = 2.5
	climbStartGradePct     = 3.0
	climbContinueGradePct  = 1.0
	climbDipToleranceM     = 200.0
	turnPressureAngleDeg   = 55.0
	turnPressureSpacingM   = 150.0
	baseFuelStartSeconds   = 20 * 60.0
	defaultBottleML        = 650.0
	defaultFluidMLPerHour  = 650.0
	defaultCarbModerateGPH = 60.0
	defaultCarbLongGPH     = 75.0
	defaultCarbVeryLongGPH = 90.0
)

// Profile captures athlete inputs that shape the race plan.
type Profile struct {
	FTPWatts        float64 `json:"ftp_w,omitempty"`
	WeightKG        float64 `json:"weight_kg,omitempty"`
	MaxCarbGPerHour float64 `json:"max_carb_g_per_h,omitempty"`
	BottleML        float64 `json:"bottle_ml,omitempty"`
	StartBottles    int     `json:"start_bottles,omitempty"`
	CaffeineMgPerKG float64 `json:"caffeine_mg_per_kg,omitempty"`
	Goal            string  `json:"goal,omitempty"`
	RiderType       string  `json:"rider_type,omitempty"`
	WeeklyHours     float64 `json:"weekly_hours,omitempty"`
	WeeklyKM        float64 `json:"weekly_km,omitempty"`
	LongestRideKM   float64 `json:"longest_recent_ride_km,omitempty"`
	TeamSupport     string  `json:"team_support,omitempty"`
	TechnicalSkill  string  `json:"technical_confidence,omitempty"`
	StrategyMode    string  `json:"strategy_mode,omitempty"`
}

// Plan is the structured race strategy output.
type Plan struct {
	SourceFileName           string            `json:"source_file_name"`
	SourceType               string            `json:"source_type"`
	CourseName               string            `json:"course_name,omitempty"`
	Sport                    string            `json:"sport,omitempty"`
	Profile                  ProfileSummary    `json:"profile"`
	DistanceMeters           float64           `json:"distance_meters"`
	ElevationGainM           float64           `json:"elevation_gain_m"`
	EstimatedDurationSeconds float64           `json:"estimated_duration_seconds"`
	EstimatedDurationLowS    float64           `json:"estimated_duration_low_seconds"`
	EstimatedDurationHighS   float64           `json:"estimated_duration_high_seconds"`
	EstimatedAverageSpeedKPH float64           `json:"estimated_average_speed_kph"`
	RiderType                string            `json:"rider_type"`
	LongestClimb             *Climb            `json:"longest_climb,omitempty"`
	Climbs                   []Climb           `json:"climbs,omitempty"`
	PressurePoints           []PressurePoint   `json:"pressure_points,omitempty"`
	DecisiveSectors          []DecisiveSector  `json:"decisive_sectors,omitempty"`
	FuelPlan                 FuelPlan          `json:"fuel_plan"`
	Strategy                 []StrategySection `json:"strategy,omitempty"`
	Warnings                 []string          `json:"warnings,omitempty"`
}

// ProfileSummary is the normalized rider profile included in the plan.
type ProfileSummary struct {
	FTPWatts        float64 `json:"ftp_watts,omitempty"`
	WeightKG        float64 `json:"weight_kg,omitempty"`
	WPerKG          float64 `json:"w_per_kg,omitempty"`
	MaxCarbGPerHour float64 `json:"max_carb_g_per_hour,omitempty"`
	BottleML        float64 `json:"bottle_ml,omitempty"`
	StartBottles    int     `json:"start_bottles,omitempty"`
	CaffeineMgPerKG float64 `json:"caffeine_mg_per_kg,omitempty"`
	Goal            string  `json:"goal,omitempty"`
	RiderType       string  `json:"rider_type,omitempty"`
	WeeklyHours     float64 `json:"weekly_hours,omitempty"`
	WeeklyKM        float64 `json:"weekly_km,omitempty"`
	LongestRideKM   float64 `json:"longest_recent_ride_km,omitempty"`
	TeamSupport     string  `json:"team_support,omitempty"`
	TechnicalSkill  string  `json:"technical_confidence,omitempty"`
	StrategyMode    string  `json:"strategy_mode"`
}

// Climb is one route segment likely to shape the race.
type Climb struct {
	Name                     string  `json:"name"`
	StartKM                  float64 `json:"start_km"`
	EndKM                    float64 `json:"end_km"`
	LengthKM                 float64 `json:"length_km"`
	GainM                    float64 `json:"gain_m"`
	AvgGradePct              float64 `json:"avg_grade_pct"`
	MaxGradePct              float64 `json:"max_grade_pct"`
	Severity                 string  `json:"severity"`
	EstimatedDurationSeconds float64 `json:"estimated_duration_seconds,omitempty"`
}

// PressurePoint flags where the race is likely to become harder to manage.
type PressurePoint struct {
	DistanceKM float64 `json:"distance_km"`
	Category   string  `json:"category"`
	Severity   string  `json:"severity"`
	Title      string  `json:"title"`
	Reason     string  `json:"reason"`
	Action     string  `json:"action"`
}

// DecisiveSector is a candidate selection or attack window.
type DecisiveSector struct {
	StartKM           float64 `json:"start_km"`
	EndKM             float64 `json:"end_km"`
	Type              string  `json:"type"`
	Title             string  `json:"title"`
	WhyItMatters      string  `json:"why_it_matters"`
	RecommendedAction string  `json:"recommended_action"`
	Score             float64 `json:"score"`
}

// FuelPlan is the pacing and nutrition plan derived from course demands.
type FuelPlan struct {
	CarbTargetGPerHour    float64          `json:"carb_target_g_per_hour"`
	FluidTargetMLPerHour  float64          `json:"fluid_target_ml_per_hour"`
	StartFuelBySeconds    float64          `json:"start_fuel_by_seconds"`
	StartFuelByDistanceKM float64          `json:"start_fuel_by_distance_km"`
	EstimatedTotalCarbG   float64          `json:"estimated_total_carb_g"`
	EstimatedBottleCount  int              `json:"estimated_bottle_count,omitempty"`
	CaffeinePlan          string           `json:"caffeine_plan,omitempty"`
	Checkpoints           []FuelCheckpoint `json:"checkpoints,omitempty"`
	Notes                 []string         `json:"notes,omitempty"`
}

// FuelCheckpoint is one practical feed reminder.
type FuelCheckpoint struct {
	DistanceKM       float64 `json:"distance_km"`
	ApproxTimeSecond float64 `json:"approx_time_seconds"`
	Title            string  `json:"title"`
	Action           string  `json:"action"`
}

// StrategySection groups coach-style tactical advice.
type StrategySection struct {
	Title string   `json:"title"`
	Items []string `json:"items"`
}

type routePoint struct {
	DistanceM  float64
	AltitudeM  float64
	LatDeg     float64
	LonDeg     float64
	ElapsedEst float64
}

type routeSource struct {
	sourceType string
	courseName string
	sport      string
	points     []routePoint
	pointsMeta []fit.CoursePointMsg
}

type feedMarker struct {
	distanceM float64
	title     string
}

type decisiveCandidate struct {
	DecisiveSector
}

// PlanBytes analyzes a FIT course or activity file and builds a race strategy.
func PlanBytes(sourceName string, data []byte, profile Profile) (*Plan, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("fit bytes are required")
	}

	src, err := decodeRouteSource(sourceName, data)
	if err != nil {
		return nil, err
	}
	if len(src.points) < 2 {
		return nil, fmt.Errorf("fit route requires at least two record points")
	}

	profile = normalizeProfile(profile)
	totalDistance := src.points[len(src.points)-1].DistanceM
	elevationGain := totalElevationGain(src.points)
	if totalDistance <= 0 {
		return nil, fmt.Errorf("fit route distance could not be determined")
	}

	estimateRouteTimings(src.points, profile)
	estimatedDuration := src.points[len(src.points)-1].ElapsedEst
	if estimatedDuration <= 0 {
		estimatedDuration = fallbackDurationSeconds(totalDistance, elevationGain, profile)
	}
	estimatedSpeedKPH := totalDistance / estimatedDuration * 3.6
	lowDuration, highDuration := durationBand(estimatedDuration)

	climbs := detectClimbs(src.points)
	for i := range climbs {
		climbs[i].EstimatedDurationSeconds = durationBetween(src.points, climbs[i].StartKM*1000, climbs[i].EndKM*1000)
	}
	longestClimb := pickLongestClimb(climbs)
	feedMarkers := extractFeedMarkers(src.pointsMeta)
	pressurePoints := buildPressurePoints(src.points, src.pointsMeta, climbs, totalDistance)
	decisiveSectors := buildDecisiveSectors(profile, climbs, pressurePoints, totalDistance)
	fuelPlan := buildFuelPlan(profile, src.points, climbs, feedMarkers, decisiveSectors)
	strategy := buildStrategy(profile, src, climbs, pressurePoints, decisiveSectors, fuelPlan, totalDistance)

	plan := &Plan{
		SourceFileName:           sourceName,
		SourceType:               src.sourceType,
		CourseName:               src.courseName,
		Sport:                    src.sport,
		Profile:                  summarizeProfile(profile),
		DistanceMeters:           round(totalDistance, 1),
		ElevationGainM:           round(elevationGain, 1),
		EstimatedDurationSeconds: round(estimatedDuration, 1),
		EstimatedDurationLowS:    round(lowDuration, 1),
		EstimatedDurationHighS:   round(highDuration, 1),
		EstimatedAverageSpeedKPH: round(estimatedSpeedKPH, 1),
		RiderType:                inferRiderType(profile),
		LongestClimb:             longestClimb,
		Climbs:                   climbs,
		PressurePoints:           pressurePoints,
		DecisiveSectors:          decisiveSectors,
		FuelPlan:                 fuelPlan,
		Strategy:                 strategy,
		Warnings:                 buildWarnings(src, profile, climbs, pressurePoints, totalDistance),
	}
	return plan, nil
}

// BuildMarkdown converts a plan into a shareable text summary.
func BuildMarkdown(plan *Plan) string {
	if plan == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("# Race Plan\n\n")
	b.WriteString("## Course Snapshot\n")
	if plan.CourseName != "" {
		fmt.Fprintf(&b, "- Course: %s\n", plan.CourseName)
	}
	fmt.Fprintf(&b, "- Source: %s FIT\n", cleanLabel(plan.SourceType))
	fmt.Fprintf(&b, "- Distance: %.1f km\n", plan.DistanceMeters/1000)
	fmt.Fprintf(&b, "- Elevation gain: %.0f m\n", plan.ElevationGainM)
	fmt.Fprintf(&b, "- Estimated duration: %s (range %s-%s)\n", formatDuration(plan.EstimatedDurationSeconds), formatDuration(plan.EstimatedDurationLowS), formatDuration(plan.EstimatedDurationHighS))
	fmt.Fprintf(&b, "- Estimated average speed: %.1f km/h\n", plan.EstimatedAverageSpeedKPH)
	fmt.Fprintf(&b, "- Rider type: %s\n", cleanLabel(plan.RiderType))
	if plan.Profile.Goal != "" {
		fmt.Fprintf(&b, "- Goal: %s\n", cleanLabel(plan.Profile.Goal))
	}
	if label := recentVolumeLabel(plan.Profile.WeeklyHours, plan.Profile.WeeklyKM); label != "" {
		fmt.Fprintf(&b, "- Recent volume: %s\n", label)
	}
	if plan.Profile.LongestRideKM > 0 {
		fmt.Fprintf(&b, "- Longest recent ride: %.0f km\n", plan.Profile.LongestRideKM)
	}
	if plan.Profile.TeamSupport != "" {
		fmt.Fprintf(&b, "- Team support: %s\n", cleanLabel(plan.Profile.TeamSupport))
	}

	b.WriteString("\n## Fueling\n")
	fmt.Fprintf(&b, "- Carb target: %.0f g/h\n", plan.FuelPlan.CarbTargetGPerHour)
	fmt.Fprintf(&b, "- Fluid target: %.0f mL/h\n", plan.FuelPlan.FluidTargetMLPerHour)
	fmt.Fprintf(&b, "- Start fueling by: %s or %.1f km\n", formatDuration(plan.FuelPlan.StartFuelBySeconds), plan.FuelPlan.StartFuelByDistanceKM)
	if plan.FuelPlan.EstimatedBottleCount > 0 {
		fmt.Fprintf(&b, "- Estimated bottles needed: %d\n", plan.FuelPlan.EstimatedBottleCount)
	}
	if plan.FuelPlan.CaffeinePlan != "" {
		fmt.Fprintf(&b, "- Caffeine: %s\n", plan.FuelPlan.CaffeinePlan)
	}
	for _, note := range plan.FuelPlan.Notes {
		fmt.Fprintf(&b, "- %s\n", note)
	}

	if len(plan.Climbs) > 0 {
		b.WriteString("\n## Key Climbs\n")
		for _, climb := range plan.Climbs {
			fmt.Fprintf(&b, "- %s: km %.1f-%.1f | %.1f km | +%.0f m | %.1f%% avg | %.1f%% max\n",
				climb.Name,
				climb.StartKM,
				climb.EndKM,
				climb.LengthKM,
				climb.GainM,
				climb.AvgGradePct,
				climb.MaxGradePct,
			)
		}
	}

	if len(plan.PressurePoints) > 0 {
		b.WriteString("\n## Pressure Points\n")
		for _, point := range plan.PressurePoints {
			fmt.Fprintf(&b, "- km %.1f %s: %s\n", point.DistanceKM, point.Title, point.Action)
		}
	}

	if len(plan.DecisiveSectors) > 0 {
		b.WriteString("\n## Decisive Sectors\n")
		for _, sector := range plan.DecisiveSectors {
			fmt.Fprintf(&b, "- %s (km %.1f-%.1f): %s\n", sector.Title, sector.StartKM, sector.EndKM, sector.RecommendedAction)
		}
	}

	if len(plan.Strategy) > 0 {
		b.WriteString("\n## Tactical Gameplan\n")
		for _, section := range plan.Strategy {
			fmt.Fprintf(&b, "### %s\n", section.Title)
			for _, item := range section.Items {
				fmt.Fprintf(&b, "- %s\n", item)
			}
			b.WriteString("\n")
		}
	}

	if len(plan.Warnings) > 0 {
		b.WriteString("## Notes\n")
		for _, warning := range plan.Warnings {
			fmt.Fprintf(&b, "- %s\n", warning)
		}
	}
	return strings.TrimSpace(b.String()) + "\n"
}

func decodeRouteSource(sourceName string, data []byte) (*routeSource, error) {
	decoded, err := fit.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode FIT payload: %w", err)
	}
	if course, err := decoded.Course(); err == nil {
		name := strings.TrimSpace(sourceName)
		if course.Course != nil && strings.TrimSpace(course.Course.Name) != "" {
			name = strings.TrimSpace(course.Course.Name)
		}
		sport := ""
		if course.Course != nil {
			sport = cleanLabel(fmt.Sprint(course.Course.Sport))
		}
		return &routeSource{
			sourceType: "course",
			courseName: name,
			sport:      sport,
			points:     buildRoutePoints(course.Records),
			pointsMeta: flattenCoursePoints(course.CoursePoints),
		}, nil
	}
	if activity, err := decoded.Activity(); err == nil {
		sport := ""
		if len(activity.Sessions) > 0 {
			sport = cleanLabel(fmt.Sprint(activity.Sessions[0].Sport))
		}
		return &routeSource{
			sourceType: "activity",
			courseName: strings.TrimSpace(sourceName),
			sport:      sport,
			points:     buildRoutePoints(activity.Records),
		}, nil
	}
	return nil, fmt.Errorf("expected FIT course or activity file")
}

func flattenCoursePoints(values []*fit.CoursePointMsg) []fit.CoursePointMsg {
	out := make([]fit.CoursePointMsg, 0, len(values))
	for _, value := range values {
		if value == nil {
			continue
		}
		out = append(out, *value)
	}
	return out
}

func buildRoutePoints(records []*fit.RecordMsg) []routePoint {
	points := make([]routePoint, 0, len(records))
	prevDist := 0.0
	prevLat := math.NaN()
	prevLon := math.NaN()
	prevAlt := math.NaN()
	for _, record := range records {
		if record == nil {
			continue
		}

		dist := record.GetDistanceScaled()
		lat := math.NaN()
		lon := math.NaN()
		if !record.PositionLat.Invalid() {
			lat = record.PositionLat.Degrees()
		}
		if !record.PositionLong.Invalid() {
			lon = record.PositionLong.Degrees()
		}
		if !isFinitePositiveOrZero(dist) {
			if isFinite(lat) && isFinite(lon) && isFinite(prevLat) && isFinite(prevLon) {
				dist = prevDist + haversineMeters(prevLat, prevLon, lat, lon)
			} else {
				dist = prevDist
			}
		}
		if dist < prevDist {
			dist = prevDist
		}

		alt := record.GetEnhancedAltitudeScaled()
		if !isFinite(alt) {
			alt = record.GetAltitudeScaled()
		}
		if !isFinite(alt) {
			alt = prevAlt
		}
		if !isFinite(alt) {
			alt = 0
		}

		point := routePoint{
			DistanceM: round(dist, 1),
			AltitudeM: round(alt, 1),
			LatDeg:    lat,
			LonDeg:    lon,
		}
		if len(points) == 0 || point.DistanceM > points[len(points)-1].DistanceM || point.AltitudeM != points[len(points)-1].AltitudeM {
			points = append(points, point)
		}

		prevDist = dist
		prevLat = lat
		prevLon = lon
		prevAlt = alt
	}
	if len(points) == 0 {
		return nil
	}
	points[0].DistanceM = 0
	return points
}

func totalElevationGain(points []routePoint) float64 {
	total := 0.0
	for i := 1; i < len(points); i++ {
		delta := points[i].AltitudeM - points[i-1].AltitudeM
		if delta > 0 {
			total += delta
		}
	}
	return total
}

func estimateRouteTimings(points []routePoint, profile Profile) {
	if len(points) == 0 {
		return
	}
	points[0].ElapsedEst = 0
	totalDistance := points[len(points)-1].DistanceM
	totalGain := totalElevationGain(points)
	if profile.FTPWatts <= 0 || profile.WeightKG <= 0 {
		speedKPH := fallbackAverageSpeedKPH(totalDistance, totalGain, profile)
		speedMPS := math.Max(4.0, speedKPH/3.6)
		for i := 1; i < len(points); i++ {
			delta := math.Max(0, points[i].DistanceM-points[i-1].DistanceM)
			grade := localGradePct(points, i)
			localSpeed := speedMPS * (1 - clamp(grade*0.02, -0.18, 0.45))
			if grade < -4 {
				localSpeed = math.Min(localSpeed*1.2, 18.0)
			}
			if localSpeed < 3.5 {
				localSpeed = 3.5
			}
			points[i].ElapsedEst = points[i-1].ElapsedEst + delta/localSpeed
		}
		return
	}

	mode := profile.StrategyMode
	if mode == "" {
		mode = "balanced"
	}
	baseFactor := 0.72
	switch mode {
	case "conservative":
		baseFactor = 0.68
	case "aggressive":
		baseFactor = 0.78
	}

	for i := 1; i < len(points); i++ {
		delta := math.Max(0, points[i].DistanceM-points[i-1].DistanceM)
		if delta == 0 {
			points[i].ElapsedEst = points[i-1].ElapsedEst
			continue
		}
		grade := localGradePct(points, i)
		powerFactor := baseFactor
		switch {
		case grade >= 6:
			powerFactor += 0.10
		case grade >= 3:
			powerFactor += 0.06
		case grade >= 1:
			powerFactor += 0.03
		case grade <= -4:
			powerFactor -= 0.10
		case grade <= -1:
			powerFactor -= 0.05
		}
		power := clamp(profile.FTPWatts*powerFactor, 120, profile.FTPWatts*0.9)
		speed := solveSpeed(power, profile.WeightKG, grade)
		if speed < 3.8 {
			speed = 3.8
		}
		points[i].ElapsedEst = points[i-1].ElapsedEst + delta/speed
	}
}

func fallbackDurationSeconds(totalDistanceM, totalGainM float64, profile Profile) float64 {
	speedKPH := fallbackAverageSpeedKPH(totalDistanceM, totalGainM, profile)
	return totalDistanceM / (speedKPH / 3.6)
}

func fallbackAverageSpeedKPH(totalDistanceM, totalGainM float64, profile Profile) float64 {
	distKM := totalDistanceM / 1000
	if distKM <= 0 {
		return 28
	}
	climbDensity := totalGainM / distKM
	speed := 32.0 - clamp(climbDensity*0.02, 0, 8)
	switch profile.StrategyMode {
	case "conservative":
		speed -= 1.0
	case "aggressive":
		speed += 1.0
	}
	return clamp(speed, 22, 38)
}

func solveSpeed(powerW, weightKG, gradePct float64) float64 {
	if powerW <= 0 || weightKG <= 0 {
		return 0
	}
	grade := clamp(gradePct/100.0, -0.12, 0.15)
	rolling := 0.004
	gravityTerm := weightKG * 9.81 * (rolling + grade)
	cda := 0.32
	if grade >= 0.04 {
		cda = 0.36
	}
	rho := 1.225
	low, high := 0.5, 30.0
	for i := 0; i < 45; i++ {
		mid := (low + high) / 2
		required := mid * (gravityTerm + 0.5*rho*cda*mid*mid)
		if required > powerW {
			high = mid
		} else {
			low = mid
		}
	}
	return low
}

func detectClimbs(points []routePoint) []Climb {
	climbs := make([]Climb, 0, 8)
	if len(points) < 2 {
		return climbs
	}

	start := -1
	gain := 0.0
	maxGrade := 0.0
	dipDistance := 0.0
	for i := 1; i < len(points); i++ {
		deltaDist := points[i].DistanceM - points[i-1].DistanceM
		if deltaDist < 10 {
			continue
		}
		deltaAlt := points[i].AltitudeM - points[i-1].AltitudeM
		grade := 100 * deltaAlt / deltaDist
		if start == -1 {
			if grade >= climbStartGradePct && deltaAlt > 0 {
				start = i - 1
				gain = math.Max(deltaAlt, 0)
				maxGrade = grade
				dipDistance = 0
			}
			continue
		}

		if deltaAlt > 0 {
			gain += deltaAlt
		}
		if grade > maxGrade {
			maxGrade = grade
		}
		if grade < climbContinueGradePct {
			dipDistance += deltaDist
		} else {
			dipDistance = 0
		}

		endClimb := dipDistance > climbDipToleranceM
		lastPoint := i == len(points)-1
		if !(endClimb || lastPoint) {
			continue
		}

		endIdx := i
		lengthM := points[endIdx].DistanceM - points[start].DistanceM
		avgGrade := 0.0
		if lengthM > 0 {
			avgGrade = 100 * gain / lengthM
		}
		if lengthM >= minClimbLengthM && gain >= minClimbGainM && avgGrade >= minClimbAvgGradePct {
			climbs = append(climbs, Climb{
				Name:        fmt.Sprintf("Climb %d", len(climbs)+1),
				StartKM:     round(points[start].DistanceM/1000, 1),
				EndKM:       round(points[endIdx].DistanceM/1000, 1),
				LengthKM:    round(lengthM/1000, 1),
				GainM:       round(gain, 0),
				AvgGradePct: round(avgGrade, 1),
				MaxGradePct: round(maxGrade, 1),
				Severity:    classifyClimb(lengthM, gain, avgGrade, maxGrade),
			})
		}
		start = -1
		gain = 0
		maxGrade = 0
		dipDistance = 0
	}

	return climbs
}

func classifyClimb(lengthM, gainM, avgGradePct, maxGradePct float64) string {
	switch {
	case gainM >= 300 || (lengthM >= 5000 && avgGradePct >= 5):
		return "major"
	case gainM >= 120 || (lengthM >= 2000 && avgGradePct >= 4):
		return "selective"
	case maxGradePct >= 8 || avgGradePct >= 6:
		return "punchy"
	default:
		return "rolling"
	}
}

func pickLongestClimb(climbs []Climb) *Climb {
	if len(climbs) == 0 {
		return nil
	}
	best := climbs[0]
	for _, climb := range climbs[1:] {
		if climb.LengthKM > best.LengthKM {
			best = climb
		}
	}
	return &best
}

func extractFeedMarkers(points []fit.CoursePointMsg) []feedMarker {
	out := make([]feedMarker, 0, len(points))
	for _, point := range points {
		distance := point.GetDistanceScaled()
		if !isFinitePositiveOrZero(distance) {
			continue
		}
		title := strings.TrimSpace(point.Name)
		if title == "" {
			title = cleanLabel(fmt.Sprint(point.Type))
		}
		switch point.Type {
		case fit.CoursePointFood, fit.CoursePointWater, fit.CoursePointAidStation, fit.CoursePointSportsDrink, fit.CoursePointEnergyGel, fit.CoursePointService, fit.CoursePointRestArea:
			out = append(out, feedMarker{distanceM: distance, title: title})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].distanceM < out[j].distanceM })
	return out
}

func buildPressurePoints(points []routePoint, coursePoints []fit.CoursePointMsg, climbs []Climb, totalDistanceM float64) []PressurePoint {
	pressure := make([]PressurePoint, 0, len(coursePoints)+len(climbs)+4)
	add := func(point PressurePoint) {
		for _, existing := range pressure {
			if math.Abs(existing.DistanceKM-point.DistanceKM) < 0.2 && existing.Category == point.Category {
				return
			}
		}
		pressure = append(pressure, point)
	}

	for _, coursePoint := range coursePoints {
		distance := coursePoint.GetDistanceScaled()
		if !isFinitePositiveOrZero(distance) {
			continue
		}
		km := round(distance/1000, 1)
		switch coursePoint.Type {
		case fit.CoursePointDanger, fit.CoursePointBridge, fit.CoursePointTunnel, fit.CoursePointCrossing, fit.CoursePointObstacle, fit.CoursePointAlert:
			add(PressurePoint{
				DistanceKM: km,
				Category:   "hazard",
				Severity:   "high",
				Title:      cleanLabel(fmt.Sprint(coursePoint.Type)),
				Reason:     "The FIT route explicitly flags this spot as a safety or handling feature.",
				Action:     "Move up early, stay predictable, and avoid making your move through the hazard itself.",
			})
		case fit.CoursePointSharpLeft, fit.CoursePointSharpRight, fit.CoursePointUTurn, fit.CoursePointSharpCurve, fit.CoursePointLeftFork, fit.CoursePointRightFork, fit.CoursePointMiddleFork:
			add(PressurePoint{
				DistanceKM: km,
				Category:   "technical",
				Severity:   "medium",
				Title:      cleanLabel(fmt.Sprint(coursePoint.Type)),
				Reason:     "This marked direction change can string the field out and create accordion effects.",
				Action:     "Enter near the front third of the group so you are not forced into a hard sprint out of the corner.",
			})
		case fit.CoursePointSteepIncline, fit.CoursePointFirstCategory, fit.CoursePointSecondCategory, fit.CoursePointThirdCategory, fit.CoursePointFourthCategory, fit.CoursePointHorsCategory, fit.CoursePointSummit:
			add(PressurePoint{
				DistanceKM: km,
				Category:   "climb_entry",
				Severity:   "high",
				Title:      cleanLabel(fmt.Sprint(coursePoint.Type)),
				Reason:     "The route metadata marks this as a climb or summit trigger where speed is likely to drop sharply.",
				Action:     "Shift early and hold position before the slope bites; it is cheaper to move up before the pace stalls.",
			})
		}
	}

	for _, climb := range climbs {
		entryKM := round(math.Max(0, climb.StartKM-0.3), 1)
		add(PressurePoint{
			DistanceKM: entryKM,
			Category:   "climb_entry",
			Severity:   climbSeverityLevel(climb.Severity),
			Title:      climb.Name + " entry",
			Reason:     fmt.Sprintf("The road tilts up into a %.1f km climb averaging %.1f%%.", climb.LengthKM, climb.AvgGradePct),
			Action:     "Start this climb in the front third. Protect your line before the gradient lowers the group speed.",
		})
	}

	for i := 1; i < len(points)-1; i++ {
		if !hasValidGeo(points[i-1]) || !hasValidGeo(points[i]) || !hasValidGeo(points[i+1]) {
			continue
		}
		d1 := points[i].DistanceM - points[i-1].DistanceM
		d2 := points[i+1].DistanceM - points[i].DistanceM
		if d1 <= 10 || d2 <= 10 || d1 > turnPressureSpacingM || d2 > turnPressureSpacingM {
			continue
		}
		turn := turnAngle(points[i-1], points[i], points[i+1])
		if turn < turnPressureAngleDeg {
			continue
		}
		km := round(points[i].DistanceM/1000, 1)
		severity := "medium"
		action := "Be ahead of the surge so you do not have to close a gap on the exit."
		if totalDistanceM-points[i].DistanceM <= 5000 {
			severity = "high"
			action = "Fight for front-third position before this turn; the finale will magnify every acceleration."
		}
		add(PressurePoint{
			DistanceKM: km,
			Category:   "technical",
			Severity:   severity,
			Title:      "Geometric turn pressure point",
			Reason:     fmt.Sprintf("The route direction changes by roughly %.0f° here, which is enough to create a concertina through the bunch.", turn),
			Action:     action,
		})
	}

	sort.Slice(pressure, func(i, j int) bool { return pressure[i].DistanceKM < pressure[j].DistanceKM })
	if len(pressure) > 10 {
		pressure = append([]PressurePoint(nil), pressure[:10]...)
	}
	return pressure
}

func buildDecisiveSectors(profile Profile, climbs []Climb, pressurePoints []PressurePoint, totalDistanceM float64) []DecisiveSector {
	totalKM := totalDistanceM / 1000
	candidates := make([]decisiveCandidate, 0, len(climbs)+3)

	for _, climb := range climbs {
		lateFactor := climb.StartKM / math.Max(totalKM, 1)
		score := climb.GainM*0.12 + climb.AvgGradePct*7 + lateFactor*40
		if climb.Severity == "major" {
			score += 15
		}
		if climb.MaxGradePct >= 8 {
			score += 8
		}
		candidates = append(candidates, decisiveCandidate{
			DecisiveSector: DecisiveSector{
				StartKM:           climb.StartKM,
				EndKM:             climb.EndKM,
				Type:              "climb",
				Title:             climb.Name,
				WhyItMatters:      fmt.Sprintf("This %s climb is long or steep enough to reduce the peloton's aerodynamic advantage and force selection.", climb.Severity),
				RecommendedAction: recommendClimbMove(profile, climb, totalKM),
				Score:             round(score, 1),
			},
		})
	}

	for _, point := range pressurePoints {
		if point.Category != "technical" || point.DistanceKM < math.Max(0, totalKM-5) {
			continue
		}
		score := 35 + (totalKM-point.DistanceKM)*2
		candidates = append(candidates, decisiveCandidate{
			DecisiveSector: DecisiveSector{
				StartKM:           point.DistanceKM,
				EndKM:             point.DistanceKM + 0.3,
				Type:              "technical_finish",
				Title:             "Final technical run-in",
				WhyItMatters:      "Late technical direction changes increase the cost of being too far back and can decide whether you start the finish sequence already boxed in.",
				RecommendedAction: "Be inside the front third before this run-in starts. Save your acceleration for the exit rather than braking mid-corner.",
				Score:             round(score, 1),
			},
		})
	}

	sort.Slice(candidates, func(i, j int) bool { return candidates[i].Score > candidates[j].Score })
	out := make([]DecisiveSector, 0, min(4, len(candidates)))
	for _, candidate := range candidates {
		if len(out) == 4 {
			break
		}
		out = append(out, candidate.DecisiveSector)
	}
	return out
}

func recommendClimbMove(profile Profile, climb Climb, totalKM float64) string {
	mode := profile.StrategyMode
	if mode == "" {
		mode = "balanced"
	}
	isLate := climb.StartKM >= totalKM*0.65 || totalKM-climb.EndKM <= 25
	riderType := inferRiderType(profile)
	switch {
	case isLate && (riderType == "climber" || riderType == "puncheur") && mode == "aggressive":
		return "This is a legitimate attack window. Hit the steepest middle-to-upper section, crest committed, and force the split over the top."
	case isLate && (riderType == "climber" || riderType == "all_rounder"):
		return "Use this climb to make the selection. Follow the first serious move and counter only if the group hesitates near the top."
	case isLate:
		return "Do not start too far back. Treat this as a selection point and hold the front group rather than making the first long move."
	default:
		return "Use this sector to save energy through good positioning. Only spend a match here if the front split would otherwise go without you."
	}
}

func buildFuelPlan(profile Profile, points []routePoint, climbs []Climb, feedMarkers []feedMarker, decisive []DecisiveSector) FuelPlan {
	totalSeconds := points[len(points)-1].ElapsedEst
	totalDistanceKM := points[len(points)-1].DistanceM / 1000
	carbTarget := carbTargetGPH(totalSeconds, profile.MaxCarbGPerHour)
	fluidTarget := fluidTargetMLPH(totalSeconds)
	bottleML := profile.BottleML
	if bottleML <= 0 {
		bottleML = defaultBottleML
	}
	bottleCount := int(math.Ceil((totalSeconds / 3600) * fluidTarget / bottleML))

	startFuelKM := distanceAtTime(points, baseFuelStartSeconds) / 1000
	plan := FuelPlan{
		CarbTargetGPerHour:    round(carbTarget, 0),
		FluidTargetMLPerHour:  round(fluidTarget, 0),
		StartFuelBySeconds:    baseFuelStartSeconds,
		StartFuelByDistanceKM: round(startFuelKM, 1),
		EstimatedTotalCarbG:   round((totalSeconds/3600)*carbTarget, 0),
		EstimatedBottleCount:  maxInt(bottleCount, profile.StartBottles),
		CaffeinePlan:          caffeinePlan(profile, totalSeconds, decisive),
		Notes: []string{
			"Base plan assumes temperate conditions. In heat, increase fluid and sodium-containing drink access rather than forcing plain water.",
			"Drink to thirst inside the plan; avoid overdrinking the easy early kilometers.",
		},
	}

	checkpoints := make([]FuelCheckpoint, 0, 10)
	addCheckpoint := func(distanceM, timeS float64, title, action string) {
		for _, existing := range checkpoints {
			if math.Abs(existing.ApproxTimeSecond-timeS) < 8*60 || math.Abs(existing.DistanceKM-distanceM/1000) < 4 {
				return
			}
		}
		checkpoints = append(checkpoints, FuelCheckpoint{
			DistanceKM:       round(distanceM/1000, 1),
			ApproxTimeSecond: round(timeS, 0),
			Title:            title,
			Action:           action,
		})
	}

	for t := baseFuelStartSeconds; t < totalSeconds-15*60; t += 30 * 60 {
		distanceM := distanceAtTime(points, t)
		addCheckpoint(distanceM, t, "Regular fueling", "Take 20-30 g carbohydrate and keep sipping from the bottle rather than waiting for a long gap between feeds.")
	}

	for _, marker := range feedMarkers {
		timeS := timeAtDistance(points, marker.distanceM)
		addCheckpoint(marker.distanceM, timeS, marker.title, "Use this marked feed opportunity to replace a bottle and top up carbohydrates before the next hard sector.")
	}

	for _, climb := range climbs {
		if climb.EstimatedDurationSeconds < 8*60 || climb.StartKM <= 8 || climb.EndKM >= totalDistanceKM-5 {
			continue
		}
		timeS := timeAtDistance(points, climb.StartKM*1000) - 10*60
		if timeS <= baseFuelStartSeconds {
			continue
		}
		distanceM := distanceAtTime(points, timeS)
		addCheckpoint(distanceM, timeS, "Pre-climb top-off", fmt.Sprintf("Eat before %s so the climb starts fueled. Do not wait until the gradient bites.", climb.Name))
	}

	sort.Slice(checkpoints, func(i, j int) bool { return checkpoints[i].ApproxTimeSecond < checkpoints[j].ApproxTimeSecond })
	if len(checkpoints) > 8 {
		checkpoints = append([]FuelCheckpoint(nil), checkpoints[:8]...)
	}
	plan.Checkpoints = checkpoints
	return plan
}

func buildStrategy(profile Profile, src *routeSource, climbs []Climb, pressure []PressurePoint, decisive []DecisiveSector, fuel FuelPlan, totalDistanceM float64) []StrategySection {
	sections := make([]StrategySection, 0, 4)
	totalKM := totalDistanceM / 1000
	riderType := inferRiderType(profile)

	preRaceItems := []string{
		fmt.Sprintf("Start taking in carbohydrate by %s, which is roughly km %.1f on this route.", formatDuration(fuel.StartFuelBySeconds), fuel.StartFuelByDistanceKM),
		fmt.Sprintf("Target about %.0f g carbohydrate and %.0f mL fluid per hour. Build that into bottles and pockets before the start.", fuel.CarbTargetGPerHour, fuel.FluidTargetMLPerHour),
	}
	if goal := goalGuidance(profile.Goal); goal != "" {
		preRaceItems = append(preRaceItems, goal)
	}
	if context := trainingContext(profile, totalKM); context != "" {
		preRaceItems = append(preRaceItems, context)
	}
	if fuel.CaffeinePlan != "" {
		preRaceItems = append(preRaceItems, fuel.CaffeinePlan)
	}
	if len(climbs) > 0 {
		steepest := climbs[0]
		for _, climb := range climbs[1:] {
			if climb.MaxGradePct > steepest.MaxGradePct {
				steepest = climb
			}
		}
		preRaceItems = append(preRaceItems, fmt.Sprintf("Gear for the route, not your ego: the steepest ramp reaches %.1f%%.", steepest.MaxGradePct))
	}
	sections = append(sections, StrategySection{Title: "Pre-race", Items: preRaceItems})

	openingItems := []string{
		"Ride the opening phase sheltered. Do not spend matches taking pointless pulls before the first real pressure point.",
	}
	if teamNote := teamGuidance(profile.TeamSupport, "opening"); teamNote != "" {
		openingItems = append(openingItems, teamNote)
	}
	if firstPressure := firstByDistance(pressure, 0, totalKM*0.5); firstPressure != nil {
		openingItems = append(openingItems, fmt.Sprintf("Prioritize position before km %.1f because %s is the first place the bunch can string out.", firstPressure.DistanceKM, strings.ToLower(firstPressure.Title)))
	}
	if technical := technicalGuidance(profile.TechnicalSkill, pressure); technical != "" {
		openingItems = append(openingItems, technical)
	}
	openingItems = append(openingItems, pacingGuidance(profile, "opening"))
	sections = append(sections, StrategySection{Title: "Opening third", Items: dedupeStrings(openingItems)})

	midItems := []string{
		"Use the middle of the race to keep feeding and protect front-half position before climbs, hazards, and any marked feed opportunity.",
		pacingGuidance(profile, "middle"),
	}
	if teamNote := teamGuidance(profile.TeamSupport, "middle"); teamNote != "" {
		midItems = append(midItems, teamNote)
	}
	if len(climbs) > 0 {
		midItems = append(midItems, fmt.Sprintf("This course has %d notable climb(s). Treat each entry as a positioning battle before it becomes a power battle.", len(climbs)))
	}
	if len(src.pointsMeta) > 0 {
		midItems = append(midItems, "The route FIT includes course-point metadata, so use those marked hazards and service spots as anchors for your race script.")
	}
	sections = append(sections, StrategySection{Title: "Middle race control", Items: dedupeStrings(midItems)})

	finaleItems := []string{
		fmt.Sprintf("Your rider profile reads as %s, so the finale should be shaped around selective terrain instead of random flat-road hero moves.", cleanLabel(riderType)),
	}
	if finaleGoal := finaleGuidance(profile.Goal); finaleGoal != "" {
		finaleItems = append(finaleItems, finaleGoal)
	}
	if teamNote := teamGuidance(profile.TeamSupport, "finale"); teamNote != "" {
		finaleItems = append(finaleItems, teamNote)
	}
	for _, sector := range decisive {
		finaleItems = append(finaleItems, fmt.Sprintf("%s: %s", sector.Title, sector.RecommendedAction))
	}
	if len(decisive) == 0 {
		finaleItems = append(finaleItems, "No obvious decisive sector stands out from the FIT alone. Default to protecting position and waiting for the strongest terrain inside the final quarter.")
	}
	sections = append(sections, StrategySection{Title: "Finale", Items: dedupeStrings(finaleItems)})

	return sections
}

func pacingGuidance(profile Profile, phase string) string {
	if profile.FTPWatts <= 0 {
		switch phase {
		case "opening":
			return "Without FTP data, pace by restraint early: keep your breathing under control and avoid chasing every surge."
		default:
			return "Without FTP data, use the long climbs to ride steady and save your hard efforts for terrain that can actually split the race."
		}
	}

	type band struct {
		low  float64
		high float64
	}
	longClimb := band{low: 0.95, high: 1.00}
	shortRamp := band{low: 1.05, high: 1.15}
	switch profile.StrategyMode {
	case "conservative":
		longClimb = band{low: 0.92, high: 0.97}
		shortRamp = band{low: 1.00, high: 1.08}
	case "aggressive":
		longClimb = band{low: 0.98, high: 1.03}
		shortRamp = band{low: 1.10, high: 1.20}
	}

	if phase == "opening" {
		return fmt.Sprintf("Use restraint early: long climbs are best opened around %.0f-%.0f%% FTP (%.0f-%.0f W), not with a race-winning move in the first few minutes.",
			longClimb.low*100,
			longClimb.high*100,
			profile.FTPWatts*longClimb.low,
			profile.FTPWatts*longClimb.high,
		)
	}

	return fmt.Sprintf("On short selective ramps, accept brief %.0f-%.0f%% FTP surges (%.0f-%.0f W), then settle quickly so you are still able to act late.",
		shortRamp.low*100,
		shortRamp.high*100,
		profile.FTPWatts*shortRamp.low,
		profile.FTPWatts*shortRamp.high,
	)
}

func goalGuidance(goal string) string {
	switch goal {
	case "finish":
		return "The stated goal is to finish well, so the first rule is simple: skip ego moves early and arrive at the final third properly fueled."
	case "lead_group":
		return "The goal is to make the lead group. Treat every marked pressure point as a position-before, not a chase-after, moment."
	case "top_10":
		return "A top-10 ride usually comes from conserving well enough to start the final quarter in the front group with something left."
	case "podium":
		return "A podium goal requires selective aggression, not constant aggression. Save your real efforts for terrain that can actually reduce the group."
	case "win":
		return "A win target means the plan should bias toward winning terrain rather than survival. Protect matches until the decisive sector, then commit."
	case "support_teammate":
		return "Supporting a teammate changes the script: spend your early matches on positioning and cover, not on protecting your own finish."
	default:
		return ""
	}
}

func finaleGuidance(goal string) string {
	switch goal {
	case "finish":
		return "If the race detonates, switch to damage control quickly: ride your own steady effort on the climbs and keep the fueling pattern intact."
	case "lead_group":
		return "The finale success metric is simple: arrive in the first selection. Do not gamble on speculative late attacks if they risk missing the decisive split."
	case "top_10":
		return "Do not launch first unless the terrain obviously suits you. A measured last-kilometer effort is usually worth more than a heroic long-range move."
	case "podium", "win":
		return "Once the decisive move goes, commit fully. Half-following a winning move is often worse than missing it and resetting for the next acceleration."
	case "support_teammate":
		return "By the finale, your value is in keeping your teammate out of the wind and closing dangerous counters before they become race-defining."
	default:
		return ""
	}
}

func trainingContext(profile Profile, totalKM float64) string {
	parts := make([]string, 0, 3)
	if volume := recentVolumeLabel(profile.WeeklyHours, profile.WeeklyKM); volume != "" {
		parts = append(parts, fmt.Sprintf("Recent volume is about %s.", volume))
	}
	if profile.LongestRideKM > 0 {
		parts = append(parts, fmt.Sprintf("Your longest recent ride is %.0f km.", profile.LongestRideKM))
		switch {
		case totalKM > profile.LongestRideKM*1.15:
			parts = append(parts, "That makes this race a durability test, so the opening phase should feel almost boring.")
		case totalKM <= profile.LongestRideKM*0.9:
			parts = append(parts, "The route is inside familiar durability territory if you stay disciplined.")
		}
	}
	return strings.Join(parts, " ")
}

func teamGuidance(teamSupport, phase string) string {
	switch teamSupport {
	case "solo":
		if phase == "opening" {
			return "You are racing solo, so avoid volunteering for chase work while teams still have helpers to burn."
		}
		if phase == "finale" {
			return "Racing solo means your currency is positioning and patience. Let organized teams expose themselves before you do."
		}
		return ""
	case "teammates":
		switch phase {
		case "opening":
			return "If you have teammates, use them to keep you near the front before the first key squeeze point instead of burning your own matches."
		case "middle":
			return "Have teammates handle low-value bottle runs or dead-road cover so your better efforts are reserved for the decisive sectors."
		case "finale":
			return "Use teammates before the winning move, not after it. One rider should launch the setup, and the protected rider should answer the real move."
		}
	}
	return ""
}

func technicalGuidance(skill string, pressure []PressurePoint) string {
	if !hasPressureCategory(pressure, "technical") {
		return ""
	}
	switch skill {
	case "low":
		return "Technical confidence is marked low. Move up before corners and narrowings, then ride them cleanly; do not try to gain positions inside them."
	case "high":
		return "Technical confidence is a strength. Use clean setup and exits through the marked technical sectors to move up without resorting to full-gas surges."
	default:
		return ""
	}
}

func hasPressureCategory(points []PressurePoint, category string) bool {
	for _, point := range points {
		if point.Category == category {
			return true
		}
	}
	return false
}

func recentVolumeLabel(hours, km float64) string {
	switch {
	case hours > 0 && km > 0:
		return fmt.Sprintf("%.1f h/week | %.0f km/week", hours, km)
	case hours > 0:
		return fmt.Sprintf("%.1f h/week", hours)
	case km > 0:
		return fmt.Sprintf("%.0f km/week", km)
	default:
		return ""
	}
}

func buildWarnings(src *routeSource, profile Profile, climbs []Climb, pressure []PressurePoint, totalDistanceM float64) []string {
	warnings := make([]string, 0, 4)
	totalKM := totalDistanceM / 1000
	if src.sourceType == "activity" {
		warnings = append(warnings, "Route plan was built from an activity FIT, not a dedicated course FIT. That is fine, but explicit course-point metadata may be missing.")
	}
	if profile.FTPWatts <= 0 || profile.WeightKG <= 0 {
		warnings = append(warnings, "Estimated duration and pacing guidance are less specific without both FTP and weight.")
	}
	if profile.LongestRideKM > 0 && totalKM > profile.LongestRideKM*1.15 {
		warnings = append(warnings, fmt.Sprintf("Course distance is meaningfully longer than your longest recent ride (%.0f km course vs %.0f km recent long ride). Respect durability and fuel early.", totalKM, profile.LongestRideKM))
	}
	if profile.WeeklyKM > 0 && totalKM > profile.WeeklyKM*0.7 {
		warnings = append(warnings, fmt.Sprintf("This route is a large fraction of your current weekly volume (%.0f km course vs %.0f km/week). Ride the opening phase conservatively.", totalKM, profile.WeeklyKM))
	}
	if profile.TechnicalSkill == "low" && hasPressureCategory(pressure, "technical") {
		warnings = append(warnings, "Technical confidence is marked low, so the biggest time losses will likely come from poor setup before corners and narrowings rather than from raw climbing power.")
	}
	if len(climbs) == 0 {
		warnings = append(warnings, "No major climbs were detected from the route profile; decisive windows will bias toward late technical sectors instead.")
	}
	warnings = append(warnings, "Pressure points and attack windows are route-driven inferences. Wind, team tactics, and field strength can change the best move on race day.")
	return dedupeStrings(warnings)
}

func summarizeProfile(profile Profile) ProfileSummary {
	summary := ProfileSummary{
		FTPWatts:        round(profile.FTPWatts, 0),
		WeightKG:        round(profile.WeightKG, 1),
		MaxCarbGPerHour: round(profile.MaxCarbGPerHour, 0),
		BottleML:        round(profile.BottleML, 0),
		StartBottles:    profile.StartBottles,
		CaffeineMgPerKG: round(profile.CaffeineMgPerKG, 1),
		Goal:            profile.Goal,
		RiderType:       profile.RiderType,
		WeeklyHours:     round(profile.WeeklyHours, 1),
		WeeklyKM:        round(profile.WeeklyKM, 1),
		LongestRideKM:   round(profile.LongestRideKM, 1),
		TeamSupport:     profile.TeamSupport,
		TechnicalSkill:  profile.TechnicalSkill,
		StrategyMode:    profile.StrategyMode,
	}
	if profile.FTPWatts > 0 && profile.WeightKG > 0 {
		summary.WPerKG = round(profile.FTPWatts/profile.WeightKG, 2)
	}
	return summary
}

func normalizeProfile(profile Profile) Profile {
	profile.FTPWatts = safePositive(profile.FTPWatts)
	profile.WeightKG = safePositive(profile.WeightKG)
	profile.MaxCarbGPerHour = safePositive(profile.MaxCarbGPerHour)
	profile.BottleML = safePositive(profile.BottleML)
	profile.CaffeineMgPerKG = safePositive(profile.CaffeineMgPerKG)
	profile.WeeklyHours = safePositive(profile.WeeklyHours)
	profile.WeeklyKM = safePositive(profile.WeeklyKM)
	profile.LongestRideKM = safePositive(profile.LongestRideKM)
	if profile.StartBottles < 0 {
		profile.StartBottles = 0
	}
	switch strings.ToLower(strings.TrimSpace(profile.Goal)) {
	case "finish", "lead_group", "top_10", "podium", "win", "support_teammate":
		profile.Goal = strings.ToLower(strings.TrimSpace(profile.Goal))
	default:
		profile.Goal = ""
	}
	switch strings.ToLower(strings.TrimSpace(profile.RiderType)) {
	case "climber", "puncheur", "all_rounder", "diesel_rouleur", "steady_endurance_rider", "sprinter":
		profile.RiderType = strings.ToLower(strings.TrimSpace(profile.RiderType))
	default:
		profile.RiderType = ""
	}
	switch strings.ToLower(strings.TrimSpace(profile.TeamSupport)) {
	case "solo", "teammates":
		profile.TeamSupport = strings.ToLower(strings.TrimSpace(profile.TeamSupport))
	default:
		profile.TeamSupport = ""
	}
	switch strings.ToLower(strings.TrimSpace(profile.TechnicalSkill)) {
	case "low", "medium", "high":
		profile.TechnicalSkill = strings.ToLower(strings.TrimSpace(profile.TechnicalSkill))
	default:
		profile.TechnicalSkill = ""
	}
	switch strings.ToLower(strings.TrimSpace(profile.StrategyMode)) {
	case "conservative", "aggressive":
		profile.StrategyMode = strings.ToLower(strings.TrimSpace(profile.StrategyMode))
	default:
		profile.StrategyMode = "balanced"
	}
	return profile
}

func inferRiderType(profile Profile) string {
	if profile.RiderType != "" {
		return profile.RiderType
	}
	if profile.FTPWatts <= 0 || profile.WeightKG <= 0 {
		if profile.FTPWatts > 300 {
			return "diesel_rouleur"
		}
		return "all_rounder"
	}
	wkg := profile.FTPWatts / profile.WeightKG
	switch {
	case wkg >= 4.4:
		return "climber"
	case profile.FTPWatts >= 320 && wkg >= 3.9:
		return "all_rounder"
	case profile.FTPWatts >= 320:
		return "diesel_rouleur"
	case wkg >= 3.9:
		return "puncheur"
	default:
		return "steady_endurance_rider"
	}
}

func carbTargetGPH(totalSeconds, maxCarb float64) float64 {
	target := defaultCarbModerateGPH
	switch {
	case totalSeconds > 4*3600:
		target = defaultCarbVeryLongGPH
	case totalSeconds > 150*60:
		target = defaultCarbLongGPH
	}
	if maxCarb > 0 {
		target = math.Min(target, maxCarb)
	}
	return target
}

func fluidTargetMLPH(totalSeconds float64) float64 {
	switch {
	case totalSeconds > 4*3600:
		return 750
	case totalSeconds > 2*3600:
		return defaultFluidMLPerHour
	default:
		return 550
	}
}

func caffeinePlan(profile Profile, totalSeconds float64, decisive []DecisiveSector) string {
	if profile.CaffeineMgPerKG <= 0 {
		if totalSeconds < 2*3600 {
			return ""
		}
		return "If caffeine is part of your normal race routine, a practiced dose around 3 mg/kg taken 45-60 minutes before the start is the conservative option."
	}
	dose := clamp(profile.CaffeineMgPerKG, 1, 6)
	if totalSeconds > 3*3600 && len(decisive) > 0 {
		return fmt.Sprintf("Use a split caffeine plan: about %.1f mg/kg pre-race, then keep a smaller top-up available before the final decisive sector if you have already practiced that strategy.", dose)
	}
	return fmt.Sprintf("Take about %.1f mg/kg caffeine 45-60 minutes before the start if that dose is already familiar to you.", dose)
}

func distanceBetween(points []routePoint, startM, endM float64) float64 {
	return math.Max(0, endM-startM)
}

func durationBetween(points []routePoint, startM, endM float64) float64 {
	return math.Max(0, timeAtDistance(points, endM)-timeAtDistance(points, startM))
}

func localGradePct(points []routePoint, idx int) float64 {
	if idx <= 0 || idx >= len(points) {
		return 0
	}
	deltaDist := points[idx].DistanceM - points[idx-1].DistanceM
	if deltaDist <= 0 {
		return 0
	}
	return clamp(100*(points[idx].AltitudeM-points[idx-1].AltitudeM)/deltaDist, -15, 15)
}

func timeAtDistance(points []routePoint, distanceM float64) float64 {
	if len(points) == 0 {
		return 0
	}
	if distanceM <= 0 {
		return 0
	}
	if distanceM >= points[len(points)-1].DistanceM {
		return points[len(points)-1].ElapsedEst
	}
	i := sort.Search(len(points), func(i int) bool {
		return points[i].DistanceM >= distanceM
	})
	if i == 0 {
		return points[0].ElapsedEst
	}
	prev := points[i-1]
	curr := points[i]
	if curr.DistanceM <= prev.DistanceM {
		return curr.ElapsedEst
	}
	ratio := (distanceM - prev.DistanceM) / (curr.DistanceM - prev.DistanceM)
	return prev.ElapsedEst + ratio*(curr.ElapsedEst-prev.ElapsedEst)
}

func distanceAtTime(points []routePoint, elapsedS float64) float64 {
	if len(points) == 0 {
		return 0
	}
	if elapsedS <= 0 {
		return 0
	}
	if elapsedS >= points[len(points)-1].ElapsedEst {
		return points[len(points)-1].DistanceM
	}
	i := sort.Search(len(points), func(i int) bool {
		return points[i].ElapsedEst >= elapsedS
	})
	if i == 0 {
		return points[0].DistanceM
	}
	prev := points[i-1]
	curr := points[i]
	if curr.ElapsedEst <= prev.ElapsedEst {
		return curr.DistanceM
	}
	ratio := (elapsedS - prev.ElapsedEst) / (curr.ElapsedEst - prev.ElapsedEst)
	return prev.DistanceM + ratio*(curr.DistanceM-prev.DistanceM)
}

func durationBand(seconds float64) (float64, float64) {
	margin := seconds * 0.1
	return math.Max(0, seconds-margin), seconds + margin
}

func climbSeverityLevel(severity string) string {
	switch severity {
	case "major":
		return "high"
	case "selective":
		return "medium"
	default:
		return "low"
	}
}

func firstByDistance(points []PressurePoint, minKM, maxKM float64) *PressurePoint {
	for i := range points {
		if points[i].DistanceKM >= minKM && points[i].DistanceKM <= maxKM {
			return &points[i]
		}
	}
	return nil
}

func cleanLabel(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = strings.ReplaceAll(value, "_", " ")
	value = strings.ReplaceAll(value, "-", " ")
	value = strings.ToLower(value)
	return strings.Title(value)
}

func round(value float64, digits int) float64 {
	pow := math.Pow(10, float64(digits))
	return math.Round(value*pow) / pow
}

func safePositive(value float64) float64 {
	if !isFinite(value) || value <= 0 {
		return 0
	}
	return value
}

func clamp(value, low, high float64) float64 {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}

func isFinite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func isFinitePositiveOrZero(value float64) bool {
	return isFinite(value) && value >= 0
}

func hasValidGeo(point routePoint) bool {
	return isFinite(point.LatDeg) && isFinite(point.LonDeg)
}

func turnAngle(a, b, c routePoint) float64 {
	h1 := bearingDegrees(a.LatDeg, a.LonDeg, b.LatDeg, b.LonDeg)
	h2 := bearingDegrees(b.LatDeg, b.LonDeg, c.LatDeg, c.LonDeg)
	diff := math.Abs(h2 - h1)
	if diff > 180 {
		diff = 360 - diff
	}
	return diff
}

func bearingDegrees(lat1, lon1, lat2, lon2 float64) float64 {
	lat1 *= math.Pi / 180
	lon1 *= math.Pi / 180
	lat2 *= math.Pi / 180
	lon2 *= math.Pi / 180
	y := math.Sin(lon2-lon1) * math.Cos(lat2)
	x := math.Cos(lat1)*math.Sin(lat2) - math.Sin(lat1)*math.Cos(lat2)*math.Cos(lon2-lon1)
	theta := math.Atan2(y, x) * 180 / math.Pi
	if theta < 0 {
		theta += 360
	}
	return theta
}

func haversineMeters(lat1, lon1, lat2, lon2 float64) float64 {
	lat1 *= math.Pi / 180
	lon1 *= math.Pi / 180
	lat2 *= math.Pi / 180
	lon2 *= math.Pi / 180
	dLat := lat2 - lat1
	dLon := lon2 - lon1
	a := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(lat1)*math.Cos(lat2)*math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return earthRadiusM * c
}

func dedupeStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func formatDuration(seconds float64) string {
	if seconds <= 0 {
		return "0m"
	}
	d := time.Duration(seconds) * time.Second
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	if hours > 0 {
		return fmt.Sprintf("%dh%02dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
