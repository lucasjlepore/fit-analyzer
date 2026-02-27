package llmexport

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tormoder/fit"
)

// ParsedBundle is the in-memory representation of a decoded FIT stream.
type ParsedBundle struct {
	Header             HeaderInfo
	HeaderCRC          CRCCheck
	FileCRC            CRCCheck
	Records            []RecordEnvelope
	DefinitionCount    int
	DataMessageCount   int
	LeftoverBytesCount int64
	SourceSHA256       string
	SourceSizeBytes    int64
}

// ParseBytes parses raw FIT bytes into the same record model used by JSONL export.
func ParseBytes(data []byte) (*ParsedBundle, error) {
	parsed, err := parseFITBytes(data)
	if err != nil {
		return nil, fmt.Errorf("parse fit bytes: %w", err)
	}
	sum := sha256.Sum256(data)
	return &ParsedBundle{
		Header:             parsed.Header,
		HeaderCRC:          parsed.HeaderCRC,
		FileCRC:            parsed.FileCRC,
		Records:            parsed.Records,
		DefinitionCount:    parsed.DefinitionCount,
		DataMessageCount:   parsed.DataMessageCount,
		LeftoverBytesCount: parsed.LeftoverBytesCount,
		SourceSHA256:       hex.EncodeToString(sum[:]),
		SourceSizeBytes:    int64(len(data)),
	}, nil
}

// ProjectFileIDFromBytes returns the file_id projection directly from bytes.
func ProjectFileIDFromBytes(data []byte) *FileIDInfo {
	_, id, err := fit.DecodeHeaderAndFileID(bytes.NewReader(data))
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
		info.TimeCreated = id.TimeCreated.UTC().Format("2006-01-02T15:04:05Z")
	}
	return info
}

// MarshalJSON renders indented JSON with deterministic key order.
func MarshalJSON(v any) ([]byte, error) {
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}
	out = append(out, '\n')
	return out, nil
}

// MarshalJSONL renders record envelopes as JSONL bytes.
func MarshalJSONL(records []RecordEnvelope) ([]byte, error) {
	var buf bytes.Buffer
	w := bufio.NewWriterSize(&buf, 1<<20)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	for _, record := range records {
		if err := enc.Encode(record); err != nil {
			return nil, err
		}
	}
	if err := w.Flush(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// BuildWarningsFromBundle returns deterministic parse-quality warning notes.
func BuildWarningsFromBundle(bundle *ParsedBundle) []string {
	if bundle == nil {
		return nil
	}
	warnings := make([]string, 0, 4)
	if bundle.HeaderCRC.Present && !bundle.HeaderCRC.Valid {
		warnings = append(warnings, "header CRC mismatch")
	}
	if bundle.FileCRC.Present && !bundle.FileCRC.Valid {
		warnings = append(warnings, "file CRC mismatch")
	}
	if bundle.LeftoverBytesCount > 0 {
		warnings = append(warnings, fmt.Sprintf("leftover trailing bytes detected: %d", bundle.LeftoverBytesCount))
	}
	for _, rec := range bundle.Records {
		if len(rec.Warnings) == 0 {
			continue
		}
		for _, w := range rec.Warnings {
			if s := strings.TrimSpace(w); s != "" {
				warnings = append(warnings, s)
			}
		}
	}
	return dedupeStrings(warnings)
}

func dedupeStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, v := range values {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}
