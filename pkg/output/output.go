// Package output provides machine-stable JSON and human-readable rendering for *ctl CLIs.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/dotdevlabs/ctlkit/pkg/clierror"
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

// Mode describes the active output rendering mode.
type Mode int

const (
	ModeTable    Mode = iota // human-readable tabwriter table (default)
	ModeJSON                 // stable {data,pagination} envelope as JSON
	ModeTemplate             // Go-template projection of data
)

// Column describes one table column header.
// Column widths are byte-based (not rune-based); suitable for ASCII/English output.
type Column struct {
	Header string
}

// Renderer holds output configuration derived from global flags.
type Renderer struct {
	Mode     Mode
	Template string
	Out      io.Writer
	ErrOut   io.Writer
}

// New creates a Renderer from resolved global flag values.
// --json takes precedence over --format; if both are set a diagnostic is written.
func New(jsonFlag bool, formatFlag string, out, errOut io.Writer) *Renderer {
	r := &Renderer{Out: out, ErrOut: errOut}
	switch {
	case jsonFlag:
		r.Mode = ModeJSON
		if formatFlag != "" {
			_, _ = fmt.Fprintln(errOut, "warning: --json takes precedence over --format")
		}
	case formatFlag != "":
		r.Mode = ModeTemplate
		r.Template = formatFlag
	default:
		r.Mode = ModeTable
	}
	return r
}

// Render writes output in the active mode.
//   - cols/rows are used for ModeTable.
//   - envelope is used for ModeJSON and ModeTemplate (the {data,pagination} value).
func (r *Renderer) Render(cols []Column, rows [][]string, envelope any) error {
	switch r.Mode {
	case ModeJSON:
		return JSONTo(r.Out, envelope)
	case ModeTemplate:
		return renderTemplate(r.Out, r.Template, envelope)
	default:
		return renderTable(r.Out, cols, rows)
	}
}

// Diag writes a diagnostic message to ErrOut.
func (r *Renderer) Diag(format string, args ...any) {
	// Diagnostic writes to stderr; errors intentionally ignored.
	_, _ = fmt.Fprintf(r.ErrOut, format+"\n", args...)
}

// HandleCLIError renders err using HandleErr and returns the exit code.
func HandleCLIError(err error, w io.Writer) int {
	return clierror.HandleErr(err, w)
}
