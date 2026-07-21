package output

import (
	"fmt"
	"io"
	"text/template"

	"github.com/dotdevlabs/ctlkit/pkg/clierror"
)

func renderTemplate(w io.Writer, tmpl string, data any) error {
	t, err := template.New("output").Parse(tmpl)
	if err != nil {
		return clierror.New(clierror.CodeUsage, fmt.Sprintf("invalid template: %s", err), "")
	}
	if err := t.Execute(w, data); err != nil {
		return fmt.Errorf("executing template: %w", err)
	}
	return nil
}
