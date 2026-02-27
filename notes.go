package fitnotes

import (
	"fmt"
	"math"
	"strings"
)

// BuildTrainingNotes turns extracted metrics into a detailed training summary.
func BuildTrainingNotes(a *Analysis) string {
	if a == nil {
		return ""
	}

	var b strings.Builder

	fmt.Fprintf(
		&b,
		"Session: %s (%s)\n",
		a.Sport,
		a.SubSport,
	)
	if !a.StartTime.IsZero() {
		fmt.Fprintf(&b, "Start: %s\n", a.StartTime.Format("2006-01-02 15:04:05"))
	}
	fmt.Fprintf(
		&b,
		"Duration %s | Distance %.1f km | Elevation +%.0f/-%0.f m\n",
		formatDuration(a.ElapsedSeconds),
		a.DistanceMeters/1000.0,
		a.ElevationGainM,
		a.ElevationLossM,
	)

	fmt.Fprintf(
		&b,
		"Power %.0f avg / %.0f NP / %.0f max W | Work %.0f kJ | VI %.2f\n",
		a.AvgPowerWatts,
		a.NormalizedPower,
		a.MaxPowerWatts,
		a.WorkKilojoules,
		a.VariabilityIndex,
	)
	fmt.Fprintf(
		&b,
		"HR %.0f avg / %.0f max bpm | Cadence %.0f avg / %.0f max rpm | Speed %.1f avg / %.1f max km/h\n",
		a.AvgHeartRate,
		a.MaxHeartRate,
		a.AvgCadence,
		a.MaxCadence,
		mpsToKmh(a.AvgSpeedMps),
		mpsToKmh(a.MaxSpeedMps),
	)

	if a.FTPWatts > 0 {
		fmt.Fprintf(
			&b,
			"Load IF %.2f | TSS %.0f | FTP %.0f W (%s)\n",
			a.IntensityFactor,
			a.TrainingStress,
			a.FTPWatts,
			a.FTPSource,
		)
	} else {
		fmt.Fprintf(&b, "Load IF/TSS unavailable (FTP not provided and could not be estimated)\n")
	}
	if a.Best20MinPower > 0 {
		fmt.Fprintf(&b, "Best 20 min power: %.0f W\n", a.Best20MinPower)
	}
	if a.PowerHRDecoupling != 0 && a.VariabilityIndex <= 1.10 {
		fmt.Fprintf(&b, "Power:HR decoupling: %+.1f%%\n", a.PowerHRDecoupling)
	} else if a.VariabilityIndex > 1.10 {
		fmt.Fprintf(&b, "Power:HR decoupling: not reliable for high-variability sessions (VI %.2f)\n", a.VariabilityIndex)
	}
	if a.FTPSource == "estimated" && a.Intervals.WorkCount > 0 {
		b.WriteString("FTP note: estimated from best 20-minute power; use --ftp for more accurate IF/TSS and zone time on interval workouts.\n")
	}

	if len(a.PowerZones) > 0 {
		b.WriteString("\nPower Zone Distribution\n")
		for _, z := range a.PowerZones {
			if z.Seconds <= 0 {
				continue
			}
			fmt.Fprintf(
				&b,
				"- %s: %s (%.1f%%)\n",
				z.Zone,
				formatDuration(z.Seconds),
				z.Percentage,
			)
		}
	}

	b.WriteString("\nInterval Execution\n")
	if a.Intervals.WorkCount > 0 {
		fmt.Fprintf(
			&b,
			"- Detected %d primary work intervals at %.0f W for %s on average.\n",
			a.Intervals.WorkCount,
			a.Intervals.AvgWorkPowerWatts,
			formatDuration(a.Intervals.AvgWorkDurationSeconds),
		)
		if a.Intervals.RecoveryCount > 0 {
			fmt.Fprintf(
				&b,
				"- Recovery intervals: %d reps at %.0f W for %s.\n",
				a.Intervals.RecoveryCount,
				a.Intervals.AvgRecoveryPowerWatts,
				formatDuration(a.Intervals.AvgRecoveryDurationSeconds),
			)
		}
		if a.Intervals.ActivationCount > 0 {
			fmt.Fprintf(&b, "- Pre-set activations: %d short high-power efforts.\n", a.Intervals.ActivationCount)
		}
		fmt.Fprintf(
			&b,
			"- Work interval trend: power %+.1f%%, cadence %+.1f%%, HR %+0.f bpm (first to last interval).\n",
			a.Intervals.WorkPowerChangePct,
			a.Intervals.WorkCadenceChangePct,
			a.Intervals.WorkHeartRateChange,
		)
	} else {
		b.WriteString("- No repeating hard interval structure was confidently detected from lap data.\n")
	}

	if a.WorkoutStructure.CanonicalLabel != "" {
		b.WriteString("\nWorkout Structure\n")
		fmt.Fprintf(
			&b,
			"- %s (confidence %.0f%%)\n",
			a.WorkoutStructure.CanonicalLabel,
			a.WorkoutStructure.Confidence*100.0,
		)
		if a.WorkoutStructure.MainSet != nil {
			fmt.Fprintf(
				&b,
				"- Main set execution: %s, drift %+.1f%% power / %+.1f%% cadence / %+0.f bpm HR.\n",
				a.WorkoutStructure.MainSet.Prescription,
				a.WorkoutStructure.MainSet.PowerDriftPct,
				a.WorkoutStructure.MainSet.CadenceDriftPct,
				a.WorkoutStructure.MainSet.HeartRateDriftBPM,
			)
		}
	}

	b.WriteString("\nCoaching Notes\n")
	b.WriteString("- ")
	b.WriteString(coachingAssessment(a))
	b.WriteString("\n- ")
	b.WriteString(nextSessionSuggestion(a))
	b.WriteByte('\n')

	return strings.TrimSpace(b.String())
}

func coachingAssessment(a *Analysis) string {
	if a == nil {
		return "No assessment available."
	}
	if a.Intervals.WorkCount >= 3 {
		switch {
		case math.Abs(a.Intervals.WorkPowerChangePct) <= 3:
			return "Execution was controlled with minimal fade; pacing and repeatability were strong."
		case a.Intervals.WorkPowerChangePct < -8:
			return "Late-session fade suggests the session sat near your current limit; consider a bit more recovery before the next high-intensity day."
		default:
			return "Interval consistency was acceptable with moderate fatigue signals; target smoother pacing in the final reps."
		}
	}
	if a.IntensityFactor >= 0.9 {
		return "High-intensity load for this duration; prioritize sleep and fueling to absorb the session."
	}
	return "Aerobic load appears manageable and supports base development."
}

func nextSessionSuggestion(a *Analysis) string {
	if a == nil {
		return "No recommendation available."
	}
	if a.Intervals.WorkCount >= 4 && math.Abs(a.Intervals.WorkPowerChangePct) <= 3 {
		return "If recovery is good, progress by adding one work interval or increasing targets by 2-3%."
	}
	if a.Intervals.WorkCount >= 4 && a.Intervals.WorkPowerChangePct < -8 {
		return "Repeat this structure before progressing, with steadier opening intervals to reduce end-of-session drop-off."
	}
	if a.IntensityFactor >= 1.0 {
		return "Follow with an easier endurance day (Z1-Z2) to consolidate adaptations."
	}
	return "Maintain consistent endurance volume and revisit this workout once cadence and HR stability improve."
}

func formatDuration(seconds float64) string {
	if seconds <= 0 {
		return "0s"
	}
	s := int(math.Round(seconds))
	h := s / 3600
	m := (s % 3600) / 60
	sec := s % 60
	if h > 0 {
		return fmt.Sprintf("%dh%02dm%02ds", h, m, sec)
	}
	if m > 0 {
		return fmt.Sprintf("%dm%02ds", m, sec)
	}
	return fmt.Sprintf("%ds", sec)
}

func mpsToKmh(v float64) float64 {
	if v <= 0 {
		return 0
	}
	return v * 3.6
}
