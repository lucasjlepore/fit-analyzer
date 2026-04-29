//go:build js && wasm

package main

import (
	"syscall/js"

	"github.com/lucasjlepore/fit-analyzer/webapp"
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

	result, err := webapp.AnalyzeBytes(webapp.AnalyzeOptions{
		SourceFileName: getString(optsArg, "source_file_name", "input.fit"),
		FitData:        fileBytes,
		FTPWatts:       getFloat(optsArg, "ftp_w"),
		WeightKG:       getFloat(optsArg, "weight_kg"),
		Format:         getString(optsArg, "format", "csv"),
	})
	if err != nil {
		return map[string]any{
			"ok":    false,
			"error": err.Error(),
		}
	}
	payload := js.Global().Get("Uint8Array").New(len(result.Zip))
	js.CopyBytesToJS(payload, result.Zip)

	return map[string]any{
		"ok":            true,
		"zip":           payload,
		"summary_md":    result.SummaryMarkdown,
		"analysis_json": summaryString(result.Files["analysis.json"]),
		"warnings":      stringsToAny(result.Warnings),
		"files":         stringsToAny(result.ArtifactNames),
	}
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

func summaryString(content []byte) string {
	if len(content) == 0 {
		return ""
	}
	return string(content)
}
