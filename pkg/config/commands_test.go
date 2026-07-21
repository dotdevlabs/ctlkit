package config_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/dotdevlabs/ctlkit/pkg/config"
)

func setupTempHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	return home
}

func writeConfig(t *testing.T, home, product, content string) {
	t.Helper()
	dir := filepath.Join(home, ".config", "atmt")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, product+".yaml"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func executeCommand(root *cobra.Command, args ...string) (string, error) {
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}

func TestAuthLogin(t *testing.T) {
	home := setupTempHome(t)

	authCmd := config.AuthCommand("testctl")
	root := &cobra.Command{Use: "testctl"}
	root.AddCommand(authCmd)

	out, err := executeCommand(root,
		"auth", "login",
		"--name", "dev",
		"--url", "https://dev.example.com",
		"--token", "tok-dev",
	)
	if err != nil {
		t.Fatalf("auth login failed: %v\nout: %s", err, out)
	}
	if !strings.Contains(out, "dev") {
		t.Errorf("expected context name in output: %q", out)
	}

	// Verify config was written.
	cfg, err := config.Load("testctl")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Contexts["dev"].BaseURL != "https://dev.example.com" {
		t.Errorf("BaseURL = %q", cfg.Contexts["dev"].BaseURL)
	}
	_ = home
}

func TestAuthLoginSetCurrent(t *testing.T) {
	home := setupTempHome(t)
	writeConfig(t, home, "testctl", `
current_context: prod
contexts:
  prod:
    base_url: https://prod.example.com
    token: tok-prod
`)

	authCmd := config.AuthCommand("testctl")
	root := &cobra.Command{Use: "testctl"}
	root.AddCommand(authCmd)

	_, err := executeCommand(root,
		"auth", "login",
		"--name", "sandbox",
		"--url", "https://sandbox.example.com",
		"--token", "tok-sandbox",
		"--set-current",
	)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load("testctl")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.CurrentContext != "sandbox" {
		t.Errorf("CurrentContext = %q, want sandbox", cfg.CurrentContext)
	}
}

func TestContextList(t *testing.T) {
	home := setupTempHome(t)
	writeConfig(t, home, "testctl", `
current_context: prod
contexts:
  prod:
    base_url: https://prod.example.com
    token: tok-prod
`)

	ctxCmd := config.ContextCommand("testctl")
	root := &cobra.Command{Use: "testctl"}
	root.AddCommand(ctxCmd)

	out, err := executeCommand(root, "context", "list")
	if err != nil {
		t.Fatalf("context list failed: %v\nout: %s", err, out)
	}
	if !strings.Contains(out, "prod") {
		t.Errorf("expected prod in output: %q", out)
	}
	if !strings.Contains(out, "https://prod.example.com") {
		t.Errorf("expected URL in output: %q", out)
	}
}

func TestContextSelect(t *testing.T) {
	home := setupTempHome(t)
	writeConfig(t, home, "testctl", `
current_context: prod
contexts:
  prod:
    base_url: https://prod.example.com
    token: tok-prod
  sandbox:
    base_url: https://sandbox.example.com
    token: tok-sandbox
`)

	ctxCmd := config.ContextCommand("testctl")
	root := &cobra.Command{Use: "testctl"}
	root.AddCommand(ctxCmd)

	out, err := executeCommand(root, "context", "select", "sandbox")
	if err != nil {
		t.Fatalf("context select failed: %v\nout: %s", err, out)
	}
	if !strings.Contains(out, "sandbox") {
		t.Errorf("expected sandbox in output: %q", out)
	}

	cfg, err := config.Load("testctl")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.CurrentContext != "sandbox" {
		t.Errorf("CurrentContext = %q, want sandbox", cfg.CurrentContext)
	}
}

func TestContextSelectNotFound(t *testing.T) {
	home := setupTempHome(t)
	writeConfig(t, home, "testctl", `
current_context: prod
contexts:
  prod:
    base_url: https://prod.example.com
    token: tok-prod
`)

	ctxCmd := config.ContextCommand("testctl")
	root := &cobra.Command{Use: "testctl"}
	root.AddCommand(ctxCmd)

	_, err := executeCommand(root, "context", "select", "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent context")
	}
}

func TestSkipContextResolution(t *testing.T) {
	authCmd := config.AuthCommand("testctl")
	root := &cobra.Command{Use: "testctl"}
	root.AddCommand(authCmd)

	// Find the login subcommand and test skipContextResolution.
	var loginCmd *cobra.Command
	for _, sub := range authCmd.Commands() {
		if sub.Name() == "login" {
			loginCmd = sub
		}
	}
	if loginCmd == nil {
		t.Fatal("login subcommand not found")
	}

	// SkipContextResolution should return true for the login subcommand.
	if !config.SkipContextResolution(loginCmd) {
		t.Error("expected SkipContextResolution=true for auth login")
	}
}
