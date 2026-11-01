package root_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/dotdevlabs/ctlkit/pkg/config"
	"github.com/dotdevlabs/ctlkit/pkg/ctxutil"
	"github.com/dotdevlabs/ctlkit/pkg/root"
	"github.com/dotdevlabs/ctlkit/pkg/version"
)

const defaultURL = "https://api.example.com"

func buildCfg(url string, cmds ...*cobra.Command) root.BuildConfig {
	return root.BuildConfig{
		Product:        "testctl",
		DefaultBaseURL: url,
		Short:          "Test CLI",
		Version:        version.Current("testctl"),
		Commands:       cmds,
	}
}

func TestBuildEmptyDefaultFails(t *testing.T) {
	_, err := root.Build(buildCfg(""))
	if err == nil || !strings.Contains(err.Error(), "testctl") || !strings.Contains(err.Error(), "required") {
		t.Fatalf("expected error naming product, got %v", err)
	}
}

func TestBuildInvalidDefaultFails(t *testing.T) {
	for _, u := range []string{"http://x.example.com", "ftp://x", "example.com", "https://", "https:// bad", "%zz"} {
		_, err := root.Build(buildCfg(u))
		if err == nil || !strings.Contains(err.Error(), "testctl") {
			t.Errorf("Build(%q) err = %v, want invalid error naming product", u, err)
		}
	}
}

func TestBuildValidDefault(t *testing.T) {
	if _, err := root.Build(buildCfg(defaultURL)); err != nil {
		t.Fatal(err)
	}
}

func TestNewPanicsOnEmptyDefault(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	root.New(buildCfg(""))
}

// runProbe executes a probe command and returns the active context it saw.
func runProbe(t *testing.T, args ...string) *config.Context {
	t.Helper()
	var captured context.Context
	probe := &cobra.Command{Use: "probe", RunE: func(cmd *cobra.Command, _ []string) error {
		captured = cmd.Context()
		return nil
	}}
	r := root.New(buildCfg(defaultURL, probe))
	r.SetOut(&bytes.Buffer{})
	r.SetErr(&bytes.Buffer{})
	r.SetArgs(append(args, "probe"))
	if err := r.Execute(); err != nil {
		t.Fatal(err)
	}
	return ctxutil.ActiveContextFrom(captured)
}

func cleanEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{"CONTEXT", "BASE_URL", "TOKEN"} {
		t.Setenv("TESTCTL_"+k, "")
	}
}

func TestExecuteDefaultsWithNoContextNoEnv(t *testing.T) {
	cleanEnv(t)
	t.Setenv("HOME", t.TempDir())
	if ac := runProbe(t); ac.BaseURL != defaultURL {
		t.Errorf("BaseURL = %q", ac.BaseURL)
	}
}

func TestExecuteBaseURLEnvOverridesDefault(t *testing.T) {
	cleanEnv(t)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("TESTCTL_BASE_URL", "https://env.example.com")
	if ac := runProbe(t); ac.BaseURL != "https://env.example.com" {
		t.Errorf("BaseURL = %q", ac.BaseURL)
	}
}

func TestExecuteContextFileWins(t *testing.T) {
	cleanEnv(t)
	writeRootConfig(t, "testctl", "current_context: dev\ncontexts:\n  dev:\n    base_url: https://dev.example.com\n    token: tok-dev\n")
	t.Setenv("TESTCTL_BASE_URL", "https://env.example.com")
	ac := runProbe(t)
	if ac.BaseURL != "https://dev.example.com" || ac.Token != "tok-dev" {
		t.Errorf("got %+v", ac)
	}
}

func TestExecuteTokenFlagAndEnv(t *testing.T) {
	cleanEnv(t)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("TESTCTL_TOKEN", "env-token")
	if ac := runProbe(t); ac.Token != "env-token" {
		t.Errorf("env token = %q", ac.Token)
	}
	if ac := runProbe(t, "--token", "flag-token"); ac.Token != "flag-token" {
		t.Errorf("flag token = %q", ac.Token)
	}
}
