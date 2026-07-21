package version_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dotdevlabs/ctlkit/pkg/output"
	"github.com/dotdevlabs/ctlkit/pkg/version"
)

func TestCurrentDefaults(t *testing.T) {
	info := version.Current("testctl")
	if info.Product != "testctl" {
		t.Errorf("Product = %q, want testctl", info.Product)
	}
	if info.Version == "" {
		t.Error("Version should not be empty")
	}
}

func TestNewCommandTableOutput(t *testing.T) {
	var out, errOut bytes.Buffer
	r := output.New(false, "", &out, &errOut)
	cmd := version.NewCommand("testctl", r)
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "testctl") {
		t.Errorf("output missing product name: %q", got)
	}
}

func TestNewCommandJSONOutput(t *testing.T) {
	var out, errOut bytes.Buffer
	r := output.New(true, "", &out, &errOut)
	cmd := version.NewCommand("testctl", r)
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var info version.Info
	if err := json.Unmarshal(out.Bytes(), &info); err != nil {
		t.Fatalf("output is not valid JSON: %v\ngot: %s", err, out.String())
	}
	if info.Product != "testctl" {
		t.Errorf("Product = %q", info.Product)
	}
}

func TestCheckUpdate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name":"v1.2.3"}`))
	}))
	defer srv.Close()

	tag, isNewer, err := version.CheckUpdate(context.Background(), "v0.1.0", srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if tag != "v1.2.3" {
		t.Errorf("tag = %q, want v1.2.3", tag)
	}
	if !isNewer {
		t.Error("expected isNewer = true")
	}
}

func TestCheckUpdateSameVersion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"tag_name":"v1.0.0"}`))
	}))
	defer srv.Close()

	_, isNewer, err := version.CheckUpdate(context.Background(), "v1.0.0", srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if isNewer {
		t.Error("expected isNewer = false for same version")
	}
}
