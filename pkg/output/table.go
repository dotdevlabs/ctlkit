package output

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

func renderTable(w io.Writer, cols []Column, rows [][]string) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)

	// tabwriter buffers all writes and surfaces errors only at Flush().
	// Intermediate fmt.Fprintln errors are therefore captured by tw.Flush() below.

	headers := make([]string, len(cols))
	for i, c := range cols {
		headers[i] = c.Header
	}
	_, _ = fmt.Fprintln(tw, strings.Join(headers, "\t"))

	seps := make([]string, len(cols))
	for i, c := range cols {
		seps[i] = strings.Repeat("─", len(c.Header))
	}
	_, _ = fmt.Fprintln(tw, strings.Join(seps, "\t"))

	for _, row := range rows {
		_, _ = fmt.Fprintln(tw, strings.Join(row, "\t"))
	}

	return tw.Flush()
}
