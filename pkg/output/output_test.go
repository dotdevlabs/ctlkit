package output_test

import (
	"bytes"
	"testing"

	"github.com/dotdevlabs/ctlkit/pkg/output"
)

func TestJSONTo(t *testing.T) {
	var buf bytes.Buffer
	if err := output.JSONTo(&buf, map[string]string{"key": "value"}); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if got == "" {
		t.Fatal("expected non-empty output")
	}
}
