// Package output provides machine-stable JSON and human-readable rendering for *ctl CLIs.
package output

import (
	"encoding/json"
	"io"
	"os"
)

// JSON encodes v as JSON to stdout.
func JSON(v any) error {
	return JSONTo(os.Stdout, v)
}

// JSONTo encodes v as JSON to w.
func JSONTo(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
