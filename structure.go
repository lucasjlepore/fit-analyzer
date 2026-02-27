package fitnotes

import (
	"fmt"
	"math"
	"strings"
)

const workoutStructureSchemaVersion = "workout_structure_v1"

// WorkoutStructure is an LLM-oriented semantic view of the session.
type WorkoutStructure struct {
	SchemaVersion  string          `json:"schema_version"`
	Confidence     float64         `json:"confidence"`
	CanonicalLabel string          `json:"canonical_label"`
	Blocks         []WorkoutBlock  `json:"blocks,omitempty"`
	Openers        *OpenersSummary `json:"openers,omitempty"`
	MainSet        *MainSetSummary `json:"main_set,omitempty"`
}

// WorkoutBlock represents one contiguous session block.
type WorkoutBlock struct {
	BlockType          string  `json:"block_type"`
	StartLap           int     `json:"start_lap"`
	EndLap             int     `json:"end_lap"`
	StartOffsetSeconds float64 `json:"start_offset_seconds"`
	EndOffsetSeconds   float64 `json:"end_offset_seconds"`
	DurationSeconds    float64 `json:"duration_seconds"`
	AvgPowerWatts      float64 `json:"avg_power_watts"`
	AvgHeartRate       float64 `json:"avg_heart_rate_bpm"`
	AvgCadence         float64 `json:"avg_cadence_rpm"`
	Description        string  `json:"description"`
}

// OpenersSummary captures short pre-main-set opener efforts.
type OpenersSummary struct {
	Reps               int     `json:"reps"`
	OnDurationSeconds  float64 `json:"on_duration_seconds"`
	OffDurationSeconds float64 `json:"off_duration_seconds"`
	OnPowerWatts       float64 `json:"on_power_watts"`
	OffPowerWatts      float64 `json:"off_power_watts"`
}

// MainSetSummary captures the primary interval set.
type MainSetSummary struct {
	Reps                    int          `json:"reps"`
	WorkDurationSeconds     float64      `json:"work_duration_seconds"`
	RecoveryDurationSeconds float64      `json:"recovery_duration_seconds"`
	WorkPowerWatts          float64      `json:"work_power_watts"`
	RecoveryPowerWatts      float64      `json:"recovery_power_watts"`
	WorkTargetWatts         float64      `json:"work_target_watts"`
	RecoveryTargetWatts     float64      `json:"recovery_target_watts"`
	WorkPctFTP              float64      `json:"work_pct_ftp"`
	RecoveryPctFTP          float64      `json:"recovery_pct_ftp"`
	PowerDriftPct           float64      `json:"power_drift_pct"`
	CadenceDriftPct         float64      `json:"cadence_drift_pct"`
	HeartRateDriftBPM       float64      `json:"heart_rate_drift_bpm"`
	Prescription            string       `json:"prescription"`
	RepsDetail              []MainSetRep `json:"reps_detail,omitempty"`
}

// MainSetRep stores rep-level execution metrics.
type MainSetRep struct {
	Rep                     int     `json:"rep"`
	WorkLap                 int     `json:"work_lap"`
	RecoveryLap             int     `json:"recovery_lap,omitempty"`
	WorkDurationSeconds     float64 `json:"work_duration_seconds"`
	RecoveryDurationSeconds float64 `json:"recovery_duration_seconds,omitempty"`
	WorkPowerWatts          float64 `json:"work_power_watts"`
	RecoveryPowerWatts      float64 `json:"recovery_power_watts,omitempty"`
	WorkPctFTP              float64 `json:"work_pct_ftp,omitempty"`
	RecoveryPctFTP          float64 `json:"recovery_pct_ftp,omitempty"`
	WorkVsTargetPct         float64 `json:"work_vs_target_pct,omitempty"`
	RecoveryVsTargetPct     float64 `json:"recovery_vs_target_pct,omitempty"`
}

// InferWorkoutStructure converts lap-level labels into explicit workout blocks and prescriptions.
func InferWorkoutStructure(laps []LapSummary, ftp float64, intervals IntervalSummary) WorkoutStructure {
	ws := WorkoutStructure{
		SchemaVersion: workoutStructureSchemaVersion,
		Confidence:    0.25,
	}
	if len(laps) == 0 {
		ws.CanonicalLabel = "unable to infer workout structure (no lap data)"
		return ws
	}

	mainStart, mainEnd := detectMainSetWindow(laps)
	openerStart, openerEnd, openers := detectOpenersWindow(laps, mainStart, intervals)

	used := make([]bool, len(laps))
	addBlock := func(blockType string, start, end int, desc string) {
		if start < 0 || end < start || start >= len(laps) {
			return
		}
		if end >= len(laps) {
			end = len(laps) - 1
		}
		block := buildBlock(laps, blockType, start, end, desc)
		ws.Blocks = append(ws.Blocks, block)
		for i := start; i <= end; i++ {
			used[i] = true
		}
	}

	if mainStart > 0 {
		warmupEnd := mainStart - 1
		if openerStart > 0 {
			warmupEnd = openerStart - 1
		}
		if warmupEnd >= 0 {
			addBlock("warmup", 0, warmupEnd, "Aerobic warmup before intensity")
			ws.Confidence += 0.08
		}
	}

	if openers.Reps >= 2 && openerStart >= 0 && openerEnd >= openerStart {
		ws.Openers = &openers
		addBlock(
			"openers",
			openerStart,
			openerEnd,
			fmt.Sprintf("%dx%s on/%s easy primer efforts", openers.Reps, shortDuration(openers.OnDurationSeconds), shortDuration(openers.OffDurationSeconds)),
		)
		ws.Confidence += 0.16
	}

	if mainStart >= 0 {
		mainSummary := buildMainSetSummary(laps, mainStart, mainEnd, ftp, intervals)
		ws.MainSet = &mainSummary
		addBlock("main_set", mainStart, mainEnd, mainSummary.Prescription)
		ws.Confidence += 0.36
		if mainSummary.Reps >= 4 {
			ws.Confidence += 0.08
		}
	}

	cooldownStart, cooldownEnd := detectCooldownWindow(laps, mainEnd)
	if cooldownStart >= 0 && cooldownEnd >= cooldownStart {
		addBlock("cooldown", cooldownStart, cooldownEnd, "Easy cooldown to finish the session")
		ws.Confidence += 0.08
	}

	// Keep all laps represented; remaining unlabeled chunks become "steady" blocks.
	i := 0
	for i < len(laps) {
		if used[i] {
			i++
			continue
		}
		j := i
		for j+1 < len(laps) && !used[j+1] {
			j++
		}
		addBlock("steady", i, j, "Unclassified steady riding block")
		i = j + 1
	}

	if len(ws.Blocks) >= 3 {
		ws.Confidence += 0.05
	}
	if ws.Confidence > 0.99 {
		ws.Confidence = 0.99
	}

	ws.CanonicalLabel = buildCanonicalStructureLabel(ws)
	return ws
}

func detectMainSetWindow(laps []LapSummary) (int, int) {
	workIdx := make([]int, 0)
	for i, lap := range laps {
		if lap.Label == "work" {
			workIdx = append(workIdx, i)
		}
	}
	if len(workIdx) == 0 {
		return -1, -1
	}
	start := workIdx[0]
	end := workIdx[len(workIdx)-1]
	if end+1 < len(laps) && laps[end+1].Label == "recovery" {
		end++
	}
	return start, end
}

func detectOpenersWindow(laps []LapSummary, mainStart int, intervals IntervalSummary) (int, int, OpenersSummary) {
	if mainStart <= 1 {
		return -1, -1, OpenersSummary{}
	}

	shortEffortMax := 75.0
	onDur := make([]float64, 0)
	offDur := make([]float64, 0)
	onPow := make([]float64, 0)
	offPow := make([]float64, 0)

	first := -1
	last := -1
	reps := 0

	for i := 0; i+1 < mainStart; i++ {
		on := laps[i]
		off := laps[i+1]

		isOn := on.Label == "activation"
		if !isOn && intervals.AvgWorkPowerWatts > 0 {
			isOn = on.DurationSeconds <= shortEffortMax && on.AvgPowerWatts >= intervals.AvgWorkPowerWatts*0.90
		}
		isOff := off.DurationSeconds <= shortEffortMax && off.AvgPowerWatts > 0 && off.AvgPowerWatts < on.AvgPowerWatts*0.80

		if isOn && isOff {
			if first < 0 {
				first = i
			}
			last = i + 1
			reps++
			onDur = append(onDur, on.DurationSeconds)
			offDur = append(offDur, off.DurationSeconds)
			onPow = append(onPow, on.AvgPowerWatts)
			offPow = append(offPow, off.AvgPowerWatts)
			i++
		}
	}
	if reps < 2 {
		return -1, -1, OpenersSummary{}
	}

	return first, last, OpenersSummary{
		Reps:               reps,
		OnDurationSeconds:  average(onDur),
		OffDurationSeconds: average(offDur),
		OnPowerWatts:       average(onPow),
		OffPowerWatts:      average(offPow),
	}
}

func detectCooldownWindow(laps []LapSummary, mainEnd int) (int, int) {
	start := -1
	searchFrom := 0
	if mainEnd >= 0 {
		searchFrom = mainEnd + 1
	}
	for i := searchFrom; i < len(laps); i++ {
		if laps[i].Label == "cooldown" {
			start = i
			break
		}
	}
	if start < 0 {
		return -1, -1
	}

	end := start
	for i := start + 1; i < len(laps); i++ {
		if laps[i].Label == "cooldown" || laps[i].Label == "easy" {
			end = i
			continue
		}
		break
	}
	return start, end
}

func buildMainSetSummary(laps []LapSummary, start, end int, ftp float64, intervals IntervalSummary) MainSetSummary {
	workIdx := make([]int, 0)
	recoveryIdx := make([]int, 0)
	for i := start; i <= end && i < len(laps); i++ {
		switch laps[i].Label {
		case "work":
			workIdx = append(workIdx, i)
		case "recovery":
			recoveryIdx = append(recoveryIdx, i)
		}
	}

	workDur := make([]float64, 0, len(workIdx))
	workPow := make([]float64, 0, len(workIdx))
	workCad := make([]float64, 0, len(workIdx))
	workHR := make([]float64, 0, len(workIdx))
	for _, idx := range workIdx {
		workDur = append(workDur, laps[idx].DurationSeconds)
		workPow = append(workPow, laps[idx].AvgPowerWatts)
		if laps[idx].AvgCadence > 0 {
			workCad = append(workCad, laps[idx].AvgCadence)
		}
		if laps[idx].AvgHeartRate > 0 {
			workHR = append(workHR, laps[idx].AvgHeartRate)
		}
	}

	recoveryDur := make([]float64, 0, len(recoveryIdx))
	recoveryPow := make([]float64, 0, len(recoveryIdx))
	for _, idx := range recoveryIdx {
		recoveryDur = append(recoveryDur, laps[idx].DurationSeconds)
		recoveryPow = append(recoveryPow, laps[idx].AvgPowerWatts)
	}

	workAvgDur := average(workDur)
	recoveryAvgDur := average(recoveryDur)
	workAvgPow := average(workPow)
	recoveryAvgPow := average(recoveryPow)

	if workAvgDur == 0 {
		workAvgDur = intervals.AvgWorkDurationSeconds
	}
	if recoveryAvgDur == 0 {
		recoveryAvgDur = intervals.AvgRecoveryDurationSeconds
	}
	if workAvgPow == 0 {
		workAvgPow = intervals.AvgWorkPowerWatts
	}
	if recoveryAvgPow == 0 {
		recoveryAvgPow = intervals.AvgRecoveryPowerWatts
	}

	workTarget := roundToNearest(workAvgPow, 5)
	recoveryTarget := roundToNearest(recoveryAvgPow, 5)
	summary := MainSetSummary{
		Reps:                    len(workIdx),
		WorkDurationSeconds:     workAvgDur,
		RecoveryDurationSeconds: recoveryAvgDur,
		WorkPowerWatts:          workAvgPow,
		RecoveryPowerWatts:      recoveryAvgPow,
		WorkTargetWatts:         workTarget,
		RecoveryTargetWatts:     recoveryTarget,
		PowerDriftPct:           intervals.WorkPowerChangePct,
		CadenceDriftPct:         intervals.WorkCadenceChangePct,
		HeartRateDriftBPM:       intervals.WorkHeartRateChange,
	}
	if ftp > 0 {
		summary.WorkPctFTP = (workAvgPow / ftp) * 100.0
		summary.RecoveryPctFTP = (recoveryAvgPow / ftp) * 100.0
	}
	summary.Prescription = fmt.Sprintf(
		"%dx%s @%.0fW with %s @%.0fW recoveries",
		summary.Reps,
		shortDuration(summary.WorkDurationSeconds),
		summary.WorkTargetWatts,
		shortDuration(summary.RecoveryDurationSeconds),
		summary.RecoveryTargetWatts,
	)

	reps := make([]MainSetRep, 0, len(workIdx))
	for i, w := range workIdx {
		rep := MainSetRep{
			Rep:                 i + 1,
			WorkLap:             laps[w].Index,
			WorkDurationSeconds: laps[w].DurationSeconds,
			WorkPowerWatts:      laps[w].AvgPowerWatts,
		}
		if ftp > 0 {
			rep.WorkPctFTP = (rep.WorkPowerWatts / ftp) * 100.0
		}
		if workTarget > 0 {
			rep.WorkVsTargetPct = ((rep.WorkPowerWatts / workTarget) - 1) * 100
		}

		nextWork := len(laps)
		if i+1 < len(workIdx) {
			nextWork = workIdx[i+1]
		}
		for _, r := range recoveryIdx {
			if r > w && r < nextWork {
				rep.RecoveryLap = laps[r].Index
				rep.RecoveryDurationSeconds = laps[r].DurationSeconds
				rep.RecoveryPowerWatts = laps[r].AvgPowerWatts
				if ftp > 0 {
					rep.RecoveryPctFTP = (rep.RecoveryPowerWatts / ftp) * 100.0
				}
				if recoveryTarget > 0 {
					rep.RecoveryVsTargetPct = ((rep.RecoveryPowerWatts / recoveryTarget) - 1) * 100
				}
				break
			}
		}
		reps = append(reps, rep)
	}
	summary.RepsDetail = reps
	return summary
}

func buildCanonicalStructureLabel(ws WorkoutStructure) string {
	if len(ws.Blocks) == 0 {
		return "unclassified session structure"
	}
	parts := make([]string, 0, 4)
	for _, b := range ws.Blocks {
		switch b.BlockType {
		case "warmup":
			parts = append(parts, fmt.Sprintf("warmup %s", shortDuration(b.DurationSeconds)))
		case "openers":
			if ws.Openers != nil {
				parts = append(parts, fmt.Sprintf("openers %dx%s/%s", ws.Openers.Reps, shortDuration(ws.Openers.OnDurationSeconds), shortDuration(ws.Openers.OffDurationSeconds)))
			}
		case "main_set":
			if ws.MainSet != nil {
				if ws.MainSet.WorkPctFTP > 0 {
					parts = append(parts, fmt.Sprintf("%s (%.0f%% FTP)", ws.MainSet.Prescription, ws.MainSet.WorkPctFTP))
				} else {
					parts = append(parts, ws.MainSet.Prescription)
				}
			}
		case "cooldown":
			parts = append(parts, fmt.Sprintf("cooldown %s", shortDuration(b.DurationSeconds)))
		}
	}
	if len(parts) == 0 {
		return "unclassified session structure"
	}
	return strings.Join(parts, " + ")
}

func buildBlock(laps []LapSummary, blockType string, start, end int, description string) WorkoutBlock {
	startOffset := laps[start].StartOffsetSeconds
	endOffset := laps[end].EndOffsetSeconds

	dur := 0.0
	sumP := 0.0
	sumHR := 0.0
	sumCad := 0.0
	weightP := 0.0
	weightHR := 0.0
	weightCad := 0.0
	for i := start; i <= end && i < len(laps); i++ {
		l := laps[i]
		d := l.DurationSeconds
		dur += d
		if l.AvgPowerWatts > 0 {
			sumP += l.AvgPowerWatts * d
			weightP += d
		}
		if l.AvgHeartRate > 0 {
			sumHR += l.AvgHeartRate * d
			weightHR += d
		}
		if l.AvgCadence > 0 {
			sumCad += l.AvgCadence * d
			weightCad += d
		}
	}

	return WorkoutBlock{
		BlockType:          blockType,
		StartLap:           laps[start].Index,
		EndLap:             laps[end].Index,
		StartOffsetSeconds: startOffset,
		EndOffsetSeconds:   endOffset,
		DurationSeconds:    dur,
		AvgPowerWatts:      safeDiv(sumP, weightP),
		AvgHeartRate:       safeDiv(sumHR, weightHR),
		AvgCadence:         safeDiv(sumCad, weightCad),
		Description:        description,
	}
}

func shortDuration(seconds float64) string {
	s := int(math.Round(seconds))
	if s <= 0 {
		return "0s"
	}
	if s%60 == 0 {
		return fmt.Sprintf("%dm", s/60)
	}
	if s < 60 {
		return fmt.Sprintf("%ds", s)
	}
	return fmt.Sprintf("%dm%02ds", s/60, s%60)
}

func roundToNearest(v, step float64) float64 {
	if v == 0 || step <= 0 {
		return v
	}
	return math.Round(v/step) * step
}

func safeDiv(num, den float64) float64 {
	if den <= 0 {
		return 0
	}
	return num / den
}
