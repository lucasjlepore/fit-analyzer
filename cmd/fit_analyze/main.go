package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"fit-analyzer/pipeline"
)

func main() {
	var (
		fitPath   = flag.String("fit", "", "Path to input .fit file")
		outDir    = flag.String("out", "", "Output directory")
		ftp       = flag.Float64("ftp", 0, "FTP override in watts")
		weightKG  = flag.Float64("weight", 0, "Athlete weight in kg")
		format    = flag.String("format", "parquet", "Canonical sample format: parquet|csv")
		overwrite = flag.Bool("overwrite", true, "Allow writing into non-empty output directories")
	)
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s --fit input.fit --out outdir [--ftp 223] [--weight 72.5] [--format parquet|csv]\n", filepath.Base(os.Args[0]))
		flag.PrintDefaults()
	}
	flag.Parse()

	if strings.TrimSpace(*fitPath) == "" || strings.TrimSpace(*outDir) == "" {
		flag.Usage()
		os.Exit(2)
	}

	result, err := pipeline.Run(pipeline.Options{
		FitPath:     *fitPath,
		OutDir:      *outDir,
		FTPOverride: *ftp,
		WeightKG:    *weightKG,
		Format:      *format,
		Overwrite:   *overwrite,
		CopySource:  true,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "fit_analyze failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("fit_analyze complete\n")
	fmt.Printf("Output dir:          %s\n", result.OutputDir)
	fmt.Printf("records.jsonl:       %s\n", result.RecordsPath)
	fmt.Printf("manifest.json:       %s\n", result.ManifestPath)
	fmt.Printf("canonical samples:   %s\n", result.CanonicalSamplesPath)
	fmt.Printf("messages index:      %s\n", result.MessagesIndexPath)
	fmt.Printf("workout structure:   %s\n", result.WorkoutStructurePath)
	if result.LapSummaryPath != "" {
		fmt.Printf("lap summary:         %s\n", result.LapSummaryPath)
	}
	fmt.Printf("activity summary:    %s\n", result.ActivitySummaryPath)
	if result.SourceCopyPath != "" {
		fmt.Printf("source copy:         %s\n", result.SourceCopyPath)
	}
	for _, w := range result.Warnings {
		fmt.Printf("warning:             %s\n", w)
	}
}
