package llmexport

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tormoder/fit"
)

// ExportFile parses a FIT file and writes an LLM-friendly, lossless export bundle.
// Output files:
//   - manifest.json
//   - records.jsonl
//   - source.fit (optional)
func ExportFile(inputPath, outputDir string, opts ExportOptions) (*ExportResult, error) {
	if strings.TrimSpace(inputPath) == "" {
		return nil, fmt.Errorf("input path is required")
	}
	if strings.TrimSpace(outputDir) == "" {
		return nil, fmt.Errorf("output directory is required")
	}

	data, err := os.ReadFile(inputPath)
	if err != nil {
		return nil, fmt.Errorf("read fit file: %w", err)
	}
	sum := sha256.Sum256(data)
	sha := hex.EncodeToString(sum[:])

	parsed, err := parseFITBytes(data)
	if err != nil {
		return nil, fmt.Errorf("parse fit file: %w", err)
	}

	if err := ensureOutputDir(outputDir, opts.Overwrite); err != nil {
		return nil, err
	}

	recordsPath := filepath.Join(outputDir, "records.jsonl")
	if err := writeJSONL(recordsPath, parsed.Records); err != nil {
		return nil, fmt.Errorf("write records.jsonl: %w", err)
	}

	fileID := projectFileID(inputPath)
	manifest := Manifest{
		FormatVersion:    ExportFormatVersion,
		GeneratedAt:      time.Now().UTC(),
		SourceFile:       inputPath,
		SourceFileName:   filepath.Base(inputPath),
		SourceSHA256:     sha,
		SourceSizeBytes:  int64(len(data)),
		Header:           parsed.Header,
		HeaderCRC:        parsed.HeaderCRC,
		FileCRC:          parsed.FileCRC,
		RecordsPath:      filepath.Base(recordsPath),
		RecordCount:      len(parsed.Records),
		DefinitionCount:  parsed.DefinitionCount,
		DataMessageCount: parsed.DataMessageCount,
		LeftoverBytes:    parsed.LeftoverBytesCount,
		FileIdProjection: fileID,
		SchemaDescription: SchemaDetails{
			RecordType: "JSONL line-per-FIT-record preserving original order and byte offsets",
			Notes: []string{
				"Lossless: every FIT data record and field payload is exported with raw hex.",
				"Each line includes decoded values and validity flags without dropping invalid sentinels.",
				"Developer data fields are preserved as raw bytes.",
				"Definition messages are preserved so unknown/global custom messages remain interpretable.",
				"Use record_index and file_offset for deterministic chunking in LLM pipelines.",
			},
		},
	}

	manifestPath := filepath.Join(outputDir, "manifest.json")
	if err := writeJSON(manifestPath, manifest); err != nil {
		return nil, fmt.Errorf("write manifest.json: %w", err)
	}

	sourceCopyPath := ""
	if opts.CopySourceFile {
		sourceCopyPath = filepath.Join(outputDir, "source.fit")
		if err := copyFile(inputPath, sourceCopyPath); err != nil {
			return nil, fmt.Errorf("copy source fit file: %w", err)
		}
	}

	return &ExportResult{
		OutputDir:         outputDir,
		ManifestPath:      manifestPath,
		RecordsPath:       recordsPath,
		SourceCopyPath:    sourceCopyPath,
		RecordCount:       len(parsed.Records),
		DefinitionCount:   parsed.DefinitionCount,
		DataMessageCount:  parsed.DataMessageCount,
		SourceSHA256:      sha,
		SourceSizeBytes:   int64(len(data)),
		FileCRCValid:      parsed.FileCRC.Valid,
		HeaderCRCValid:    parsed.HeaderCRC.Valid,
		ChainedDataRemain: parsed.LeftoverBytesCount,
	}, nil
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

func writeJSONL(path string, records []RecordEnvelope) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	buf := bufio.NewWriterSize(f, 1<<20)
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	for _, record := range records {
		if err := enc.Encode(record); err != nil {
			return err
		}
	}
	return buf.Flush()
}

func projectFileID(inputPath string) *FileIDInfo {
	f, err := os.Open(inputPath)
	if err != nil {
		return nil
	}
	defer f.Close()

	_, id, err := fit.DecodeHeaderAndFileID(f)
	if err != nil {
		return nil
	}
	info := &FileIDInfo{
		Type:         fmt.Sprint(id.Type),
		Manufacturer: fmt.Sprint(id.Manufacturer),
		Product:      fmt.Sprint(id.GetProduct()),
		SerialNumber: id.SerialNumber,
	}
	if !id.TimeCreated.IsZero() {
		info.TimeCreated = id.TimeCreated.UTC().Format(time.RFC3339)
	}
	return info
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}
