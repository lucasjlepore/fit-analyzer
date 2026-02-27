package pipeline

import "time"

// Options configures the fit_analyze pipeline.
type Options struct {
	FitPath     string
	OutDir      string
	FTPOverride float64
	Format      string // parquet|csv
	Overwrite   bool
	CopySource  bool
}

// Result returns generated output paths.
type Result struct {
	OutputDir            string `json:"output_dir"`
	ManifestPath         string `json:"manifest_path"`
	RecordsPath          string `json:"records_path"`
	SourceCopyPath       string `json:"source_copy_path,omitempty"`
	CanonicalSamplesPath string `json:"canonical_samples_path"`
	MessagesIndexPath    string `json:"messages_index_path"`
	WorkoutStructurePath string `json:"workout_structure_path"`
	LapSummaryPath       string `json:"lap_summary_path,omitempty"`
	ActivitySummaryPath  string `json:"activity_summary_path"`
}

// CanonicalSample represents one global message 20 sample row.
type CanonicalSample struct {
	TSUTCISO     string    `json:"ts_utc_iso"`
	Timestamp    time.Time `json:"-"`
	ElapsedS     float64   `json:"elapsed_s"`
	PowerW       *float64  `json:"power_w,omitempty"`
	HRBPM        *float64  `json:"hr_bpm,omitempty"`
	CadenceRPM   *float64  `json:"cadence_rpm,omitempty"`
	SpeedMPS     *float64  `json:"speed_mps,omitempty"`
	DistanceM    *float64  `json:"distance_m,omitempty"`
	AltitudeM    *float64  `json:"altitude_m,omitempty"`
	TemperatureC *float64  `json:"temperature_c,omitempty"`
	GradePct     *float64  `json:"grade_pct,omitempty"`
	ValidPower   bool      `json:"valid_power"`
	ValidHR      bool      `json:"valid_hr"`
	ValidCadence bool      `json:"valid_cadence"`
	FileOffset   int64     `json:"file_offset"`
	RecordIndex  int       `json:"record_index"`
}

// MessageIndexFile contains local/global message mapping metadata.
type MessageIndexFile struct {
	LocalMessageTypes []LocalMessageIndex `json:"local_message_types"`
	ReverseIndex      map[string][]int    `json:"reverse_index"`
}

// LocalMessageIndex maps one local message type to its global message and fields.
type LocalMessageIndex struct {
	LocalMessageType  int                         `json:"local_message_type"`
	GlobalMessageNum  int                         `json:"global_message_num"`
	GlobalMessageName string                      `json:"global_message_name"`
	Fields            map[string]MessageFieldMeta `json:"fields"`
}

// MessageFieldMeta describes one field in message index.
type MessageFieldMeta struct {
	FieldName   string `json:"field_name"`
	Units       string `json:"units,omitempty"`
	InvalidRule string `json:"invalid_rule,omitempty"`
}

// WorkoutStructureFile is the semantic workout plan/execution output.
type WorkoutStructureFile struct {
	FTPSources []FTPCandidate `json:"ftp_sources"`
	FTPWUsed   *FTPCandidate  `json:"ftp_w_used,omitempty"`
	Steps      []WorkoutStep  `json:"steps,omitempty"`
}

// FTPCandidate is one FTP source hypothesis.
type FTPCandidate struct {
	FTPW       float64 `json:"ftp_w"`
	Source     string  `json:"source"` // zwift_setting|user_profile|developer_field|unknown
	Message    string  `json:"message"`
	Confidence float64 `json:"confidence"`
	Reason     string  `json:"reason,omitempty"`
}

// WorkoutStep describes one workout prescription step.
type WorkoutStep struct {
	StepIndex         int      `json:"step_index"`
	StepName          string   `json:"step_name,omitempty"`
	DurationS         *float64 `json:"duration_s,omitempty"`
	DistanceM         *float64 `json:"distance_m,omitempty"`
	TargetType        string   `json:"target_type"` // power_w|percent_ftp|power_range_w
	TargetLowW        *float64 `json:"target_low_w,omitempty"`
	TargetHighW       *float64 `json:"target_high_w,omitempty"`
	TargetLowPctFTP   *float64 `json:"target_low_pct_ftp,omitempty"`
	TargetHighPctFTP  *float64 `json:"target_high_pct_ftp,omitempty"`
	StartTSUTC        string   `json:"start_ts_utc,omitempty"`
	EndTSUTC          string   `json:"end_ts_utc,omitempty"`
	StartSampleIndex  int      `json:"start_sample_index"`
	EndSampleIndex    int      `json:"end_sample_index"`
	Source            string   `json:"source"` // workout_step|lap|event_derived
	ObservedAvgPowerW *float64 `json:"observed_avg_power_w,omitempty"`
	ObservedNPW       *float64 `json:"observed_np_w,omitempty"`
	TimeInTargetPct   *float64 `json:"time_in_target_pct,omitempty"`
	PowerStdDev       *float64 `json:"power_stddev,omitempty"`
}

// LapSummaryFile contains lap-level aggregate data.
type LapSummaryFile struct {
	Laps []LapSummary `json:"laps"`
}

// LapSummary is one lap summary row.
type LapSummary struct {
	LapIndex         int     `json:"lap_index"`
	StartTS          string  `json:"start_ts"`
	EndTS            string  `json:"end_ts"`
	ElapsedS         float64 `json:"elapsed_s"`
	AvgPowerW        float64 `json:"avg_power_w"`
	MaxPowerW        float64 `json:"max_power_w"`
	AvgHRBPM         float64 `json:"avg_hr_bpm"`
	MaxHRBPM         float64 `json:"max_hr_bpm"`
	AvgCadenceRPM    float64 `json:"avg_cadence_rpm"`
	StartSampleIndex int     `json:"start_sample_index"`
	EndSampleIndex   int     `json:"end_sample_index"`
}

// ActivitySummaryFile contains one-session aggregate metrics.
type ActivitySummaryFile struct {
	DurationS     float64  `json:"duration_s"`
	AvgPowerW     float64  `json:"avg_power_w"`
	NPW           float64  `json:"np_w"`
	MaxPowerW     float64  `json:"max_power_w"`
	AvgHRBPM      float64  `json:"avg_hr_bpm"`
	MaxHRBPM      float64  `json:"max_hr_bpm"`
	AvgCadenceRPM float64  `json:"avg_cadence_rpm"`
	MaxCadenceRPM float64  `json:"max_cadence_rpm"`
	TotalWorkKJ   float64  `json:"total_work_kj"`
	FTPWUsed      *float64 `json:"ftp_w_used,omitempty"`
	IF            *float64 `json:"if,omitempty"`
	TSSLike       *float64 `json:"tss_like,omitempty"`
	Warnings      []string `json:"warnings,omitempty"`
}
