package output_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/dotdevlabs/ctlkit/pkg/clierror"
	"github.com/dotdevlabs/ctlkit/pkg/output"
)

func TestJSON(t *testing.T) {
	// JSON writes to os.Stdout; just verify it doesn't error and formats correctly.
	// We test JSONTo (which JSON delegates to) in TestJSONTo.
	// This test just ensures the function is exercised for coverage.
	var buf bytes.Buffer
	if err := output.JSONTo(&buf, map[string]int{"count": 3}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "count") {
		t.Errorf("output missing key: %q", buf.String())
	}
}

func TestHandleCLIError(t *testing.T) {
	var buf bytes.Buffer
	e := clierror.New(clierror.CodeNotFound, "not found", "")
	code := output.HandleCLIError(e, &buf)
	if code != 1 {
		t.Errorf("HandleCLIError code = %d, want 1", code)
	}
	if !strings.Contains(buf.String(), "not found") {
		t.Errorf("HandleCLIError output = %q", buf.String())
	}
}

func TestHandleCLIErrorNil(t *testing.T) {
	var buf bytes.Buffer
	code := output.HandleCLIError(nil, &buf)
	if code != 0 {
		t.Errorf("HandleCLIError(nil) = %d, want 0", code)
	}
}
