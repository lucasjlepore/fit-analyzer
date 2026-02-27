# fit-analyzer

- Analyze `.fit` workout files and generate training notes.
- Export FIT files losslessly to an LLM-friendly format.
- Produce deterministic artifacts for downstream LLM analysis.
- Run analysis directly in-browser via GitHub Pages (WASM).

## Features

- Decode activity FIT files (Zwift/Strava/Garmin exports).
- Extract core metrics: time, distance, elevation, speed, power, HR, cadence, kJ.
- Compute derived metrics: normalized power (NP), variability index (VI), best 20 min power, IF/TSS (with FTP).
- Estimate FTP from data when not provided.
- Build FTP-based power zone distribution.
- Detect interval/recovery structure from lap data and assess execution trends.
- Generate coaching-style training notes from metrics.

## LLM Export Format (Best for LLM Pipelines)

This repo uses **JSONL (newline-delimited JSON)** as the primary export format for LLM workflows.

Why JSONL is used:

- Streamable and chunkable for embeddings/RAG pipelines.
- One FIT record per line, preserving order and byte offsets.
- Works well with large files without loading one giant JSON object.
- Deterministic and easy to append, index, and re-process.

Lossless export bundle output:

- `manifest.json`: metadata, checksums, schema version, and pointers.
- `records.jsonl`: every FIT definition/data record with raw hex + decoded values.
- `analysis.json`: session metrics and inferred interval labels.
- `workout_structure.json`: explicit block-level workout structure for LLM reasoning.
- `source.fit` (optional): source copy for provenance.

Schema version: `fit_llm_jsonl_v1`

## Usage

```bash
go run ./cmd/fitnotes /path/to/workout.fit
```

Optional flags:

```bash
go run ./cmd/fitnotes --ftp 260 --laps /path/to/workout.fit
go run ./cmd/fitnotes --json /path/to/workout.fit
```

Lossless LLM export:

```bash
go run ./cmd/fitllmexport --out-dir ./exports/my-workout /path/to/workout.fit
go run ./cmd/fitllmexport --ftp 223 --out-dir ./exports/my-workout /path/to/workout.fit
```

Deterministic analyzer pipeline:

```bash
go run ./cmd/fit_analyze --fit /path/to/workout.fit --out ./outputs/workout --ftp 223 --weight 72.5 --format parquet
```

`fit_analyze` outputs (additive to lossless JSONL):

- `canonical_samples.parquet` (or `.csv`)
- `messages_index.json`
- `workout_structure.json`
- `lap_summary.json` (if laps exist)
- `activity_summary.json`

`activity_summary.json` also includes:

- `weight_kg`
- `avg_power_w_per_kg`
- `np_w_per_kg`
- `max_power_w_per_kg`
- deterministic `warnings[]`

## Browser UI (GitHub Pages)

The UI is a static app in `web/` and runs analysis fully in browser with WASM.

Features:

- Drag/drop or browse `.fit` files.
- Required inputs: FTP (W) and weight (kg).
- Browser UI exports `canonical_samples` as CSV.
- Displays a markdown ride summary for copy/paste into notes or an LLM.
- Download one `.zip` containing all artifacts.

Note:

- Native CLI supports both `parquet` and `csv`.
- Browser/WASM mode uses CSV directly for compatibility.

Local web build:

```bash
./scripts/build-web.sh
```

This writes build artifacts to `web/dist/`.

GitHub Pages deploy:

- Workflow: `.github/workflows/pages.yml`
- Published URL: `https://lucasjlepore.github.io/fit-analyzer/`

## Library API

```go
analysis, err := fitnotes.AnalyzeFile("/path/to/workout.fit", fitnotes.Config{
    FTPWatts: 260, // optional
})
if err != nil {
    // handle
}

fmt.Println(analysis.Notes)
```

LLM export API:

```go
result, err := llmexport.ExportFile("/path/to/workout.fit", "./exports/my-workout", llmexport.ExportOptions{
    Overwrite:      true,
    CopySourceFile: true,
})
if err != nil {
    // handle
}
fmt.Println(result.ManifestPath, result.RecordsPath)
```

In-memory pipeline API (used by WASM UI):

```go
res, err := pipeline.RunBytes(pipeline.BytesOptions{
    SourceFileName: "workout.fit",
    FitData:        fitBytes,
    FTPOverride:    223,
    WeightKG:       72.5,
    Format:         "csv",
    CopySource:     true,
})
if err != nil {
    // handle
}
fmt.Println(len(res.Files), res.Warnings)
```
