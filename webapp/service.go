package webapp

import (
	"archive/zip"
	"bytes"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/lucasjlepore/fit-analyzer/analyzer"
	"github.com/lucasjlepore/fit-analyzer/pipeline"
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
