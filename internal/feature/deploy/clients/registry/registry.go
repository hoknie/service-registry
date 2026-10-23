package registry

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	deploytypes "svc-registry/internal/feature/deploy/types"
)

const answerLimit = 1 << 20

type RegistryClient struct {
	http      *http.Client
	base      string
	token     string
	userAgent string
}

func NewRegistryClient(hc *http.Client, address string, cluster uuid.UUID, token, version string) *RegistryClient {
	return &RegistryClient{http: hc, base: strings.TrimSuffix(address, "/") + "/api/v1/ingest/clusters/" + cluster.String(),
		token: token, userAgent: "svc-registry-agent/" + version}
}

func (c *RegistryClient) Config(ctx context.Context) (deploytypes.AgentConfig, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/config", nil)
	if err != nil {
		return deploytypes.AgentConfig{}, err
	}
	var out deploytypes.AgentConfig
	return out, c.do(req, &out)
}

func (c *RegistryClient) Report(ctx context.Context, r deploytypes.Report) (deploytypes.ReportAnswer, error) {
	pr, pw := io.Pipe()
	go func() { pw.CloseWithError(encodeReport(pw, r)) }()
	defer func() { _ = pr.Close() }()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/reports", pr)
	if err != nil {
		return deploytypes.ReportAnswer{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	var out deploytypes.ReportAnswer
	return out, c.do(req, &out)
}

func encodeReport(w io.Writer, r deploytypes.Report) error {
	zw := gzip.NewWriter(w)
	if err := json.NewEncoder(zw).Encode(r); err != nil {
		return err
	}
	return zw.Close()
}

func (c *RegistryClient) do(req *http.Request, out any) error {
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = res.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(res.Body, answerLimit))
	if err != nil {
		return err
	}
	if res.StatusCode == http.StatusUnauthorized {
		return deploytypes.ErrAgentRefused
	}
	if res.StatusCode != http.StatusOK {
		var apiErr struct {
			Code string `json:"code"`
		}
		_ = json.Unmarshal(raw, &apiErr)
		e := &deploytypes.RegistryError{Status: res.StatusCode, Code: apiErr.Code}
		if secs, err := strconv.Atoi(res.Header.Get("Retry-After")); err == nil && secs > 0 {
			e.RetryAfter = time.Duration(secs) * time.Second
		}
		return e
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("the registry answer is not JSON: %w", err)
	}
	return nil
}
