//go:build js

package pipeline

import "fmt"

func marshalCanonicalParquet(_ []CanonicalSample) ([]byte, error) {
	return nil, fmt.Errorf("parquet generation is not available in js/wasm runtime")
}
