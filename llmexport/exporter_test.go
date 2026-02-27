package llmexport

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tormoder/fit"
)

func TestParseFITBytesParsesRecords(t *testing.T) {
	data := buildTestFIT(t)

	out, err := parseFITBytes(data)
	if err != nil {
		t.Fatalf("parseFITBytes error: %v", err)
	}

	if out.Header.DataType != ".FIT" {
		t.Fatalf("unexpected header type: %q", out.Header.DataType)
	}
	if len(out.Records) == 0 {
		t.Fatal("expected records, got none")
	}
	if out.DefinitionCount == 0 {
		t.Fatal("expected at least one definition record")
	}
	if out.DataMessageCount == 0 {
		t.Fatal("expected at least one data record")
	}
	if !out.FileCRC.Valid {
		t.Fatal("expected valid file CRC")
	}
	if !out.HeaderCRC.Valid {
		t.Fatal("expected valid header CRC")
	}
}

func TestExportFileWritesBundle(t *testing.T) {
	data := buildTestFIT(t)

	tmp := t.TempDir()
	inputPath := filepath.Join(tmp, "sample.fit")
	if err := os.WriteFile(inputPath, data, 0o644); err != nil {
		t.Fatalf("write sample fit: %v", err)
	}

	outDir := filepath.Join(tmp, "export")
	result, err := ExportFile(inputPath, outDir, ExportOptions{
		Overwrite:      true,
		CopySourceFile: true,
	})
	if err != nil {
		t.Fatalf("ExportFile error: %v", err)
	}

	if result.RecordCount == 0 {
		t.Fatal("expected exported records")
	}
	if _, err := os.Stat(result.ManifestPath); err != nil {
		t.Fatalf("manifest missing: %v", err)
	}
	if _, err := os.Stat(result.RecordsPath); err != nil {
		t.Fatalf("records missing: %v", err)
	}
	if _, err := os.Stat(result.SourceCopyPath); err != nil {
		t.Fatalf("source copy missing: %v", err)
	}

	manifestData, err := os.ReadFile(result.ManifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var manifest Manifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}
	if manifest.FormatVersion != ExportFormatVersion {
		t.Fatalf("unexpected format version: %q", manifest.FormatVersion)
	}
	if manifest.RecordCount != result.RecordCount {
		t.Fatalf("manifest record count mismatch: %d != %d", manifest.RecordCount, result.RecordCount)
	}

	recordsData, err := os.ReadFile(result.RecordsPath)
	if err != nil {
		t.Fatalf("read records: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(recordsData)), "\n")
	if len(lines) != result.RecordCount {
		t.Fatalf("records line count mismatch: %d != %d", len(lines), result.RecordCount)
	}
}

func buildTestFIT(t *testing.T) []byte {
	t.Helper()

	header := fit.NewHeader(fit.V20, true)
	file, err := fit.NewFile(fit.FileTypeActivity, header)
	if err != nil {
		t.Fatalf("new fit file: %v", err)
	}

	activity, err := file.Activity()
	if err != nil {
		t.Fatalf("activity accessor: %v", err)
	}

	start := time.Date(2026, 2, 26, 23, 0, 0, 0, time.UTC)
	event := fit.NewEventMsg()
	event.Timestamp = start
	event.Event = fit.EventTimer
	event.EventType = fit.EventTypeStart
	activity.Events = append(activity.Events, event)

	stop := fit.NewEventMsg()
	stop.Timestamp = start.Add(10 * time.Minute)
	stop.Event = fit.EventTimer
	stop.EventType = fit.EventTypeStop
	activity.Events = append(activity.Events, stop)

	record := fit.NewRecordMsg()
	record.Timestamp = start.Add(30 * time.Second)
	record.HeartRate = 135
	record.Power = 245
	record.Cadence = 92
	activity.Records = append(activity.Records, record)

	var buf bytes.Buffer
	if err := fit.Encode(&buf, file, binary.LittleEndian); err != nil {
		t.Fatalf("encode fit: %v", err)
	}
	return buf.Bytes()
}
