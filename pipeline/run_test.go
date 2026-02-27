package pipeline

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRunOnKnownZwiftFIT(t *testing.T) {
	fitPath := "/Users/lucaslepore/Downloads/Zwift_W1_5x4_110.fit"
	if _, err := os.Stat(fitPath); err != nil {
		t.Skipf("sample fit file not found at %s", fitPath)
	}

	outDir := filepath.Join(t.TempDir(), "out")
	res, err := Run(Options{
		FitPath:     fitPath,
		OutDir:      outDir,
		FTPOverride: 223,
		WeightKG:    72.5,
		Format:      "csv",
		Overwrite:   true,
		CopySource:  false,
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// canonical_samples has required columns and roughly 1Hz count.
	f, err := os.Open(res.CanonicalSamplesPath)
	if err != nil {
		t.Fatalf("open canonical samples: %v", err)
	}
	defer f.Close()
	cr := csv.NewReader(f)
	rows, err := cr.ReadAll()
	if err != nil {
		t.Fatalf("read canonical csv: %v", err)
	}
	if len(rows) < 3000 {
		t.Fatalf("expected ~1Hz sample count, got %d rows", len(rows)-1)
	}
	header := rows[0]
	required := []string{
		"ts_utc_iso", "elapsed_s", "power_w", "hr_bpm", "cadence_rpm", "speed_mps", "distance_m", "altitude_m", "temperature_c", "grade_pct",
		"valid_power", "valid_hr", "valid_cadence", "file_offset", "record_index",
	}
	for i, col := range required {
		if i >= len(header) || header[i] != col {
			t.Fatalf("unexpected header column %d: got %q want %q", i, header[i], col)
		}
	}

	activitySummary := ActivitySummaryFile{}
	data, err := os.ReadFile(res.ActivitySummaryPath)
	if err != nil {
		t.Fatalf("read activity summary: %v", err)
	}
	if err := json.Unmarshal(data, &activitySummary); err != nil {
		t.Fatalf("unmarshal activity summary: %v", err)
	}
	if activitySummary.NPW <= 0 {
		t.Fatalf("expected np_w > 0, got %v", activitySummary.NPW)
	}
	if activitySummary.WeightKG == nil || *activitySummary.WeightKG <= 0 {
		t.Fatalf("expected weight_kg to be populated")
	}
	if activitySummary.NPWPerKG == nil || *activitySummary.NPWPerKG <= 0 {
		t.Fatalf("expected np_w_per_kg > 0")
	}

	structure := WorkoutStructureFile{}
	data, err = os.ReadFile(res.WorkoutStructurePath)
	if err != nil {
		t.Fatalf("read workout structure: %v", err)
	}
	if err := json.Unmarshal(data, &structure); err != nil {
		t.Fatalf("unmarshal workout structure: %v", err)
	}
	if structure.FTPWUsed == nil || structure.FTPWUsed.FTPW <= 0 {
		t.Fatalf("expected ftp_w_used when override supplied")
	}

	sampleCount := len(rows) - 1
	for _, step := range structure.Steps {
		if step.StartSampleIndex < 0 || step.EndSampleIndex < step.StartSampleIndex || step.EndSampleIndex >= sampleCount {
			t.Fatalf("invalid sample indices for step %d: %d..%d (sample_count=%d)", step.StepIndex, step.StartSampleIndex, step.EndSampleIndex, sampleCount)
		}
		if step.DurationS != nil && step.StartTSUTC != "" && step.EndTSUTC != "" {
			start, err := time.Parse(time.RFC3339, step.StartTSUTC)
			if err != nil {
				t.Fatalf("parse step start time: %v", err)
			}
			end, err := time.Parse(time.RFC3339, step.EndTSUTC)
			if err != nil {
				t.Fatalf("parse step end time: %v", err)
			}
			diff := end.Sub(start).Seconds() - *step.DurationS
			if diff < -2 || diff > 2 {
				t.Fatalf("step duration mismatch >2s for step %d: start/end=%.1fs duration=%.1fs", step.StepIndex, end.Sub(start).Seconds(), *step.DurationS)
			}
		}
	}
}

func TestRunBytesProducesArtifacts(t *testing.T) {
	fitPath := "/Users/lucaslepore/Downloads/Zwift_W1_5x4_110.fit"
	data, err := os.ReadFile(fitPath)
	if err != nil {
		t.Skipf("sample fit file not found at %s", fitPath)
	}

	res, err := RunBytes(BytesOptions{
		SourceFileName: "Zwift_W1_5x4_110.fit",
		FitData:        data,
		FTPOverride:    223,
		WeightKG:       72.5,
		Format:         "csv",
		CopySource:     true,
	})
	if err != nil {
		t.Fatalf("RunBytes() error: %v", err)
	}

	required := []string{
		"manifest.json",
		"records.jsonl",
		"messages_index.json",
		"workout_structure.json",
		"activity_summary.json",
		"training_summary.md",
		"canonical_samples.csv",
		"source.fit",
	}
	for _, name := range required {
		if _, ok := res.Files[name]; !ok {
			t.Fatalf("missing artifact %s", name)
		}
	}
}
