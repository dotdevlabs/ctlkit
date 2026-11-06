package root

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/dotdevlabs/ctlkit/pkg/httpclient"
)

func checkContract(cmd *cobra.Command, cfg BuildConfig, baseURL, token string) {
	if cfg.ContractVersion == "" {
		return
	}
	for c := cmd; c.Parent() != nil; c = c.Parent() {
		if c.Parent().Parent() == nil {
			switch c.Name() {
			case "version", "ai", "help", "completion", "auth", "context":
				return
			}
		}
	}
	ctx, cancel := context.WithTimeout(cmd.Context(), 500*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/status", nil)
	if err != nil {
		return
	}
	req.Header.Set("Accept", "application/vnd.api+json")
	req.Header.Set("User-Agent", httpclient.BrowserUserAgent)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	client := &http.Client{Timeout: 500 * time.Millisecond, CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return
	}
	var status struct {
		Data struct {
			Attributes struct {
				ContractVersion string `json:"contract_version"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64*1024)).Decode(&status); err != nil {
		return
	}
	server := status.Data.Attributes.ContractVersion
	if server != "" && server != cfg.ContractVersion {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: CLI contract version %s differs from server contract version %s. Upgrade: %s\n", cfg.ContractVersion, server, cfg.UpgradeInstructions)
	}
}
