package fitnotes

import (
	"fmt"
	"math"
	"os"
	"sort"
	"time"

	"github.com/tormoder/fit"
)

const (
	secondsPerHour = 3600.0
)

// Config controls optional calculations that require athlete-specific inputs.
type Config struct {
	FTPWatts float64
}

// Analysis contains extracted metrics and generated notes for a FIT activity.
type Analysis struct {
	FilePath          string           `json:"file_path"`
	Sport             string           `json:"sport"`
	SubSport          string           `json:"sub_sport"`
	StartTime         time.Time        `json:"start_time"`
	EndTime           time.Time        `json:"end_time"`
	ElapsedSeconds    float64          `json:"elapsed_seconds"`
	MovingSeconds     float64          `json:"moving_seconds"`
	DistanceMeters    float64          `json:"distance_meters"`
	ElevationGainM    float64          `json:"elevation_gain_m"`
	ElevationLossM    float64          `json:"elevation_loss_m"`
	Calories          int              `json:"calories"`
	AvgSpeedMps       float64          `json:"avg_speed_mps"`
	MaxSpeedMps       float64          `json:"max_speed_mps"`
	AvgPowerWatts     float64          `json:"avg_power_watts"`
	MaxPowerWatts     float64          `json:"max_power_watts"`
	NormalizedPower   float64          `json:"normalized_power_watts"`
	VariabilityIndex  float64          `json:"variability_index"`
	WorkKilojoules    float64          `json:"work_kilojoules"`
	AvgHeartRate      float64          `json:"avg_heart_rate_bpm"`
	MaxHeartRate      float64          `json:"max_heart_rate_bpm"`
	AvgCadence        float64          `json:"avg_cadence_rpm"`
	MaxCadence        float64          `json:"max_cadence_rpm"`
	FTPWatts          float64          `json:"ftp_watts"`
	FTPSource         string           `json:"ftp_source"`
	IntensityFactor   float64          `json:"intensity_factor"`
	TrainingStress    float64          `json:"training_stress_score"`
	Best20MinPower    float64          `json:"best_20min_power_watts"`
	PowerHRDecoupling float64          `json:"power_hr_decoupling_pct"`
	PowerZones        []ZoneDuration   `json:"power_zones,omitempty"`
	Laps              []LapSummary     `json:"laps,omitempty"`
	Intervals         IntervalSummary  `json:"intervals"`
	WorkoutStructure  WorkoutStructure `json:"workout_structure"`
	Notes             string           `json:"notes"`
}

// ZoneDuration stores duration spent in a given FTP-based power zone.
type ZoneDuration struct {
	Zone       string  `json:"zone"`
	MinPctFTP  float64 `json:"min_pct_ftp"`
	MaxPctFTP  float64 `json:"max_pct_ftp"`
	Seconds    float64 `json:"seconds"`
	Percentage float64 `json:"percentage"`
}

// LapSummary is a compact lap-level view for interval and pacing analysis.
type LapSummary struct {
	Index              int     `json:"index"`
	StartOffsetSeconds float64 `json:"start_offset_seconds"`
	EndOffsetSeconds   float64 `json:"end_offset_seconds"`
	DurationSeconds    float64 `json:"duration_seconds"`
	DistanceMeters     float64 `json:"distance_meters"`
	AvgPowerWatts      float64 `json:"avg_power_watts"`
	MaxPowerWatts      float64 `json:"max_power_watts"`
	AvgHeartRate       float64 `json:"avg_heart_rate_bpm"`
	AvgCadence         float64 `json:"avg_cadence_rpm"`
	Label              string  `json:"label"`
}

// IntervalSummary captures the detected interval structure of the workout.
type IntervalSummary struct {
	WorkCount                  int     `json:"work_count"`
	RecoveryCount              int     `json:"recovery_count"`
	ActivationCount            int     `json:"activation_count"`
	AvgWorkDurationSeconds     float64 `json:"avg_work_duration_seconds"`
	AvgRecoveryDurationSeconds float64 `json:"avg_recovery_duration_seconds"`
	AvgWorkPowerWatts          float64 `json:"avg_work_power_watts"`
	AvgRecoveryPowerWatts      float64 `json:"avg_recovery_power_watts"`
	WorkPowerChangePct         float64 `json:"work_power_change_pct"`
	WorkCadenceChangePct       float64 `json:"work_cadence_change_pct"`
	WorkHeartRateChange        float64 `json:"work_heart_rate_change_bpm"`
}

type recordSeries struct {
	start       time.Time
	end         time.Time
	durationSec float64

	powerSamples []float64
	powerForNP   []float64
	hrSamples    []float64
	cadSamples   []float64
	speedSamples []float64

	pairedPower []float64
	pairedHR    []float64

	lastDistanceMeters float64
	workKJ             float64
}

// AnalyzeFile decodes and analyzes an activity FIT file.
func AnalyzeFile(path string, cfg Config) (*Analysis, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open FIT file: %w", err)
	}
	defer f.Close()

	decoded, err := fit.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decode FIT file: %w", err)
	}

	activity, err := decoded.Activity()
	if err != nil {
		return nil, fmt.Errorf("activity FIT expected: %w", err)
	}
	if len(activity.Sessions) == 0 {
		return nil, fmt.Errorf("activity file has no session message")
	}

	series := buildRecordSeries(activity.Records)
	session := activity.Sessions[0]

	analysis := &Analysis{
		FilePath: path,
		Sport:    fmt.Sprint(session.Sport),
		SubSport: fmt.Sprint(session.SubSport),
	}

	analysis.StartTime = validTimeOrZero(session.StartTime)
	analysis.EndTime = validTimeOrZero(session.Timestamp)
	if analysis.StartTime.IsZero() {
		analysis.StartTime = series.start
	}
	if analysis.EndTime.IsZero() {
		analysis.EndTime = series.end
	}

	analysis.ElapsedSeconds = safePositive(session.GetTotalTimerTimeScaled())
	if analysis.ElapsedSeconds == 0 {
		analysis.ElapsedSeconds = series.durationSec
	}
	analysis.MovingSeconds = safePositive(session.GetTotalMovingTimeScaled())
	if analysis.MovingSeconds == 0 {
		analysis.MovingSeconds = analysis.ElapsedSeconds
	}
	analysis.DistanceMeters = safePositive(session.GetTotalDistanceScaled())
	if analysis.DistanceMeters == 0 {
		analysis.DistanceMeters = series.lastDistanceMeters
	}
	analysis.ElevationGainM = safePositive(float64(validUint16(session.TotalAscent)))
	analysis.ElevationLossM = safePositive(float64(validUint16(session.TotalDescent)))
	analysis.Calories = int(validUint16(session.TotalCalories))

	analysis.AvgSpeedMps = safePositive(session.GetEnhancedAvgSpeedScaled())
	if analysis.AvgSpeedMps == 0 {
		analysis.AvgSpeedMps = safePositive(session.GetAvgSpeedScaled())
	}
	if analysis.AvgSpeedMps == 0 && analysis.ElapsedSeconds > 0 {
		analysis.AvgSpeedMps = analysis.DistanceMeters / analysis.ElapsedSeconds
	}
	analysis.MaxSpeedMps = safePositive(session.GetEnhancedMaxSpeedScaled())
	if analysis.MaxSpeedMps == 0 {
		analysis.MaxSpeedMps = safePositive(session.GetMaxSpeedScaled())
	}
	if analysis.MaxSpeedMps == 0 {
		analysis.MaxSpeedMps = maxValue(series.speedSamples)
	}

	analysis.AvgPowerWatts = float64(validUint16(session.AvgPower))
	if analysis.AvgPowerWatts == 0 {
		analysis.AvgPowerWatts = average(series.powerSamples)
	}
	analysis.MaxPowerWatts = float64(validUint16(session.MaxPower))
	if analysis.MaxPowerWatts == 0 {
		analysis.MaxPowerWatts = maxValue(series.powerSamples)
	}

	analysis.NormalizedPower = float64(validUint16(session.NormalizedPower))
	if analysis.NormalizedPower == 0 {
		analysis.NormalizedPower = normalizedPower(series.powerForNP)
	}
	if analysis.NormalizedPower == 0 {
		analysis.NormalizedPower = analysis.AvgPowerWatts
	}

	analysis.WorkKilojoules = float64(validUint32(session.TotalWork)) / 1000.0
	if analysis.WorkKilojoules == 0 {
		analysis.WorkKilojoules = series.workKJ
	}
	if analysis.WorkKilojoules == 0 && analysis.AvgPowerWatts > 0 && analysis.ElapsedSeconds > 0 {
		analysis.WorkKilojoules = analysis.AvgPowerWatts * analysis.ElapsedSeconds / 1000.0
	}

	analysis.AvgHeartRate = float64(validUint8(session.AvgHeartRate))
	if analysis.AvgHeartRate == 0 {
		analysis.AvgHeartRate = average(series.hrSamples)
	}
	analysis.MaxHeartRate = float64(validUint8(session.MaxHeartRate))
	if analysis.MaxHeartRate == 0 {
		analysis.MaxHeartRate = maxValue(series.hrSamples)
	}

	analysis.AvgCadence = cadenceFromAny(session.GetAvgCadence())
	if analysis.AvgCadence == 0 {
		analysis.AvgCadence = average(series.cadSamples)
	}
	analysis.MaxCadence = cadenceFromAny(session.GetMaxCadence())
	if analysis.MaxCadence == 0 {
		analysis.MaxCadence = maxValue(series.cadSamples)
	}

	analysis.Best20MinPower = bestRollingPower(series.powerForNP, 20*60)
	analysis.FTPWatts = safePositive(cfg.FTPWatts)
	if analysis.FTPWatts > 0 {
		analysis.FTPSource = "input"
	} else {
		estimated := estimateFTP(series.powerForNP)
		if estimated > 0 {
			analysis.FTPWatts = estimated
			analysis.FTPSource = "estimated"
		} else {
			analysis.FTPSource = "unavailable"
		}
	}

	if analysis.AvgPowerWatts > 0 {
		analysis.VariabilityIndex = analysis.NormalizedPower / analysis.AvgPowerWatts
	}
	if analysis.FTPWatts > 0 && analysis.NormalizedPower > 0 {
		analysis.IntensityFactor = analysis.NormalizedPower / analysis.FTPWatts
	}
	if analysis.ElapsedSeconds > 0 && analysis.IntensityFactor > 0 {
		analysis.TrainingStress = (analysis.ElapsedSeconds / secondsPerHour) * analysis.IntensityFactor * analysis.IntensityFactor * 100.0
	}

	analysis.PowerHRDecoupling = powerHRDecoupling(series.pairedPower, series.pairedHR)
	analysis.PowerZones = buildPowerZones(series.powerForNP, analysis.FTPWatts)
	analysis.Laps, analysis.Intervals = summarizeLaps(activity.Laps, analysis.AvgPowerWatts)
	analysis.WorkoutStructure = InferWorkoutStructure(analysis.Laps, analysis.FTPWatts, analysis.Intervals)
	analysis.Notes = BuildTrainingNotes(analysis)

	return analysis, nil
}

func buildRecordSeries(records []*fit.RecordMsg) recordSeries {
	rs := recordSeries{}
	if len(records) == 0 {
		return rs
	}

	type row struct {
		ts time.Time
		r  *fit.RecordMsg
	}

	rows := make([]row, 0, len(records))
	for _, rec := range records {
		if rec == nil {
			continue
		}
		rows = append(rows, row{ts: rec.Timestamp, r: rec})
	}

	sort.Slice(rows, func(i, j int) bool {
		return rows[i].ts.Before(rows[j].ts)
	})

	var (
		haveStart    bool
		lastTS       time.Time
		haveLastTS   bool
		lastPower    float64
		haveLastPwr  bool
		workJoules   float64
		lastDistance float64
	)

	for _, entry := range rows {
		rec := entry.r
		ts := validTimeOrZero(rec.Timestamp)
		if !ts.IsZero() {
			if !haveStart {
				rs.start = ts
				haveStart = true
			}
			rs.end = ts
		}

		power, hasPower := extractPower(rec)
		hr, hasHR := extractHeartRate(rec)
		cadence, hasCadence := extractCadence(rec)
		speed, hasSpeed := extractSpeed(rec)

		if hasPower {
			rs.powerSamples = append(rs.powerSamples, power)
		}
		if hasHR {
			rs.hrSamples = append(rs.hrSamples, hr)
		}
		if hasCadence {
			rs.cadSamples = append(rs.cadSamples, cadence)
		}
		if hasSpeed {
			rs.speedSamples = append(rs.speedSamples, speed)
		}
		if hasPower && hasHR && hr > 0 {
			rs.pairedPower = append(rs.pairedPower, power)
			rs.pairedHR = append(rs.pairedHR, hr)
		}

		distance := safePositive(rec.GetDistanceScaled())
		if distance > 0 {
			lastDistance = distance
		}

		if hasPower {
			if haveLastTS && !ts.IsZero() && ts.After(lastTS) && haveLastPwr {
				delta := ts.Sub(lastTS).Seconds()
				if delta > 0 && delta <= 5 {
					workJoules += lastPower * delta
				}

				missing := int(math.Round(delta)) - 1
				if missing > 0 && missing <= 30 {
					for i := 0; i < missing; i++ {
						rs.powerForNP = append(rs.powerForNP, lastPower)
					}
				}
			}
			rs.powerForNP = append(rs.powerForNP, power)
			lastPower = power
			haveLastPwr = true
		}

		if !ts.IsZero() {
			lastTS = ts
			haveLastTS = true
		}
	}

	rs.lastDistanceMeters = lastDistance
	if !rs.start.IsZero() && !rs.end.IsZero() && rs.end.After(rs.start) {
		rs.durationSec = rs.end.Sub(rs.start).Seconds()
	}
	if workJoules == 0 && len(rs.powerSamples) > 0 {
		for _, p := range rs.powerSamples {
			workJoules += p
		}
	}
	rs.workKJ = workJoules / 1000.0

	return rs
}

func summarizeLaps(laps []*fit.LapMsg, sessionAvgPower float64) ([]LapSummary, IntervalSummary) {
	if len(laps) == 0 {
		return nil, IntervalSummary{}
	}

	summaries := make([]LapSummary, 0, len(laps))
	lapPowers := make([]float64, 0, len(laps))
	offset := 0.0
	for idx, lap := range laps {
		if lap == nil {
			continue
		}
		duration := safePositive(lap.GetTotalTimerTimeScaled())
		if duration == 0 {
			duration = safePositive(lap.GetTotalElapsedTimeScaled())
		}

		avgPower := float64(validUint16(lap.AvgPower))
		if avgPower > 0 {
			lapPowers = append(lapPowers, avgPower)
		}

		summaries = append(summaries, LapSummary{
			Index:              idx + 1,
			StartOffsetSeconds: offset,
			EndOffsetSeconds:   offset + duration,
			DurationSeconds:    duration,
			DistanceMeters:     safePositive(lap.GetTotalDistanceScaled()),
			AvgPowerWatts:      avgPower,
			MaxPowerWatts:      float64(validUint16(lap.MaxPower)),
			AvgHeartRate:       float64(validUint8(lap.AvgHeartRate)),
			AvgCadence:         cadenceFromAny(lap.GetAvgCadence()),
			Label:              "steady",
		})
		offset += duration
	}
	if len(summaries) == 0 {
		return summaries, IntervalSummary{}
	}

	baselinePower := sessionAvgPower
	if baselinePower <= 0 {
		baselinePower = average(lapPowers)
	}
	if baselinePower <= 0 {
		baselinePower = 150
	}
	hardThreshold := baselinePower * 1.20
	easyThreshold := baselinePower * 0.90

	workIndices := make([]int, 0)
	recoveryIndices := make([]int, 0)
	activationCount := 0

	for i := range summaries {
		lap := &summaries[i]
		if lap.AvgPowerWatts <= 0 || lap.DurationSeconds <= 0 {
			continue
		}
		if lap.AvgPowerWatts >= hardThreshold {
			if lap.DurationSeconds < 90 {
				lap.Label = "activation"
				activationCount++
			} else {
				lap.Label = "work"
				workIndices = append(workIndices, i)
			}
			continue
		}
		if lap.DurationSeconds >= 60 && lap.AvgPowerWatts <= easyThreshold {
			lap.Label = "easy"
		}
	}

	seenRecovery := make(map[int]struct{})
	for _, wi := range workIndices {
		next := wi + 1
		if next >= len(summaries) {
			continue
		}
		candidate := &summaries[next]
		if candidate.DurationSeconds >= 60 && candidate.AvgPowerWatts > 0 && candidate.AvgPowerWatts <= easyThreshold {
			candidate.Label = "recovery"
			if _, exists := seenRecovery[next]; !exists {
				seenRecovery[next] = struct{}{}
				recoveryIndices = append(recoveryIndices, next)
			}
		}
	}

	if len(workIndices) > 0 {
		firstWork := workIndices[0]
		lastWork := workIndices[len(workIndices)-1]
		for i := 0; i < firstWork; i++ {
			if summaries[i].Label == "easy" || i == 0 {
				summaries[i].Label = "warmup"
			}
		}
		for i := lastWork + 1; i < len(summaries); i++ {
			if summaries[i].Label == "recovery" {
				continue
			}
			if summaries[i].Label == "easy" || summaries[i].AvgPowerWatts <= easyThreshold {
				summaries[i].Label = "cooldown"
			}
		}
	}

	intervals := IntervalSummary{
		WorkCount:       len(workIndices),
		RecoveryCount:   len(recoveryIndices),
		ActivationCount: activationCount,
	}

	workPowers := make([]float64, 0, len(workIndices))
	workDurations := make([]float64, 0, len(workIndices))
	workCadences := make([]float64, 0, len(workIndices))
	workHR := make([]float64, 0, len(workIndices))
	for _, idx := range workIndices {
		workPowers = append(workPowers, summaries[idx].AvgPowerWatts)
		workDurations = append(workDurations, summaries[idx].DurationSeconds)
		if summaries[idx].AvgCadence > 0 {
			workCadences = append(workCadences, summaries[idx].AvgCadence)
		}
		if summaries[idx].AvgHeartRate > 0 {
			workHR = append(workHR, summaries[idx].AvgHeartRate)
		}
	}

	recoveryPowers := make([]float64, 0, len(recoveryIndices))
	recoveryDurations := make([]float64, 0, len(recoveryIndices))
	for _, idx := range recoveryIndices {
		recoveryPowers = append(recoveryPowers, summaries[idx].AvgPowerWatts)
		recoveryDurations = append(recoveryDurations, summaries[idx].DurationSeconds)
	}

	intervals.AvgWorkPowerWatts = average(workPowers)
	intervals.AvgWorkDurationSeconds = average(workDurations)
	intervals.AvgRecoveryPowerWatts = average(recoveryPowers)
	intervals.AvgRecoveryDurationSeconds = average(recoveryDurations)
	intervals.WorkPowerChangePct = pctChange(firstValue(workPowers), lastValue(workPowers))
	intervals.WorkCadenceChangePct = pctChange(firstValue(workCadences), lastValue(workCadences))
	if len(workHR) >= 2 {
		intervals.WorkHeartRateChange = lastValue(workHR) - firstValue(workHR)
	}

	return summaries, intervals
}

func buildPowerZones(powerSamples []float64, ftp float64) []ZoneDuration {
	if ftp <= 0 || len(powerSamples) == 0 {
		return nil
	}

	type boundary struct {
		zone string
		min  float64
		max  float64
	}
	zones := []boundary{
		{zone: "Z1 Active Recovery", min: 0, max: 55},
		{zone: "Z2 Endurance", min: 55, max: 75},
		{zone: "Z3 Tempo", min: 75, max: 90},
		{zone: "Z4 Threshold", min: 90, max: 105},
		{zone: "Z5 VO2", min: 105, max: 120},
		{zone: "Z6 Anaerobic", min: 120, max: 150},
		{zone: "Z7 Neuromuscular", min: 150, max: 1000},
	}

	counts := make([]int, len(zones))
	total := 0
	for _, p := range powerSamples {
		if p < 0 {
			continue
		}
		percent := (p / ftp) * 100.0
		for i, z := range zones {
			if percent >= z.min && percent < z.max {
				counts[i]++
				total++
				break
			}
		}
	}
	if total == 0 {
		return nil
	}

	out := make([]ZoneDuration, 0, len(zones))
	for i, z := range zones {
		seconds := float64(counts[i])
		out = append(out, ZoneDuration{
			Zone:       z.zone,
			MinPctFTP:  z.min,
			MaxPctFTP:  z.max,
			Seconds:    seconds,
			Percentage: (seconds / float64(total)) * 100.0,
		})
	}
	return out
}

func normalizedPower(powerSamples []float64) float64 {
	if len(powerSamples) == 0 {
		return 0
	}
	if len(powerSamples) < 30 {
		return average(powerSamples)
	}

	window := 30
	sum := 0.0
	for i := 0; i < window; i++ {
		sum += powerSamples[i]
	}

	fourthPowerTotal := 0.0
	count := 0
	for i := window - 1; i < len(powerSamples); i++ {
		if i >= window {
			sum += powerSamples[i] - powerSamples[i-window]
		}
		rolling := sum / float64(window)
		fourthPowerTotal += math.Pow(rolling, 4)
		count++
	}
	if count == 0 {
		return average(powerSamples)
	}
	return math.Pow(fourthPowerTotal/float64(count), 0.25)
}

func estimateFTP(powerSamples []float64) float64 {
	best20 := bestRollingPower(powerSamples, 20*60)
	if best20 <= 0 {
		return 0
	}
	return best20 * 0.95
}

func bestRollingPower(powerSamples []float64, seconds int) float64 {
	if len(powerSamples) == 0 || seconds <= 0 {
		return 0
	}
	if len(powerSamples) < seconds {
		return average(powerSamples)
	}

	sum := 0.0
	for i := 0; i < seconds; i++ {
		sum += powerSamples[i]
	}
	best := sum / float64(seconds)
	for i := seconds; i < len(powerSamples); i++ {
		sum += powerSamples[i] - powerSamples[i-seconds]
		current := sum / float64(seconds)
		if current > best {
			best = current
		}
	}
	return best
}

func powerHRDecoupling(power, hr []float64) float64 {
	n := len(power)
	if n == 0 || n != len(hr) || n < 20 {
		return 0
	}
	mid := n / 2

	p1, h1 := average(power[:mid]), average(hr[:mid])
	p2, h2 := average(power[mid:]), average(hr[mid:])
	if p1 == 0 || p2 == 0 || h1 == 0 || h2 == 0 {
		return 0
	}

	firstRatio := p1 / h1
	secondRatio := p2 / h2
	if firstRatio == 0 {
		return 0
	}
	return ((secondRatio / firstRatio) - 1.0) * 100.0
}

func extractPower(rec *fit.RecordMsg) (float64, bool) {
	if rec.Power == math.MaxUint16 {
		return 0, false
	}
	return float64(rec.Power), true
}

func extractHeartRate(rec *fit.RecordMsg) (float64, bool) {
	if rec.HeartRate == math.MaxUint8 {
		return 0, false
	}
	return float64(rec.HeartRate), true
}

func extractCadence(rec *fit.RecordMsg) (float64, bool) {
	cad256 := safePositive(rec.GetCadence256Scaled())
	if cad256 > 0 {
		return cad256, true
	}
	if rec.Cadence == math.MaxUint8 {
		return 0, false
	}
	return float64(rec.Cadence), true
}

func extractSpeed(rec *fit.RecordMsg) (float64, bool) {
	speed := rec.GetEnhancedSpeedScaled()
	if isFinite(speed) && speed >= 0 {
		return speed, true
	}
	speed = rec.GetSpeedScaled()
	if isFinite(speed) && speed >= 0 {
		return speed, true
	}
	return 0, false
}

func validTimeOrZero(t time.Time) time.Time {
	if t.IsZero() || fit.IsBaseTime(t) {
		return time.Time{}
	}
	return t
}

func validUint8(v uint8) uint8 {
	if v == math.MaxUint8 {
		return 0
	}
	return v
}

func validUint16(v uint16) uint16 {
	if v == math.MaxUint16 {
		return 0
	}
	return v
}

func validUint32(v uint32) uint32 {
	if v == math.MaxUint32 {
		return 0
	}
	return v
}

func cadenceFromAny(v any) float64 {
	switch x := v.(type) {
	case uint8:
		if x == math.MaxUint8 {
			return 0
		}
		return float64(x)
	case uint16:
		if x == math.MaxUint16 {
			return 0
		}
		return float64(x)
	case int:
		if x < 0 {
			return 0
		}
		return float64(x)
	case float64:
		return safePositive(x)
	default:
		return 0
	}
}

func average(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	total := 0.0
	count := 0
	for _, v := range values {
		if !isFinite(v) {
			continue
		}
		total += v
		count++
	}
	if count == 0 {
		return 0
	}
	return total / float64(count)
}

func maxValue(values []float64) float64 {
	max := 0.0
	found := false
	for _, v := range values {
		if !isFinite(v) {
			continue
		}
		if !found || v > max {
			max = v
			found = true
		}
	}
	if !found {
		return 0
	}
	return max
}

func pctChange(start, end float64) float64 {
	if start == 0 {
		return 0
	}
	return ((end / start) - 1.0) * 100.0
}

func firstValue(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	return values[0]
}

func lastValue(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	return values[len(values)-1]
}

func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

func safePositive(v float64) float64 {
	if !isFinite(v) || v <= 0 {
		return 0
	}
	return v
}
