package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"fit-analyzer/llmexport"
)

func main() {
	var (
		outDir     = flag.String("out-dir", "", "Output directory for manifest.json and records.jsonl")
		overwrite  = flag.Bool("overwrite", true, "Allow writing to non-empty output directories")
		copySource = flag.Bool("copy-source", true, "Copy original FIT file into export directory as source.fit")
	)

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [flags] <path-to-fit-file>\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}

	inputPath := flag.Arg(0)
	if strings.TrimSpace(*outDir) == "" {
		base := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
		*outDir = filepath.Join(".", "exports", base+"_"+llmexport.ExportFormatVersion)
	}

	result, err := llmexport.ExportFile(inputPath, *outDir, llmexport.ExportOptions{
		Overwrite:      *overwrite,
		CopySourceFile: *copySource,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "export failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Export complete\n")
	fmt.Printf("Output dir: %s\n", result.OutputDir)
	fmt.Printf("Manifest:   %s\n", result.ManifestPath)
	fmt.Printf("Records:    %s\n", result.RecordsPath)
	if result.SourceCopyPath != "" {
		fmt.Printf("Source fit: %s\n", result.SourceCopyPath)
	}
	fmt.Printf("Records:    %d (%d definitions, %d data messages)\n", result.RecordCount, result.DefinitionCount, result.DataMessageCount)
	fmt.Printf("CRC valid:  header=%t file=%t\n", result.HeaderCRCValid, result.FileCRCValid)
}
