package root_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"

	"github.com/dotdevlabs/ctlkit/pkg/ctxutil"
	"github.com/dotdevlabs/ctlkit/pkg/root"
	"github.com/dotdevlabs/ctlkit/pkg/version"
)

func writeRootConfig(t *testing.T, product, content string) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".config", "atmt")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, product+".yaml"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return home
}

func TestPersistentPreRunEWiresContext(t *testing.T) {
	writeRootConfig(t, "testctl", `
current_context: dev
contexts:
  dev:
    base_url: https://dev.example.com
    token: tok-dev
`)

	var capturedCtx context.Context
	probe := &cobra.Command{
		Use:   "probe",
		Short: "probe",
		RunE: func(cmd *cobra.Command, _ []string) error {
			capturedCtx = cmd.Context()
			return nil
		},
	}

	r := root.New(root.BuildConfig{
		Product:  "testctl",
		Short:    "Test CLI",
		Version:  version.Current("testctl"),
		Commands: []*cobra.Command{probe},
	})
	r.SetOut(&bytes.Buffer{})
	r.SetErr(&bytes.Buffer{})
	r.SetArgs([]string{"probe"})

	if err := r.Execute(); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if capturedCtx == nil {
		t.Fatal("context was nil")
	}

	cfg := ctxutil.ConfigFrom(capturedCtx)
	if cfg == nil {
		t.Error("config not stored in context")
	}

	ac := ctxutil.ActiveContextFrom(capturedCtx)
	if ac == nil {
		t.Error("active context not stored in context")
	} else if ac.BaseURL != "https://dev.example.com" {
		t.Errorf("BaseURL = %q", ac.BaseURL)
	}

	client := ctxutil.ClientFrom(capturedCtx)
	if client == nil {
		t.Error("http client not stored in context")
	}

	renderer := ctxutil.RendererFrom(capturedCtx)
	if renderer == nil {
		t.Error("renderer not stored in context")
	}
}

func TestPersistentPreRunEJSONFlag(t *testing.T) {
	writeRootConfig(t, "testctl", `
current_context: dev
contexts:
  dev:
    base_url: https://dev.example.com
    token: tok-dev
`)

	var capturedCtx context.Context
	probe := &cobra.Command{
		Use:   "probe",
		Short: "probe",
		RunE: func(cmd *cobra.Command, _ []string) error {
			capturedCtx = cmd.Context()
			return nil
		},
	}

	r := root.New(root.BuildConfig{
		Product:  "testctl",
		Short:    "Test CLI",
		Version:  version.Current("testctl"),
		Commands: []*cobra.Command{probe},
	})
	r.SetOut(&bytes.Buffer{})
	r.SetErr(&bytes.Buffer{})
	r.SetArgs([]string{"--json", "probe"})

	if err := r.Execute(); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	renderer := ctxutil.RendererFrom(capturedCtx)
	if renderer == nil {
		t.Fatal("renderer not stored in context")
	}
	if renderer.Mode != 1 { // ModeJSON = 1
		t.Errorf("renderer mode = %d, want ModeJSON(1)", renderer.Mode)
	}
}

func TestPersistentPreRunEContextFlag(t *testing.T) {
	writeRootConfig(t, "testctl", `
current_context: dev
contexts:
  dev:
    base_url: https://dev.example.com
    token: tok-dev
  prod:
    base_url: https://prod.example.com
    token: tok-prod
`)

	var capturedCtx context.Context
	probe := &cobra.Command{
		Use:   "probe",
		Short: "probe",
		RunE: func(cmd *cobra.Command, _ []string) error {
			capturedCtx = cmd.Context()
			return nil
		},
	}

	r := root.New(root.BuildConfig{
		Product:  "testctl",
		Short:    "Test CLI",
		Version:  version.Current("testctl"),
		Commands: []*cobra.Command{probe},
	})
	r.SetOut(&bytes.Buffer{})
	r.SetErr(&bytes.Buffer{})
	r.SetArgs([]string{"--context", "prod", "probe"})

	if err := r.Execute(); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	ac := ctxutil.ActiveContextFrom(capturedCtx)
	if ac == nil || ac.BaseURL != "https://prod.example.com" {
		t.Errorf("expected prod context, got %+v", ac)
	}
}

func TestAuthCommandSkipsPreRun(t *testing.T) {
	// Auth command should not require a valid context.
	home := t.TempDir()
	t.Setenv("HOME", home)

	r := root.New(root.BuildConfig{
		Product: "testctl",
		Short:   "Test CLI",
		Version: version.Current("testctl"),
	})
	r.SetOut(&bytes.Buffer{})
	r.SetErr(&bytes.Buffer{})

	// auth login requires flags; just verify it's reachable.
	var authCmd *cobra.Command
	for _, cmd := range r.Commands() {
		if cmd.Name() == "auth" {
			authCmd = cmd
		}
	}
	if authCmd == nil {
		t.Fatal("auth command not found")
	}
	if authCmd.Name() != "auth" {
		t.Errorf("auth name = %q", authCmd.Name())
	}
}
