package k8s

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/net/http/httpproxy"
	"k8s.io/client-go/kubernetes"
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
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, failure(err)
	}
	return NewClient(cs, f.limit), nil
}

type Client struct {
	cs    kubernetes.Interface
	limit int64
}

func NewClient(cs kubernetes.Interface, limit int64) *Client { return &Client{cs: cs, limit: limit} }

func (c *Client) Version(ctx context.Context) (string, error) {
	v, err := c.cs.Discovery().ServerVersion()
	if err != nil {
		return "", failure(err)
	}
	_ = ctx
	return v.GitVersion, nil
}
