package pipeline

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	fitnotes "fit-analyzer"
	"fit-analyzer/llmexport"
	"github.com/tormoder/fit"
)

// Run executes the full fit_analyze pipeline and writes all required artifacts.
func Run(opts Options) (*Result, error) {
	if strings.TrimSpace(opts.FitPath) == "" {
		return nil, fmt.Errorf("fit path is required")
	}
	if strings.TrimSpace(opts.OutDir) == "" {
		return nil, fmt.Errorf("output directory is required")
	}
	if err := ensureOutputDir(opts.OutDir, opts.Overwrite); err != nil {
		return nil, err
	}

	data, err := os.ReadFile(opts.FitPath)
	if err != nil {
		return nil, fmt.Errorf("read fit file: %w", err)
	}

	bytesResult, err := RunBytes(BytesOptions{
		SourceFileName: filepath.Base(opts.FitPath),
		FitData:        data,
		FTPOverride:    opts.FTPOverride,
		WeightKG:       opts.WeightKG,
		Format:         opts.Format,
		CopySource:     opts.CopySource,
	})
	if err != nil {
		return nil, err
	}

	canonicalName := canonicalArtifactName(bytesResult.Files)
	if canonicalName == "" {
		canonicalName = "canonical_samples." + formatExtension(strings.ToLower(strings.TrimSpace(opts.Format)))
	}
	canonicalPath := filepath.Join(opts.OutDir, canonicalName)
	result := &Result{
		OutputDir:            opts.OutDir,
		ManifestPath:         filepath.Join(opts.OutDir, "manifest.json"),
		RecordsPath:          filepath.Join(opts.OutDir, "records.jsonl"),
		CanonicalSamplesPath: canonicalPath,
		MessagesIndexPath:    filepath.Join(opts.OutDir, "messages_index.json"),
		WorkoutStructurePath: filepath.Join(opts.OutDir, "workout_structure.json"),
		ActivitySummaryPath:  filepath.Join(opts.OutDir, "activity_summary.json"),
		Warnings:             append([]string(nil), bytesResult.Warnings...),
	}
	if _, ok := bytesResult.Files["lap_summary.json"]; ok {
		result.LapSummaryPath = filepath.Join(opts.OutDir, "lap_summary.json")
	}
	if _, ok := bytesResult.Files["source.fit"]; ok {
		result.SourceCopyPath = filepath.Join(opts.OutDir, "source.fit")
	}

	for name, content := range bytesResult.Files {
		path := filepath.Join(opts.OutDir, name)
		if err := os.WriteFile(path, content, 0o644); err != nil {
			return nil, fmt.Errorf("write %s: %w", name, err)
		}
	}
	return result, nil
}

// RunBytes executes fit analysis fully in memory and returns file payloads.
func RunBytes(opts BytesOptions) (*BytesResult, error) {
	if len(opts.FitData) == 0 {
		return nil, fmt.Errorf("fit bytes are required")
	}
	format := strings.ToLower(strings.TrimSpace(opts.Format))
	if format == "" {
		format = "parquet"
	}
	if format != "parquet" && format != "csv" {
		return nil, fmt.Errorf("unsupported format %q (expected parquet|csv)", format)
	}

	sourceName := strings.TrimSpace(opts.SourceFileName)
	if sourceName == "" {
		sourceName = "input.fit"
	}
	files := make(map[string][]byte, 8)
	warnings := make([]string, 0, 8)
	if !strings.HasSuffix(strings.ToLower(sourceName), ".fit") {
		warnings = append(warnings, "input filename does not end with .fit")
	}
	if opts.FTPOverride <= 0 {
		warnings = append(warnings, "ftp override not provided; ftp_w_used will be inferred from FIT metadata when possible")
	}
	if opts.WeightKG <= 0 {
		warnings = append(warnings, "weight_kg missing or invalid; W/kg metrics omitted")
	}

	bundle, err := llmexport.ParseBytes(opts.FitData)
	if err != nil {
		return nil, err
	}
	warnings = append(warnings, llmexport.BuildWarningsFromBundle(bundle)...)

	records := bundle.Records
	samples, err := buildCanonicalSamples(records)
	if err != nil {
		return nil, fmt.Errorf("build canonical samples: %w", err)
	}
	if len(samples) == 0 {
		return nil, fmt.Errorf("no global message 20 record samples found")
	}

	outputFormat := format
	var canonical []byte
	switch format {
	case "csv":
		canonical, err = marshalCanonicalCSV(samples)
		if err != nil {
			return nil, fmt.Errorf("marshal canonical csv: %w", err)
		}
	case "parquet":
		canonical, err = marshalCanonicalParquet(samples)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("parquet unavailable: %v; falling back to csv", err))
			canonical, err = marshalCanonicalCSV(samples)
			if err != nil {
				return nil, fmt.Errorf("marshal canonical csv fallback: %w", err)
			}
			outputFormat = "csv"
		}
	}
	files["canonical_samples."+formatExtension(outputFormat)] = canonical

	indexJSON, err := llmexport.MarshalJSON(buildMessagesIndex(records))
	if err != nil {
		return nil, fmt.Errorf("marshal messages index: %w", err)
	}
	files["messages_index.json"] = indexJSON

	analysis, err := fitnotes.AnalyzeBytes(opts.FitData, sourceName, fitnotes.Config{
		FTPWatts: opts.FTPOverride,
		WeightKG: opts.WeightKG,
	})
	if err != nil {
		return nil, fmt.Errorf("analyze fit bytes: %w", err)
	}
	activity, err := decodeActivityBytes(opts.FitData)
	if err != nil {
		return nil, fmt.Errorf("decode activity: %w", err)
	}

	ftpCandidates := collectFTPCandidates(records, activity, opts.FTPOverride)
	ftpUsed := chooseFTPCandidate(ftpCandidates)
	if ftpUsed == nil {
		warnings = append(warnings, "unable to determine ftp_w_used from metadata or override")
	}

	lapSummary := buildLapSummary(activity, samples)
	if len(lapSummary.Laps) > 0 {
		lapJSON, err := llmexport.MarshalJSON(lapSummary)
		if err != nil {
			return nil, fmt.Errorf("marshal lap summary: %w", err)
		}
		files["lap_summary.json"] = lapJSON
	}

	steps := buildWorkoutSteps(records, analysis, samples, lapSummary, ftpUsed)
	for i := range steps {
		ftp := 0.0
		if ftpUsed != nil {
			ftp = ftpUsed.FTPW
		}
		enrichStepCompliance(&steps[i], samples, ftp)
	}
	workout := WorkoutStructureFile{
		FTPSources: ftpCandidates,
		FTPWUsed:   ftpUsed,
		Steps:      steps,
	}
	workoutJSON, err := llmexport.MarshalJSON(workout)
	if err != nil {
		return nil, fmt.Errorf("marshal workout structure: %w", err)
	}
	files["workout_structure.json"] = workoutJSON

	activitySummary := buildActivitySummary(samples, ftpUsed, analysis.ElapsedSeconds, opts.WeightKG, warnings)
	warnings = dedupeStrings(append(warnings, activitySummary.Warnings...))
	activityJSON, err := llmexport.MarshalJSON(activitySummary)
	if err != nil {
		return nil, fmt.Errorf("marshal activity summary: %w", err)
	}
	files["activity_summary.json"] = activityJSON

	summaryMD := fitnotes.BuildTrainingSummaryMarkdown(analysis)
	if summaryMD != "" {
		files["training_summary.md"] = append([]byte(summaryMD), '\n')
	}

	recordsJSONL, err := llmexport.MarshalJSONL(records)
	if err != nil {
		return nil, fmt.Errorf("marshal records jsonl: %w", err)
	}
	files["records.jsonl"] = recordsJSONL

	manifest, err := buildManifest(sourceName, opts.FitData, bundle, warnings)
	if err != nil {
		return nil, fmt.Errorf("build manifest: %w", err)
	}
	manifestJSON, err := llmexport.MarshalJSON(manifest)
	if err != nil {
		return nil, fmt.Errorf("marshal manifest: %w", err)
	}
	files["manifest.json"] = manifestJSON

	if opts.CopySource {
		files["source.fit"] = append([]byte(nil), opts.FitData...)
	}

	return &BytesResult{
		Files:    files,
		Warnings: dedupeStrings(warnings),
	}, nil
}

func formatExtension(format string) string {
	if format == "csv" {
		return "csv"
	}
	return "parquet"
}

func canonicalArtifactName(files map[string][]byte) string {
	for name := range files {
		if strings.HasPrefix(name, "canonical_samples.") {
			return name
		}
	}
	return ""
}

func ensureOutputDir(path string, overwrite bool) error {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return fmt.Errorf("read output directory: %w", err)
	}
	if len(entries) > 0 && !overwrite {
		return fmt.Errorf("output directory is not empty: %s (set overwrite=true to allow)", path)
	}
	return nil
}

func buildManifest(sourceName string, fitBytes []byte, bundle *llmexport.ParsedBundle, warnings []string) (llmexport.Manifest, error) {
	manifest := llmexport.Manifest{
		FormatVersion:        llmexport.ExportFormatVersion,
		GeneratedAt:          time.Now().UTC(),
		SourceFile:           sourceName,
		SourceFileName:       filepath.Base(sourceName),
		SourceSHA256:         bundle.SourceSHA256,
		SourceSizeBytes:      bundle.SourceSizeBytes,
		Header:               bundle.Header,
		HeaderCRC:            bundle.HeaderCRC,
		FileCRC:              bundle.FileCRC,
		RecordsPath:          "records.jsonl",
		WorkoutStructurePath: "workout_structure.json",
		RecordCount:          len(bundle.Records),
		DefinitionCount:      bundle.DefinitionCount,
		DataMessageCount:     bundle.DataMessageCount,
		LeftoverBytes:        bundle.LeftoverBytesCount,
		FileIdProjection:     llmexport.ProjectFileIDFromBytes(fitBytes),
		SchemaDescription: llmexport.SchemaDetails{
			RecordType: "JSONL line-per-FIT-record preserving original order and byte offsets",
			Notes: []string{
				"Lossless: every FIT data record and field payload is exported with raw hex.",
				"Each line includes decoded values and validity flags without dropping invalid sentinels.",
				"Developer data fields are preserved as raw bytes.",
				"Definition messages are preserved so unknown/global custom messages remain interpretable.",
				"Use record_index and file_offset for deterministic chunking in LLM pipelines.",
				"analysis artifacts provide semantic block labels for LLM reasoning.",
			},
		},
		Warnings: dedupeStrings(warnings),
	}
	return manifest, nil
}

func decodeActivityBytes(data []byte) (*fit.ActivityFile, error) {
	decoded, err := fit.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return decoded.Activity()
}

func buildCanonicalSamples(records []llmexport.RecordEnvelope) ([]CanonicalSample, error) {
	out := make([]CanonicalSample, 0, 4096)
	var firstTS time.Time
	for _, rec := range records {
		if rec.RecordKind != "data" || rec.GlobalMessageNum != 20 || rec.Data == nil {
			continue
		}

		flat := rec.Data.Flat
		if flat == nil {
			flat = recFlatFromFields(rec.Data.Fields)
		}
		if flat == nil || flat.TimestampUTC == "" {
			continue
		}
		ts, err := time.Parse(time.RFC3339, flat.TimestampUTC)
		if err != nil {
			continue
		}
		if firstTS.IsZero() {
			firstTS = ts
		}

		out = append(out, CanonicalSample{
			TSUTCISO:     ts.UTC().Format(time.RFC3339),
			Timestamp:    ts,
			ElapsedS:     ts.Sub(firstTS).Seconds(),
			PowerW:       flat.PowerW,
			HRBPM:        flat.HRBPM,
			CadenceRPM:   flat.CadenceRPM,
			SpeedMPS:     flat.SpeedMPS,
			DistanceM:    flat.DistanceM,
			AltitudeM:    flat.AltitudeM,
			TemperatureC: flat.TemperatureC,
			GradePct:     flat.GradePct,
			ValidPower:   flat.ValidPower,
			ValidHR:      flat.ValidHR,
			ValidCadence: flat.ValidCadence,
			FileOffset:   rec.FileOffset,
			RecordIndex:  rec.RecordIndex,
		})
	}
	return out, nil
}

func recFlatFromFields(fields []llmexport.FieldValue) *llmexport.RecordFlat {
	m := make(map[uint8]llmexport.FieldValue, len(fields))
	for _, f := range fields {
		m[f.FieldNumber] = f
	}
	tsField, ok := m[253]
	if !ok {
		return nil
	}
	utc := ""
	if tsField.Timestamp != nil {
		utc = tsField.Timestamp.UTC
	} else if s, ok := tsField.Scaled.(string); ok {
		utc = s
	}
	if utc == "" {
		return nil
	}
	flat := &llmexport.RecordFlat{
		TimestampUTC: utc,
	}
	if v := floatFromField(m[7]); v != nil && !m[7].Invalid {
		flat.PowerW = v
		flat.ValidPower = true
	}
	if v := floatFromField(m[3]); v != nil && !m[3].Invalid {
		flat.HRBPM = v
		flat.ValidHR = true
	}
	if v := floatFromField(m[4]); v != nil && !m[4].Invalid {
		flat.CadenceRPM = v
		flat.ValidCadence = true
	}
	if v := scaledOrDecodedFloat(m[6]); v != nil {
		flat.SpeedMPS = v
	}
	if v := scaledOrDecodedFloat(m[5]); v != nil {
		flat.DistanceM = v
	}
	if v := scaledOrDecodedFloat(m[2]); v != nil {
		flat.AltitudeM = v
	}
	if v := floatFromField(m[13]); v != nil {
		flat.TemperatureC = v
	}
	if v := scaledOrDecodedFloat(m[9]); v != nil {
		flat.GradePct = v
	}
	return flat
}

func floatFromField(f llmexport.FieldValue) *float64 {
	return floatAny(f.Decoded)
}

func scaledOrDecodedFloat(f llmexport.FieldValue) *float64 {
	if f.Scaled != nil {
		if v := floatAny(f.Scaled); v != nil {
			return v
		}
	}
	return floatAny(f.Decoded)
}

func floatAny(v any) *float64 {
	switch x := v.(type) {
	case float64:
		out := x
		return &out
	case float32:
		out := float64(x)
		return &out
	case int:
		out := float64(x)
		return &out
	case int8:
		out := float64(x)
		return &out
	case int16:
		out := float64(x)
		return &out
	case int32:
		out := float64(x)
		return &out
	case int64:
		out := float64(x)
		return &out
	case uint:
		out := float64(x)
		return &out
	case uint8:
		out := float64(x)
		return &out
	case uint16:
		out := float64(x)
		return &out
	case uint32:
		out := float64(x)
		return &out
	case uint64:
		out := float64(x)
		return &out
	default:
		return nil
	}
}

func buildMessagesIndex(records []llmexport.RecordEnvelope) MessageIndexFile {
	localLatest := make(map[int]LocalMessageIndex)
	reverseSets := make(map[string]map[int]struct{})

	for _, rec := range records {
		if rec.RecordKind != "definition" || rec.Definition == nil {
			continue
		}
		local := int(rec.LocalMessageType)
		global := int(rec.Definition.GlobalMessageNum)
		fields := make(map[string]MessageFieldMeta, len(rec.Definition.FieldDefinitions))
		for _, fd := range rec.Definition.FieldDefinitions {
			key := strconv.Itoa(int(fd.FieldNumber))
			fields[key] = MessageFieldMeta{
				FieldName:   fd.FieldName,
				Units:       fd.Units,
				InvalidRule: fd.InvalidRule,
			}
		}
		localLatest[local] = LocalMessageIndex{
			LocalMessageType:  local,
			GlobalMessageNum:  global,
			GlobalMessageName: fmt.Sprint(fit.MesgNum(global)),
			Fields:            fields,
		}

		gKey := strconv.Itoa(global)
		if _, ok := reverseSets[gKey]; !ok {
			reverseSets[gKey] = make(map[int]struct{})
		}
		reverseSets[gKey][local] = struct{}{}
	}

	locals := make([]int, 0, len(localLatest))
	for k := range localLatest {
		locals = append(locals, k)
	}
	sort.Ints(locals)
	localList := make([]LocalMessageIndex, 0, len(locals))
	for _, k := range locals {
		localList = append(localList, localLatest[k])
	}

	reverse := make(map[string][]int, len(reverseSets))
	for gKey, set := range reverseSets {
		list := make([]int, 0, len(set))
		for l := range set {
			list = append(list, l)
		}
		sort.Ints(list)
		reverse[gKey] = list
	}
	return MessageIndexFile{
		LocalMessageTypes: localList,
		ReverseIndex:      reverse,
	}
}

func collectFTPCandidates(records []llmexport.RecordEnvelope, activity *fit.ActivityFile, ftpOverride float64) []FTPCandidate {
	candidates := make([]FTPCandidate, 0, 6)
	add := func(c FTPCandidate) {
		if c.FTPW <= 0 || c.FTPW > 600 {
			return
		}
		candidates = append(candidates, c)
	}

	if activity != nil && len(activity.Sessions) > 0 {
		s := activity.Sessions[0]
		if s.ThresholdPower != 0 && s.ThresholdPower != ^uint16(0) {
			add(FTPCandidate{
				FTPW:       float64(s.ThresholdPower),
				Source:     "zwift_setting",
				Message:    "session.threshold_power",
				Confidence: 0.95,
				Reason:     "Session threshold power field present",
			})
		}
	}

	type devKey struct{ idx, field int }
	type devDesc struct {
		name    string
		baseRaw int
	}
	descMap := make(map[devKey]devDesc)
	for _, rec := range records {
		if rec.RecordKind != "data" || rec.Data == nil {
			continue
		}
		if rec.GlobalMessageNum == 206 {
			fdIdx := int(fieldFloatValue(rec.Data.Fields, 0))
			fieldNum := int(fieldFloatValue(rec.Data.Fields, 1))
			baseRaw := int(fieldFloatValue(rec.Data.Fields, 2))
			name := fieldStringValue(rec.Data.Fields, 3)
			if fdIdx >= 0 && fieldNum >= 0 && name != "" {
				descMap[devKey{idx: fdIdx, field: fieldNum}] = devDesc{name: strings.ToLower(name), baseRaw: baseRaw}
			}
		}
	}
	for _, rec := range records {
		if rec.RecordKind != "data" || rec.Data == nil {
			continue
		}
		for _, d := range rec.Data.DeveloperFields {
			key := devKey{idx: int(d.DeveloperDataIdx), field: int(d.FieldNumber)}
			desc, ok := descMap[key]
			if !ok {
				continue
			}
			if !strings.Contains(desc.name, "ftp") {
				continue
			}
			val := decodeDeveloperNumeric(d.DecodedByteValues, desc.baseRaw)
			if val <= 0 {
				continue
			}
			add(FTPCandidate{
				FTPW:       val,
				Source:     "developer_field",
				Message:    fmt.Sprintf("developer_field[%d:%d](%s)", d.DeveloperDataIdx, d.FieldNumber, desc.name),
				Confidence: 0.80,
				Reason:     "Developer field name matched FTP",
			})
		}
	}

	if ftpOverride > 0 {
		add(FTPCandidate{
			FTPW:       ftpOverride,
			Source:     "unknown",
			Message:    "cli:--ftp",
			Confidence: 0.55,
			Reason:     "CLI override provided",
		})
	}

	// Deterministic de-dup by source+message+rounded ftp.
	seen := make(map[string]struct{})
	dedup := make([]FTPCandidate, 0, len(candidates))
	for _, c := range candidates {
		key := fmt.Sprintf("%s|%s|%.1f", c.Source, c.Message, c.FTPW)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		dedup = append(dedup, c)
	}
	sort.Slice(dedup, func(i, j int) bool {
		pi, pj := ftpPriority(dedup[i].Source), ftpPriority(dedup[j].Source)
		if pi != pj {
			return pi > pj
		}
		if dedup[i].Confidence != dedup[j].Confidence {
			return dedup[i].Confidence > dedup[j].Confidence
		}
		if dedup[i].FTPW != dedup[j].FTPW {
			return dedup[i].FTPW > dedup[j].FTPW
		}
		return dedup[i].Message < dedup[j].Message
	})
	return dedup
}

func ftpPriority(source string) int {
	switch source {
	case "zwift_setting":
		return 4
	case "developer_field":
		return 3
	case "user_profile":
		return 2
	default:
		return 1
	}
}

func chooseFTPCandidate(candidates []FTPCandidate) *FTPCandidate {
	if len(candidates) == 0 {
		return nil
	}
	chosen := candidates[0]
	chosen.Reason = fmt.Sprintf("Selected highest-priority/highest-confidence source: %s (%s)", chosen.Source, chosen.Message)
	return &chosen
}

func fieldFloatValue(fields []llmexport.FieldValue, num uint8) float64 {
	for _, f := range fields {
		if f.FieldNumber == num {
			if v := floatAny(f.Decoded); v != nil {
				return *v
			}
		}
	}
	return -1
}

func fieldStringValue(fields []llmexport.FieldValue, num uint8) string {
	for _, f := range fields {
		if f.FieldNumber == num {
			if s, ok := f.Decoded.(string); ok {
				return s
			}
		}
	}
	return ""
}

func decodeDeveloperNumeric(values []int, baseRaw int) float64 {
	if len(values) == 0 {
		return 0
	}
	// Heuristic decoding for common uint16/uint32 fields.
	if len(values) >= 2 && (baseRaw&0x1F) == 0x04 { // uint16
		return float64(values[0] | (values[1] << 8))
	}
	if len(values) >= 4 && (baseRaw&0x1F) == 0x06 { // uint32
		return float64(values[0] | (values[1] << 8) | (values[2] << 16) | (values[3] << 24))
	}
	return float64(values[0])
}

func buildLapSummary(activity *fit.ActivityFile, samples []CanonicalSample) LapSummaryFile {
	if activity == nil || len(activity.Laps) == 0 {
		return LapSummaryFile{}
	}
	laps := make([]LapSummary, 0, len(activity.Laps))
	for i, lap := range activity.Laps {
		if lap == nil {
			continue
		}
		start := lap.StartTime.UTC()
		end := lap.Timestamp.UTC()
		elapsed := lap.GetTotalTimerTimeScaled()
		if elapsed <= 0 {
			elapsed = lap.GetTotalElapsedTimeScaled()
		}
		startIdx := sampleIndexAtOrAfter(samples, start)
		endIdx := sampleIndexAtOrBefore(samples, end)
		laps = append(laps, LapSummary{
			LapIndex:         i + 1,
			StartTS:          start.Format(time.RFC3339),
			EndTS:            end.Format(time.RFC3339),
			ElapsedS:         elapsed,
			AvgPowerW:        float64(safeU16(lap.AvgPower)),
			MaxPowerW:        float64(safeU16(lap.MaxPower)),
			AvgHRBPM:         float64(safeU8(lap.AvgHeartRate)),
			MaxHRBPM:         float64(safeU8(lap.MaxHeartRate)),
			AvgCadenceRPM:    cadenceFromLapAny(lap.GetAvgCadence()),
			StartSampleIndex: startIdx,
			EndSampleIndex:   endIdx,
		})
	}
	return LapSummaryFile{Laps: laps}
}

func buildWorkoutSteps(records []llmexport.RecordEnvelope, analysis *fitnotes.Analysis, samples []CanonicalSample, lapSummary LapSummaryFile, ftpUsed *FTPCandidate) []WorkoutStep {
	if steps := buildWorkoutStepsFromWorkoutMessages(records, samples, ftpUsed); len(steps) > 0 {
		return steps
	}
	if len(lapSummary.Laps) > 0 && analysis != nil && len(analysis.Laps) == len(lapSummary.Laps) {
		return buildWorkoutStepsFromLaps(analysis, lapSummary, ftpUsed)
	}

	if len(samples) == 0 {
		return nil
	}
	startIdx, endIdx := 0, len(samples)-1
	dur := samples[endIdx].ElapsedS - samples[startIdx].ElapsedS
	step := WorkoutStep{
		StepIndex:        1,
		StepName:         "activity",
		DurationS:        floatPtr(dur),
		TargetType:       "power_w",
		StartTSUTC:       samples[startIdx].TSUTCISO,
		EndTSUTC:         samples[endIdx].TSUTCISO,
		StartSampleIndex: startIdx,
		EndSampleIndex:   endIdx,
		Source:           "event_derived",
	}
	return []WorkoutStep{step}
}

func buildWorkoutStepsFromWorkoutMessages(records []llmexport.RecordEnvelope, samples []CanonicalSample, ftpUsed *FTPCandidate) []WorkoutStep {
	stepsRaw := make([]map[uint8]llmexport.FieldValue, 0)
	for _, rec := range records {
		if rec.RecordKind == "data" && rec.GlobalMessageNum == 27 && rec.Data != nil {
			m := make(map[uint8]llmexport.FieldValue, len(rec.Data.Fields))
			for _, f := range rec.Data.Fields {
				m[f.FieldNumber] = f
			}
			stepsRaw = append(stepsRaw, m)
		}
	}
	if len(stepsRaw) == 0 || len(samples) == 0 {
		return nil
	}

	startTS := samples[0].Timestamp
	steps := make([]WorkoutStep, 0, len(stepsRaw))
	cursor := 0.0
	for i, m := range stepsRaw {
		step := WorkoutStep{
			StepIndex: i + 1,
			Source:    "workout_step",
		}
		if name, ok := asString(m[0].Decoded); ok {
			step.StepName = name
		}
		durationType := int(asFloatDefault(m[1].Decoded, -1))
		durationValue := asFloatDefault(m[2].Decoded, 0)
		if durationType == 0 || durationType == 28 || durationType == 31 {
			d := durationValue / 1000.0
			step.DurationS = floatPtr(d)
		} else if durationType == 1 {
			dist := durationValue / 100.0
			step.DistanceM = floatPtr(dist)
		}

		targetType := int(asFloatDefault(m[3].Decoded, -1))
		targetValue := asFloatDefault(m[4].Decoded, 0)
		targetLow := asFloatDefault(m[5].Decoded, 0)
		targetHigh := asFloatDefault(m[6].Decoded, 0)

		configureTargetFromWorkoutValues(&step, targetType, targetValue, targetLow, targetHigh, ftpUsed)

		stepStart := startTS.Add(time.Duration(cursor * float64(time.Second)))
		step.StartTSUTC = stepStart.UTC().Format(time.RFC3339)
		if step.DurationS != nil {
			cursor += *step.DurationS
		}
		stepEnd := startTS.Add(time.Duration(cursor * float64(time.Second)))
		step.EndTSUTC = stepEnd.UTC().Format(time.RFC3339)
		step.StartSampleIndex = sampleIndexAtOrAfter(samples, stepStart)
		step.EndSampleIndex = sampleIndexAtOrBefore(samples, stepEnd)

		steps = append(steps, step)
	}
	return steps
}

func configureTargetFromWorkoutValues(step *WorkoutStep, targetType int, targetValue, low, high float64, ftpUsed *FTPCandidate) {
	// target_type power for workout steps.
	if targetType == 4 {
		lowW, lowPct := decodeWorkoutPowerValue(low)
		highW, highPct := decodeWorkoutPowerValue(high)
		valW, valPct := decodeWorkoutPowerValue(targetValue)

		if low > 0 && high > 0 {
			if lowW > 0 || highW > 0 {
				step.TargetType = "power_range_w"
				step.TargetLowW = floatPtr(nonZeroOr(lowW, valW))
				step.TargetHighW = floatPtr(nonZeroOr(highW, valW))
			} else {
				step.TargetType = "percent_ftp"
				step.TargetLowPctFTP = floatPtr(nonZeroOr(lowPct, valPct))
				step.TargetHighPctFTP = floatPtr(nonZeroOr(highPct, valPct))
			}
		} else if valW > 0 {
			step.TargetType = "power_w"
			step.TargetLowW = floatPtr(valW)
			step.TargetHighW = floatPtr(valW)
		} else if valPct > 0 {
			step.TargetType = "percent_ftp"
			step.TargetLowPctFTP = floatPtr(valPct)
			step.TargetHighPctFTP = floatPtr(valPct)
		}
	} else {
		step.TargetType = "power_w"
	}

	if ftpUsed != nil && ftpUsed.FTPW > 0 {
		applyFTPConversions(step, ftpUsed.FTPW)
	}
}

func decodeWorkoutPowerValue(v float64) (watts float64, pctFTP float64) {
	if v <= 0 {
		return 0, 0
	}
	if v >= 1000 {
		return v - 1000, 0
	}
	return 0, v
}

func nonZeroOr(primary, fallback float64) float64 {
	if primary > 0 {
		return primary
	}
	return fallback
}

func buildWorkoutStepsFromLaps(analysis *fitnotes.Analysis, lapSummary LapSummaryFile, ftpUsed *FTPCandidate) []WorkoutStep {
	steps := make([]WorkoutStep, 0, len(lapSummary.Laps))
	for i, lap := range lapSummary.Laps {
		label := analysis.Laps[i].Label
		step := WorkoutStep{
			StepIndex:        i + 1,
			StepName:         label,
			DurationS:        floatPtr(lap.ElapsedS),
			TargetType:       "power_w",
			TargetLowW:       floatPtr(roundToNearest(lap.AvgPowerW, 5)),
			TargetHighW:      floatPtr(roundToNearest(lap.AvgPowerW, 5)),
			StartTSUTC:       lap.StartTS,
			EndTSUTC:         lap.EndTS,
			StartSampleIndex: lap.StartSampleIndex,
			EndSampleIndex:   lap.EndSampleIndex,
			Source:           "lap",
		}
		if ftpUsed != nil && ftpUsed.FTPW > 0 {
			pct := (lap.AvgPowerW / ftpUsed.FTPW) * 100
			if label == "work" || label == "recovery" {
				step.TargetType = "percent_ftp"
				step.TargetLowPctFTP = floatPtr(roundToNearest(pct, 1))
				step.TargetHighPctFTP = floatPtr(roundToNearest(pct, 1))
				step.TargetLowW = floatPtr(roundToNearest(lap.AvgPowerW, 5))
				step.TargetHighW = floatPtr(roundToNearest(lap.AvgPowerW, 5))
			}
		}
		steps = append(steps, step)
	}
	return steps
}

func applyFTPConversions(step *WorkoutStep, ftp float64) {
	if ftp <= 0 {
		return
	}
	if step.TargetLowPctFTP != nil && step.TargetLowW == nil {
		v := (ftp * (*step.TargetLowPctFTP)) / 100.0
		step.TargetLowW = floatPtr(v)
	}
	if step.TargetHighPctFTP != nil && step.TargetHighW == nil {
		v := (ftp * (*step.TargetHighPctFTP)) / 100.0
		step.TargetHighW = floatPtr(v)
	}
	if step.TargetLowW != nil && step.TargetLowPctFTP == nil {
		v := (*step.TargetLowW / ftp) * 100.0
		step.TargetLowPctFTP = floatPtr(v)
	}
	if step.TargetHighW != nil && step.TargetHighPctFTP == nil {
		v := (*step.TargetHighW / ftp) * 100.0
		step.TargetHighPctFTP = floatPtr(v)
	}
}

func enrichStepCompliance(step *WorkoutStep, samples []CanonicalSample, ftp float64) {
	if len(samples) == 0 || step.StartSampleIndex < 0 || step.EndSampleIndex < step.StartSampleIndex || step.EndSampleIndex >= len(samples) {
		return
	}
	segment := samples[step.StartSampleIndex : step.EndSampleIndex+1]
	powers := make([]float64, 0, len(segment))
	inTarget := 0
	validCount := 0

	lowW := -1.0
	highW := -1.0
	if step.TargetLowW != nil {
		lowW = *step.TargetLowW
	}
	if step.TargetHighW != nil {
		highW = *step.TargetHighW
	}
	if (lowW <= 0 || highW <= 0) && ftp > 0 {
		if step.TargetLowPctFTP != nil {
			lowW = ftp * (*step.TargetLowPctFTP) / 100.0
		}
		if step.TargetHighPctFTP != nil {
			highW = ftp * (*step.TargetHighPctFTP) / 100.0
		}
	}

	for _, s := range segment {
		if s.PowerW == nil || !s.ValidPower {
			continue
		}
		p := *s.PowerW
		powers = append(powers, p)
		validCount++
		if lowW > 0 && highW > 0 && p >= lowW && p <= highW {
			inTarget++
		}
	}
	if len(powers) == 0 {
		return
	}

	avg := avgFloat(powers)
	step.ObservedAvgPowerW = floatPtr(avg)
	np := normalizedPowerFromFloats(powers)
	step.ObservedNPW = floatPtr(np)
	sd := stddevFloat(powers, avg)
	step.PowerStdDev = floatPtr(sd)
	if lowW > 0 && highW > 0 && validCount > 0 {
		pct := (float64(inTarget) / float64(validCount)) * 100.0
		step.TimeInTargetPct = floatPtr(pct)
	}
}

func buildActivitySummary(samples []CanonicalSample, ftpUsed *FTPCandidate, fallbackDuration float64, weightKG float64, warnings []string) ActivitySummaryFile {
	power := make([]float64, 0, len(samples))
	hr := make([]float64, 0, len(samples))
	cad := make([]float64, 0, len(samples))
	for _, s := range samples {
		if s.PowerW != nil && s.ValidPower {
			power = append(power, *s.PowerW)
		}
		if s.HRBPM != nil && s.ValidHR {
			hr = append(hr, *s.HRBPM)
		}
		if s.CadenceRPM != nil && s.ValidCadence {
			cad = append(cad, *s.CadenceRPM)
		}
	}

	duration := fallbackDuration
	if duration <= 0 && len(samples) > 1 {
		duration = samples[len(samples)-1].ElapsedS - samples[0].ElapsedS
	}
	if duration <= 0 {
		duration = float64(len(samples))
	}
	np := normalizedPowerFromFloats(power)
	workKJ := totalWorkKJ(samples)

	summary := ActivitySummaryFile{
		DurationS:     duration,
		AvgPowerW:     avgFloat(power),
		NPW:           np,
		MaxPowerW:     maxFloat(power),
		AvgHRBPM:      avgFloat(hr),
		MaxHRBPM:      maxFloat(hr),
		AvgCadenceRPM: avgFloat(cad),
		MaxCadenceRPM: maxFloat(cad),
		TotalWorkKJ:   workKJ,
		Warnings:      append([]string(nil), warnings...),
	}
	if weightKG > 0 {
		summary.WeightKG = floatPtr(weightKG)
		summary.AvgPowerWPerKG = floatPtr(summary.AvgPowerW / weightKG)
		summary.NPWPerKG = floatPtr(summary.NPW / weightKG)
		summary.MaxPowerWPerKG = floatPtr(summary.MaxPowerW / weightKG)
	} else {
		summary.Warnings = append(summary.Warnings, "weight_kg unavailable: W/kg metrics omitted")
	}
	if ftpUsed == nil || ftpUsed.FTPW <= 0 {
		summary.Warnings = append(summary.Warnings, "ftp_w_used unavailable: IF and tss_like omitted")
		summary.Warnings = dedupeStrings(summary.Warnings)
		return summary
	}

	ftp := ftpUsed.FTPW
	summary.FTPWUsed = floatPtr(ftp)
	ifv := np / ftp
	summary.IF = floatPtr(ifv)
	tss := (duration / 3600.0) * ifv * ifv * 100.0
	summary.TSSLike = floatPtr(tss)
	if ftpUsed.Source == "unknown" {
		summary.Warnings = append(summary.Warnings, "ftp_w_used selected from override/unknown source")
	}
	summary.Warnings = dedupeStrings(summary.Warnings)
	return summary
}

func totalWorkKJ(samples []CanonicalSample) float64 {
	if len(samples) == 0 {
		return 0
	}
	work := 0.0
	for i := 1; i < len(samples); i++ {
		prev := samples[i-1]
		if prev.PowerW == nil || !prev.ValidPower {
			continue
		}
		delta := samples[i].Timestamp.Sub(prev.Timestamp).Seconds()
		if delta <= 0 || delta > 5 {
			delta = 1
		}
		work += (*prev.PowerW) * delta
	}
	if work == 0 {
		for _, s := range samples {
			if s.PowerW != nil && s.ValidPower {
				work += *s.PowerW
			}
		}
	}
	return work / 1000.0
}

func normalizedPowerFromFloats(power []float64) float64 {
	if len(power) == 0 {
		return 0
	}
	if len(power) < 30 {
		return avgFloat(power)
	}
	window := 30
	sum := 0.0
	for i := 0; i < window; i++ {
		sum += power[i]
	}
	totalFourth := 0.0
	count := 0
	for i := window - 1; i < len(power); i++ {
		if i >= window {
			sum += power[i] - power[i-window]
		}
		roll := sum / float64(window)
		totalFourth += math.Pow(roll, 4)
		count++
	}
	if count == 0 {
		return avgFloat(power)
	}
	return math.Pow(totalFourth/float64(count), 0.25)
}

func avgFloat(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func maxFloat(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	m := values[0]
	for i := 1; i < len(values); i++ {
		if values[i] > m {
			m = values[i]
		}
	}
	return m
}

func stddevFloat(values []float64, mean float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		d := v - mean
		sum += d * d
	}
	return math.Sqrt(sum / float64(len(values)))
}

func writeJSON(path string, v any) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func writeCanonicalCSV(path string, samples []CanonicalSample) error {
	out, err := marshalCanonicalCSV(samples)
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644)
}

func marshalCanonicalCSV(samples []CanonicalSample) ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	header := []string{
		"ts_utc_iso", "elapsed_s", "power_w", "hr_bpm", "cadence_rpm", "speed_mps", "distance_m", "altitude_m", "temperature_c", "grade_pct",
		"valid_power", "valid_hr", "valid_cadence", "file_offset", "record_index",
	}
	if err := w.Write(header); err != nil {
		return nil, err
	}
	for _, s := range samples {
		row := []string{
			s.TSUTCISO,
			formatFloat(s.ElapsedS),
			formatFloatPtr(s.PowerW),
			formatFloatPtr(s.HRBPM),
			formatFloatPtr(s.CadenceRPM),
			formatFloatPtr(s.SpeedMPS),
			formatFloatPtr(s.DistanceM),
			formatFloatPtr(s.AltitudeM),
			formatFloatPtr(s.TemperatureC),
			formatFloatPtr(s.GradePct),
			strconv.FormatBool(s.ValidPower),
			strconv.FormatBool(s.ValidHR),
			strconv.FormatBool(s.ValidCadence),
			strconv.FormatInt(s.FileOffset, 10),
			strconv.Itoa(s.RecordIndex),
		}
		if err := w.Write(row); err != nil {
			return nil, err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeCanonicalParquet(path string, samples []CanonicalSample) error {
	out, err := marshalCanonicalParquet(samples)
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644)
}

func valueOrNaN(v *float64) float64 {
	if v == nil {
		return math.NaN()
	}
	return *v
}

func sampleIndexAtOrAfter(samples []CanonicalSample, ts time.Time) int {
	if len(samples) == 0 {
		return 0
	}
	i := sort.Search(len(samples), func(i int) bool {
		return !samples[i].Timestamp.Before(ts)
	})
	if i >= len(samples) {
		return len(samples) - 1
	}
	return i
}

func sampleIndexAtOrBefore(samples []CanonicalSample, ts time.Time) int {
	if len(samples) == 0 {
		return 0
	}
	i := sort.Search(len(samples), func(i int) bool {
		return samples[i].Timestamp.After(ts)
	})
	if i <= 0 {
		return 0
	}
	if i > len(samples) {
		return len(samples) - 1
	}
	return i - 1
}

func safeU16(v uint16) uint16 {
	if v == ^uint16(0) {
		return 0
	}
	return v
}

func safeU8(v uint8) uint8 {
	if v == ^uint8(0) {
		return 0
	}
	return v
}

func cadenceFromLapAny(v any) float64 {
	switch x := v.(type) {
	case uint8:
		return float64(x)
	case uint16:
		return float64(x)
	case float64:
		return x
	default:
		return 0
	}
}

func roundToNearest(v, step float64) float64 {
	if step <= 0 {
		return v
	}
	return math.Round(v/step) * step
}

func asFloatDefault(v any, def float64) float64 {
	if p := floatAny(v); p != nil {
		return *p
	}
	return def
}

func asString(v any) (string, bool) {
	s, ok := v.(string)
	return s, ok
}

func floatPtr(v float64) *float64 {
	out := v
	return &out
}

func dedupeStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func formatFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', 6, 64)
}

func formatFloatPtr(v *float64) string {
	if v == nil {
		return ""
	}
	return formatFloat(*v)
}
