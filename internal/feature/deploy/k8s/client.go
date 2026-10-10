package k8s

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/http/httpproxy"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"

	"svc-registry/internal/feature/deploy"
	"svc-registry/internal/platform/config"
)

const userAgent = "svc-registry"

type Factory struct {
	proxy     func(*url.URL) (*url.URL, error)
	timeout   time.Duration
	limit     int64
	inCluster func() (*rest.Config, error)
}

func NewFactory(out config.OutboundConfig, k config.K8sConfig) *Factory {
	proxy := (&httpproxy.Config{HTTPSProxy: out.HTTPSProxy, HTTPProxy: out.HTTPProxy, NoProxy: out.NoProxy}).ProxyFunc()
	return &Factory{proxy: proxy, timeout: time.Duration(k.RequestTimeoutSecs) * time.Second,
		limit: int64(k.ListLimit), inCluster: rest.InClusterConfig}
}

func (f *Factory) Client(a deploy.Access) (deploy.ClusterClient, error) {
	var cfg *rest.Config
	if a.InCluster {
		c, err := f.inCluster()
		if err != nil {
			return nil, &deploy.Failure{Code: deploy.FailUnreachable, Message: "in-cluster config: " + err.Error()}
		}
		cfg = c
	} else {
		cfg = &rest.Config{Host: a.APIURL, BearerToken: a.Token, TLSClientConfig: rest.TLSClientConfig{CAData: a.CAPEM}}
	}
	cfg.Timeout = f.timeout
	cfg.UserAgent = userAgent
	cfg.Proxy = func(r *http.Request) (*url.URL, error) { return f.proxy(r.URL) }
	hc, err := rest.HTTPClientFor(cfg)
	if err != nil {
		return nil, failure(err)
	}
	dyn, err := dynamic.NewForConfigAndClient(cfg, hc)
	if err != nil {
		return nil, failure(err)
	}
	return NewClient(dyn, hc, cfg.Host, f.limit), nil
}

type Client struct {
	dyn   dynamic.Interface
	http  *http.Client
	host  string
	limit int64
}

func NewClient(dyn dynamic.Interface, hc *http.Client, host string, limit int64) *Client {
	return &Client{dyn: dyn, http: hc, host: strings.TrimRight(host, "/"), limit: limit}
}

func (c *Client) Version(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.host+"/version", nil)
	if err != nil {
		return "", failure(err)
	}
	req.Header.Set("Accept", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return "", failure(err)
	}
	defer func() { _ = res.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<16))
	if err != nil {
		return "", failure(err)
	}
	if res.StatusCode != http.StatusOK {
		return "", failure(statusError(res.StatusCode, body))
	}
	var v version.Info
	if err := json.Unmarshal(body, &v); err != nil {
		return "", failure(fmt.Errorf("decode /version: %w", err))
	}
	return v.GitVersion, nil
}

func statusError(code int, body []byte) error {
	msg := strings.TrimSpace(string(body))
	switch code {
	case http.StatusUnauthorized:
		return apierrors.NewUnauthorized(msg)
	case http.StatusForbidden:
		return apierrors.NewForbidden(schema.GroupResource{}, "", errors.New(msg))
	case http.StatusGatewayTimeout, http.StatusRequestTimeout:
		return apierrors.NewTimeoutError(msg, 0)
	}
	return fmt.Errorf("GET /version: HTTP %d: %s", code, msg)
}
