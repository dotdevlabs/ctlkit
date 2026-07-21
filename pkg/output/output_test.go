package output_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/dotdevlabs/ctlkit/pkg/clierror"
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

func TestRenderTable(t *testing.T) {
	var out, errOut bytes.Buffer
	r := output.New(false, "", &out, &errOut)
	cols := []output.Column{{Header: "NAME"}, {Header: "STATUS"}}
	rows := [][]string{{"task-001", "done"}, {"task-002", "pending"}}
	if err := r.Render(cols, rows, nil); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "NAME") {
		t.Errorf("output missing header NAME: %q", got)
	}
	if !strings.Contains(got, "task-001") {
		t.Errorf("output missing row data: %q", got)
	}
	if !strings.Contains(got, "─") {
		t.Errorf("output missing separator: %q", got)
	}
}

func TestRenderJSON(t *testing.T) {
	var out, errOut bytes.Buffer
	r := output.New(true, "", &out, &errOut)

	type Item struct {
		ID int `json:"id"`
	}
	envelope := map[string]any{
		"data":       []Item{{ID: 1}},
		"pagination": map[string]int{"total": 1, "page": 1, "per_page": 10},
	}
	if err := r.Render(nil, nil, envelope); err != nil {
		t.Fatal(err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v\ngot: %s", err, out.String())
	}
	if _, ok := decoded["data"]; !ok {
		t.Errorf("output missing 'data' key: %q", out.String())
	}
}

func TestRenderTemplate(t *testing.T) {
	var out, errOut bytes.Buffer
	r := output.New(false, "{{.Data}}", &out, &errOut)

	type Envelope struct {
		Data string
	}
	if err := r.Render(nil, nil, Envelope{Data: "hello"}); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "hello" {
		t.Errorf("template output = %q, want %q", got, "hello")
	}
}

func TestRenderInvalidTemplate(t *testing.T) {
	var out, errOut bytes.Buffer
	r := output.New(false, "{{.Unclosed", &out, &errOut)
	err := r.Render(nil, nil, nil)
	if err == nil {
		t.Fatal("expected error for invalid template")
	}
	ce, ok := err.(*clierror.CLIError)
	if !ok {
		t.Fatalf("expected CLIError, got %T", err)
	}
	if ce.Code != clierror.CodeUsage {
		t.Errorf("code = %d, want CodeUsage", ce.Code)
	}
}

func TestRendererDiag(t *testing.T) {
	var out, errOut bytes.Buffer
	r := output.New(false, "", &out, &errOut)
	r.Diag("info: %s", "test message")
	if out.Len() != 0 {
		t.Errorf("Diag wrote to Out, expected ErrOut only")
	}
	if !strings.Contains(errOut.String(), "test message") {
		t.Errorf("Diag output = %q", errOut.String())
	}
}

func TestNewJSONPrecedenceOverFormat(t *testing.T) {
	var out, errOut bytes.Buffer
	r := output.New(true, "{{.Data}}", &out, &errOut)
	if r.Mode != output.ModeJSON {
		t.Errorf("mode = %d, want ModeJSON", r.Mode)
	}
	if !strings.Contains(errOut.String(), "precedence") {
		t.Errorf("expected warning about precedence, got: %q", errOut.String())
	}
}
