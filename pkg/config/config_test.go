package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dotdevlabs/ctlkit/pkg/config"
)

func writeTemp(t *testing.T, product, content string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".config", "atmt")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, product+".yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadTwoContexts(t *testing.T) {
	writeTemp(t, "testctl", `
current_context: prod
contexts:
  prod:
    base_url: https://prod.example.com
    token: tok-prod
  sandbox:
    base_url: https://sandbox.example.com
    token: tok-sandbox
`)
	cfg, err := config.Load("testctl")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.CurrentContext != "prod" {
		t.Errorf("CurrentContext = %q, want %q", cfg.CurrentContext, "prod")
	}
	if len(cfg.Contexts) != 2 {
		t.Errorf("len(Contexts) = %d, want 2", len(cfg.Contexts))
	}
	if cfg.Contexts["prod"].BaseURL != "https://prod.example.com" {
		t.Errorf("prod BaseURL = %q", cfg.Contexts["prod"].BaseURL)
	}
}

func TestLoadMissingFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cfg, err := config.Load("nonexistent")
	if err != nil {
		t.Fatalf("expected no error for missing file, got: %v", err)
	}
	if cfg.CurrentContext != "" {
		t.Errorf("expected empty CurrentContext")
	}
}

func TestResolvByName(t *testing.T) {
	writeTemp(t, "testctl", `
current_context: prod
contexts:
  prod:
    base_url: https://prod.example.com
    token: tok-prod
  sandbox:
    base_url: https://sandbox.example.com
    token: tok-sandbox
`)
	cfg, _ := config.Load("testctl")
	ctx, name, err := config.Resolve("testctl", cfg, "sandbox")
	if err != nil {
		t.Fatal(err)
	}
	if name != "sandbox" {
		t.Errorf("name = %q, want sandbox", name)
	}
	if ctx.BaseURL != "https://sandbox.example.com" {
		t.Errorf("BaseURL = %q", ctx.BaseURL)
	}
}

func TestResolveCurrentContext(t *testing.T) {
	writeTemp(t, "testctl", `
current_context: prod
contexts:
  prod:
    base_url: https://prod.example.com
    token: tok-prod
`)
	cfg, _ := config.Load("testctl")
	ctx, name, err := config.Resolve("testctl", cfg, "")
	if err != nil {
		t.Fatal(err)
	}
	if name != "prod" {
		t.Errorf("name = %q, want prod", name)
	}
	if ctx.Token != "tok-prod" {
		t.Errorf("token = %q", ctx.Token)
	}
}

func TestResolveEnvContext(t *testing.T) {
	writeTemp(t, "testctl", `
current_context: prod
contexts:
  prod:
    base_url: https://prod.example.com
    token: tok-prod
  sandbox:
    base_url: https://sandbox.example.com
    token: tok-sandbox
`)
	t.Setenv("TESTCTL_CONTEXT", "sandbox")
	cfg, _ := config.Load("testctl")
	_, name, err := config.Resolve("testctl", cfg, "")
	if err != nil {
		t.Fatal(err)
	}
	if name != "sandbox" {
		t.Errorf("name = %q, want sandbox", name)
	}
}

func TestResolveTokenEnvOverride(t *testing.T) {
	writeTemp(t, "testctl", `
current_context: prod
contexts:
  prod:
    base_url: https://prod.example.com
    token: tok-prod
`)
	t.Setenv("TESTCTL_TOKEN", "env-token")
	cfg, _ := config.Load("testctl")
	ctx, _, err := config.Resolve("testctl", cfg, "")
	if err != nil {
		t.Fatal(err)
	}
	if ctx.Token != "env-token" {
		t.Errorf("token = %q, want env-token", ctx.Token)
	}
}

func TestResolveNoContext(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("TESTCTL_CONTEXT", "")

	cfg, _ := config.Load("testctl")
	cfg.CurrentContext = ""
	_, _, err := config.Resolve("testctl", cfg, "")
	if err == nil {
		t.Fatal("expected error for no context")
	}
}

func TestSaveRoundTrip(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cfg := &config.Config{
		CurrentContext: "dev",
		Contexts: map[string]config.Context{
			"dev": {BaseURL: "https://dev.example.com", Token: "tok-dev"},
		},
	}
	if err := config.Save("testctl", cfg); err != nil {
		t.Fatal(err)
	}
	loaded, err := config.Load("testctl")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.CurrentContext != "dev" {
		t.Errorf("CurrentContext = %q", loaded.CurrentContext)
	}
	if loaded.Contexts["dev"].Token != "tok-dev" {
		t.Errorf("token = %q", loaded.Contexts["dev"].Token)
	}
}
