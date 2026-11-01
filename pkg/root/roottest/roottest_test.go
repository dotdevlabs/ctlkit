package roottest_test

import (
	"testing"

	"github.com/dotdevlabs/ctlkit/pkg/root"
	"github.com/dotdevlabs/ctlkit/pkg/root/roottest"
	"github.com/dotdevlabs/ctlkit/pkg/version"
)

// recorder is a testing.TB that records Fatalf instead of aborting.
type recorder struct {
	testing.TB
	failed bool
}

func (r *recorder) Helper()               {}
func (r *recorder) Fatalf(string, ...any) { r.failed = true }
func (r *recorder) Setenv(k, v string)    { r.TB.Setenv(k, v) }
func (r *recorder) TempDir() string       { return r.TB.TempDir() }

func cfg(url string) root.BuildConfig {
	return root.BuildConfig{Product: "testctl", Short: "t", Version: version.Current("testctl"), DefaultBaseURL: url}
}

func TestAssertBuildsValid(t *testing.T) {
	roottest.AssertBuilds(t, cfg("https://api.example.com"))
}

func TestAssertBuildsReportsFailure(t *testing.T) {
	for _, u := range []string{"", "http://api.example.com", "nope"} {
		r := &recorder{TB: t}
		roottest.AssertBuilds(r, cfg(u))
		if !r.failed {
			t.Errorf("AssertBuilds(%q) did not report failure", u)
		}
	}
}
