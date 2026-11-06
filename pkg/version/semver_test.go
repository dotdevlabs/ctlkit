package version

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCheckUpdateSemanticPrecedence(t *testing.T) {
	for _, tc := range []struct {
		current, latest string
		newer           bool
	}{
		{"0.0.9", "0.0.10", true}, {"0.9.0", "0.10.0", true}, {"0.0.10", "0.0.9", false}, {"0.10.0", "0.9.0", false},
		{"1.9.9", "v2.0.0", true}, {"v1.0.0", "1.0.0", false}, {"1.0.0-rc.9", "1.0.0-rc.10", true},
		{"1.0.0-rc", "1.0.0", true}, {"1.0.0", "1.0.0-rc", false}, {"1.0.0+one", "1.0.0+two", false},
		{"dev", "2.0.0", false}, {"1.0.0", "invalid", false}, {"1.0.0", "2.0.0-01", false},
		{"1.0.0-1", "1.0.0-alpha", true}, {"1.0.0-alpha", "1.0.0-1", false}, {"1.0.0-a", "1.0.0-b", true},
		{"1.0.0-a", "1.0.0-a.1", true}, {"1.0.0-10", "1.0.0-9", false},
	} {
		t.Run(tc.current+"/"+tc.latest, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = fmt.Fprintf(w, `{"tag_name":%q}`, tc.latest) }))
			defer srv.Close()
			tag, newer, err := CheckUpdate(context.Background(), tc.current, srv.URL)
			if err != nil || tag != tc.latest || newer != tc.newer {
				t.Fatalf("got %q %v %v", tag, newer, err)
			}
		})
	}
}
