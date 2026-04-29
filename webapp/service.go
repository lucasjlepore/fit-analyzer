package webapp

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/lucasjlepore/fit-analyzer/analyzer"
	"github.com/lucasjlepore/fit-analyzer/pipeline"
	"github.com/lucasjlepore/fit-analyzer/raceplan"
)

// AnalyzeOptions configures one in-browser analysis run.
type AnalyzeOptions struct {
	SourceFileName string
	FitData        []byte
	FTPWatts       float64
	WeightKG       float64
	Format         string
}

// AnalyzeResult packages analyzer output and downloadable artifacts for the UI.
type AnalyzeResult struct {
	Analysis        *analyzer.Analysis
	SummaryMarkdown string
	Warnings        []string
	Files           map[string][]byte
	ArtifactNames   []string
	Zip             []byte
}

// RacePlanOptions configures one in-browser race planning run.
type RacePlanOptions struct {
	SourceFileName  string
	FitData         []byte
	FTPWatts        float64
	WeightKG        float64
	MaxCarbGPerHour float64
	BottleML        float64
	StartBottles    int
	CaffeineMgPerKG float64
	StrategyMode    string
}

// RacePlanResult packages route planning output and downloadable artifacts for the UI.
type RacePlanResult struct {
	Plan            *raceplan.Plan
	SummaryMarkdown string
	Warnings        []string
	Files           map[string][]byte
	ArtifactNames   []string
	Zip             []byte
}

// AnalyzeBytes runs the browser-safe analysis pipeline and assembles a ZIP bundle.
func AnalyzeBytes(opts AnalyzeOptions) (*AnalyzeResult, error) {
	format := strings.TrimSpace(opts.Format)
	if format == "" {
		format = "csv"
	}

	result, err := pipeline.RunBytes(pipeline.BytesOptions{
		SourceFileName: opts.SourceFileName,
		FitData:        opts.FitData,
		FTPOverride:    opts.FTPWatts,
		WeightKG:       opts.WeightKG,
		Format:         format,
		CopySource:     true,
	})
	if err != nil {
		return nil, err
	}

	zipBytes, err := zipArtifacts(result.Files)
	if err != nil {
		return nil, fmt.Errorf("create zip: %w", err)
	}

	fileNames := make([]string, 0, len(result.Files))
	for name := range result.Files {
		fileNames = append(fileNames, name)
	}
	sort.Strings(fileNames)

	return &AnalyzeResult{
		Analysis:        result.Analysis,
		SummaryMarkdown: string(result.Files["training_summary.md"]),
		Warnings:        append([]string(nil), result.Warnings...),
		Files:           result.Files,
		ArtifactNames:   fileNames,
		Zip:             zipBytes,
	}, nil
}

// PlanRaceBytes runs the browser-safe route planning pipeline and assembles a ZIP bundle.
func PlanRaceBytes(opts RacePlanOptions) (*RacePlanResult, error) {
	plan, err := raceplan.PlanBytes(opts.SourceFileName, opts.FitData, raceplan.Profile{
		FTPWatts:        opts.FTPWatts,
		WeightKG:        opts.WeightKG,
		MaxCarbGPerHour: opts.MaxCarbGPerHour,
		BottleML:        opts.BottleML,
		StartBottles:    opts.StartBottles,
		CaffeineMgPerKG: opts.CaffeineMgPerKG,
		StrategyMode:    opts.StrategyMode,
	})
	if err != nil {
		return nil, err
	}

	planJSON, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal race plan: %w", err)
	}
	planJSON = append(planJSON, '\n')

	summaryMD := raceplan.BuildMarkdown(plan)
	files := map[string][]byte{
		"race_plan.json": []byte(planJSON),
		"race_plan.md":   []byte(summaryMD),
		"source.fit":     append([]byte(nil), opts.FitData...),
	}
	zipBytes, err := zipArtifacts(files)
	if err != nil {
		return nil, fmt.Errorf("create zip: %w", err)
	}

	fileNames := make([]string, 0, len(files))
	for name := range files {
		fileNames = append(fileNames, name)
	}
	sort.Strings(fileNames)

	return &RacePlanResult{
		Plan:            plan,
		SummaryMarkdown: summaryMD,
		Warnings:        append([]string(nil), plan.Warnings...),
		Files:           files,
		ArtifactNames:   fileNames,
		Zip:             zipBytes,
	}, nil
}

func zipArtifacts(files map[string][]byte) ([]byte, error) {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	fixedTime := time.Unix(0, 0).UTC()

	for _, name := range names {
		h := &zip.FileHeader{
			Name:   name,
			Method: zip.Deflate,
		}
		h.SetModTime(fixedTime)
		w, err := zw.CreateHeader(h)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(files[name]); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
