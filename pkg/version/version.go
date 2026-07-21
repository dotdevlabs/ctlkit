// Package version provides build info, a version command, and a self-update check.
package version

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/dotdevlabs/ctlkit/pkg/output"
)

// Build vars — populated via ldflags at build time.
//
//nolint:gochecknoglobals
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// Info carries version metadata for a product CLI.
type Info struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
	Product string `json:"product"`
}

// Current returns the version info for the given product.
func Current(product string) Info {
	return Info{
		Version: Version,
		Commit:  Commit,
		Date:    Date,
		Product: product,
	}
}

// NewCommand returns the "version" cobra.Command.
func NewCommand(product string, r *output.Renderer) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		RunE: func(cmd *cobra.Command, _ []string) error {
			info := Current(product)
			if r != nil && r.Mode == output.ModeJSON {
				return output.JSONTo(cmd.OutOrStdout(), info)
			}
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "%s version %s (commit %s, built %s)\n",
				info.Product, info.Version, info.Commit, info.Date)
			return err
		},
	}
}

// CheckUpdate fetches latestURL and compares the returned tag to currentVersion.
// Returns (latestVersion, isNewer, error). Never panics; on any error returns ("", false, err).
func CheckUpdate(ctx context.Context, currentVersion, latestURL string) (string, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, latestURL, nil)
	if err != nil {
		return "", false, err
	}
	req.Header.Set("Accept", "application/json")

	hc := &http.Client{Timeout: 5 * time.Second}
	resp, err := hc.Do(req)
	if err != nil {
		return "", false, err
	}
	defer func() { _ = resp.Body.Close() }()

	var payload struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", false, err
	}

	latest := strings.TrimPrefix(payload.TagName, "v")
	current := strings.TrimPrefix(currentVersion, "v")
	isNewer := latest != "" && latest != current && latest > current
	return payload.TagName, isNewer, nil
}
