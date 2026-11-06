package root_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dotdevlabs/ctlkit/pkg/ctxutil"
	"github.com/dotdevlabs/ctlkit/pkg/root"
	"github.com/spf13/cobra"
)

func TestContractAdvisory(t *testing.T) {
	for _, tc := range []struct {
		name, body        string
		code              int
		delay, warn, fail bool
	}{
		{"mismatch", `{"data":{"attributes":{"contract_version":"2"}}}`, 200, false, true, false},
		{"older", `{"data":{"attributes":{"contract_version":"0"}}}`, 200, false, true, false},
		{"failure unchanged", `{"data":{"attributes":{"contract_version":"2"}}}`, 200, false, true, true},
		{"match", `{"data":{"attributes":{"contract_version":"1"}}}`, 200, false, false, false},
		{"missing", `{"data":{"attributes":{}}}`, 200, false, false, false},
		{"null", `{"data":{"attributes":{"contract_version":null}}}`, 200, false, false, false},
		{"empty", `{"data":{"attributes":{"contract_version":""}}}`, 200, false, false, false},
		{"malformed", `{`, 200, false, false, false},
		{"error", ``, 500, false, false, false},
		{"timeout", ``, 200, true, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/product" {
					_, _ = fmt.Fprint(w, `{"ok":true}`)
					return
				}
				calls.Add(1)
				if r.URL.Path != "/status" || r.Header.Get("Authorization") != "Bearer secret" || r.Header.Get("Accept") != "application/vnd.api+json" || r.UserAgent() == "" {
					t.Error("incorrect status request")
				}
				if tc.delay {
					<-r.Context().Done()
					return
				}
				w.WriteHeader(tc.code)
				_, _ = fmt.Fprint(w, tc.body)
			}))
			defer srv.Close()
			writeRootConfig(t, "testctl", "")
			t.Setenv("TESTCTL_BASE_URL", srv.URL)
			t.Setenv("TESTCTL_TOKEN", "secret")
			failure := errors.New("product failure")
			probe := &cobra.Command{Use: "probe", RunE: func(cmd *cobra.Command, _ []string) error {
				if calls.Load() != 1 {
					t.Error("status must precede command exactly once")
				}
				if cmd.Context().Err() != nil {
					t.Error("command context canceled")
				}
				var data map[string]bool
				if err := ctxutil.ClientFrom(cmd.Context()).Get(cmd.Context(), "/product", &data); err != nil {
					return err
				}
				renderer := ctxutil.RendererFrom(cmd.Context())
				renderer.Out = cmd.OutOrStdout()
				if err := renderer.Render(nil, nil, data); err != nil {
					return err
				}
				if tc.fail {
					return failure
				}
				return nil
			}}
			parent := &cobra.Command{Use: "resource"}
			parent.AddCommand(probe)
			r := root.New(root.BuildConfig{Product: "testctl", DefaultBaseURL: "https://example.com", ContractVersion: "1", UpgradeInstructions: "brew upgrade testctl", Commands: []*cobra.Command{parent}})
			var out, stderr bytes.Buffer
			r.SetOut(&out)
			r.SetErr(&stderr)
			r.SilenceErrors = true
			r.SilenceUsage = true
			r.SetArgs([]string{"resource", "probe", "--json"})
			start := time.Now()
			err := r.Execute()
			if (tc.fail && !errors.Is(err, failure)) || (!tc.fail && err != nil) {
				t.Fatalf("result: %v", err)
			}
			if time.Since(start) > 2*time.Second {
				t.Fatal("probe exceeded bounded timeout")
			}
			if out.String() != "{\n  \"ok\": true\n}\n" || !json.Valid(out.Bytes()) {
				t.Fatalf("stdout changed: %s", out.String())
			}
			if tc.warn {
				if strings.Count(stderr.String(), "Warning:") != 1 || !strings.Contains(stderr.String(), "version 1") || !strings.Contains(stderr.String(), "brew upgrade testctl") || !strings.Contains(stderr.String(), "version "+map[bool]string{true: "0", false: "2"}[tc.name == "older"]) {
					t.Fatalf("warning: %s", stderr.String())
				}
			} else if stderr.Len() != 0 {
				t.Fatalf("unexpected warning: %s", stderr.String())
			}
		})
	}
}

func TestContractConfigurationAndLocalCommands(t *testing.T) {
	if _, err := root.Build(root.BuildConfig{Product: "testctl", DefaultBaseURL: "https://example.com", ContractVersion: "1"}); err == nil {
		t.Fatal("missing instructions accepted")
	}
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
	defer srv.Close()
	writeRootConfig(t, "testctl", "")
	t.Setenv("TESTCTL_BASE_URL", srv.URL)
	for _, args := range [][]string{{"version"}, {"ai", "--help"}, {"context", "list"}, {"completion", "bash"}, {"--help"}, {"probe"}} {
		cfg := root.BuildConfig{Product: "testctl", DefaultBaseURL: "https://example.com", Commands: []*cobra.Command{{Use: "probe", Run: func(_ *cobra.Command, _ []string) {}}}}
		if args[0] != "probe" {
			cfg.ContractVersion = "1"
			cfg.UpgradeInstructions = "upgrade"
		}
		r := root.New(cfg)
		r.SetOut(&bytes.Buffer{})
		r.SetErr(&bytes.Buffer{})
		r.SetArgs(args)
		if err := r.Execute(); err != nil {
			t.Fatal(err)
		}
	}
	if calls.Load() != 0 {
		t.Fatal("local or legacy command requested status")
	}
}

func TestContractRedirectAndUnavailable(t *testing.T) {
	var forwarded atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { forwarded.Add(1) }))
	defer target.Close()
	var requests atomic.Int32
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer redirect.Close()
	unavailable := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	unavailable.Close()
	writeRootConfig(t, "testctl", "")
	for _, url := range []string{redirect.URL, unavailable.URL} {
		t.Setenv("TESTCTL_BASE_URL", url)
		var ran bool
		r := root.New(root.BuildConfig{Product: "testctl", DefaultBaseURL: "https://example.com", ContractVersion: "1", UpgradeInstructions: "upgrade", Commands: []*cobra.Command{{Use: "probe", Run: func(_ *cobra.Command, _ []string) { ran = true }}}})
		var stderr bytes.Buffer
		r.SetErr(&stderr)
		r.SetArgs([]string{"probe"})
		if err := r.Execute(); err != nil || !ran || stderr.Len() != 0 {
			t.Fatalf("advisory affected command: %v %s", err, stderr.String())
		}
	}
	if requests.Load() != 1 || forwarded.Load() != 0 {
		t.Fatal("redirect followed or repeated")
	}
}
