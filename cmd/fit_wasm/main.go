//go:build js && wasm

package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"sort"
	"syscall/js"
	"time"

	"fit-analyzer/pipeline"
)

func main() {
	js.Global().Set("analyzeFit", js.FuncOf(analyzeFit))
	select {}
}

func analyzeFit(_ js.Value, args []js.Value) any {
	if len(args) < 2 {
		return map[string]any{
			"ok":    false,
			"error": "expected arguments: fileBytes(Uint8Array), options(object)",
		}
	}
	fileArg := args[0]
	optsArg := args[1]
	if fileArg.IsUndefined() || fileArg.IsNull() || fileArg.Get("length").Int() == 0 {
		return map[string]any{
			"ok":    false,
			"error": "fit file bytes are required",
		}
	}

	fileBytes := make([]byte, fileArg.Get("length").Int())
	if n := js.CopyBytesToGo(fileBytes, fileArg); n == 0 {
		return map[string]any{
			"ok":    false,
			"error": "failed to read FIT bytes from JS input",
		}
	}

	opts := pipeline.BytesOptions{
		SourceFileName: getString(optsArg, "source_file_name", "input.fit"),
		FitData:        fileBytes,
		FTPOverride:    getFloat(optsArg, "ftp_w"),
		WeightKG:       getFloat(optsArg, "weight_kg"),
		Format:         getString(optsArg, "format", "parquet"),
		CopySource:     true,
	}
	result, err := pipeline.RunBytes(opts)
	if err != nil {
		return map[string]any{
			"ok":    false,
			"error": err.Error(),
		}
	}

	zipBytes, err := zipArtifacts(result.Files)
	if err != nil {
		return map[string]any{
			"ok":    false,
			"error": fmt.Sprintf("create zip: %v", err),
		}
	}
	payload := js.Global().Get("Uint8Array").New(len(zipBytes))
	js.CopyBytesToJS(payload, zipBytes)

	fileNames := make([]string, 0, len(result.Files))
	for name := range result.Files {
		fileNames = append(fileNames, name)
	}
	sort.Strings(fileNames)

	return map[string]any{
		"ok":       true,
		"zip":      payload,
		"warnings": stringsToAny(result.Warnings),
		"files":    stringsToAny(fileNames),
	}
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

func getString(v js.Value, key, fallback string) string {
	if v.IsUndefined() || v.IsNull() {
		return fallback
	}
	out := v.Get(key)
	if out.IsUndefined() || out.IsNull() {
		return fallback
	}
	s := out.String()
	if s == "" || s == "undefined" || s == "null" {
		return fallback
	}
	return s
}

func getFloat(v js.Value, key string) float64 {
	if v.IsUndefined() || v.IsNull() {
		return 0
	}
	out := v.Get(key)
	if out.IsUndefined() || out.IsNull() || out.Type() != js.TypeNumber {
		return 0
	}
	return out.Float()
}

func stringsToAny(values []string) []any {
	out := make([]any, len(values))
	for i, v := range values {
		out[i] = v
	}
	return out
}
