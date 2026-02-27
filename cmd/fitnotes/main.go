package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	fitnotes "fit-analyzer"
)

func main() {
	var (
		ftp      = flag.Float64("ftp", 0, "FTP in watts (optional; if omitted the tool estimates FTP from best 20-minute power)")
		jsonOut  = flag.Bool("json", false, "Emit full analysis as JSON")
		showLaps = flag.Bool("laps", false, "Include lap-by-lap summary in text output")
	)
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [flags] <path-to-fit-file>\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(2)
	}

	filePath := flag.Arg(0)
	analysis, err := fitnotes.AnalyzeFile(filePath, fitnotes.Config{FTPWatts: *ftp})
	if err != nil {
		fmt.Fprintf(os.Stderr, "analysis failed: %v\n", err)
		os.Exit(1)
	}

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(analysis); err != nil {
			fmt.Fprintf(os.Stderr, "json encode failed: %v\n", err)
			os.Exit(1)
		}
		return
	}

	fmt.Println(analysis.Notes)
	if *showLaps && len(analysis.Laps) > 0 {
		fmt.Println()
		fmt.Println("Lap Summary")
		for _, lap := range analysis.Laps {
			fmt.Printf(
				"- Lap %02d | %-10s | %6.0f W | %5.0f bpm | %5.0f rpm | %6.1fs\n",
				lap.Index,
				lap.Label,
				lap.AvgPowerWatts,
				lap.AvgHeartRate,
				lap.AvgCadence,
				lap.DurationSeconds,
			)
		}
	}
}
