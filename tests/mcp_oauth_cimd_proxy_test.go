package tests

import (
	"context"
	"encoding/pem"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"golang.org/x/oauth2"
)

type tunnelProxy struct {
	srv   *httptest.Server
	mu    sync.Mutex
	hosts []string
}

func newTunnelProxy(t *testing.T, target string) *tunnelProxy {
	t.Helper()
	p := &tunnelProxy{}
	p.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodConnect {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		p.mu.Lock()
		p.hosts = append(p.hosts, r.Host)
		p.mu.Unlock()
		up, err := net.Dial("tcp", target)
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			_ = up.Close()
			return
		}
		_, _ = conn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
		go func() { _, _ = io.Copy(up, conn); _ = up.Close() }()
		_, _ = io.Copy(conn, up)
		_ = conn.Close()
	}))
	t.Cleanup(p.srv.Close)
	return p
}

func (p *tunnelProxy) seen() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string{}, p.hosts...)
}

func TestMCPOAuthClientMetadataThroughTheProxy(t *testing.T) {
	t.Parallel()
	docs := newDocumentServer(t)
	_, port, err := net.SplitHostPort(docs.srv.Listener.Addr().String())
	must(t, err)
	proxy := newTunnelProxy(t, docs.srv.Listener.Addr().String())
	ca := filepath.Join(t.TempDir(), "ca.pem")
	must(t, os.WriteFile(ca, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: docs.srv.Certificate().Raw}), 0o600))
	app := startApp(t, "MCP_OAUTH", "on", "PUBLIC_URL", oauthIssuer, "MCP_OAUTH_CLIENT_METADATA_USE_PROXY", "true",
		"MCP_OAUTH_CLIENT_METADATA_HOSTS", "example.com,127.0.0.1", "HTTPS_PROXY", proxy.srv.URL, "HTTP_CA_FILE", ca)

	app.startAuthorization("https://example.com:" + port + "/client.json")
	eq(t, docs.hits.Load(), int64(1))
	eq(t, strings.Join(proxy.seen(), ","), "example.com:"+port)

	eq(t, app.authorizeRefusal(docs.url("/client.json"), oauthRedirect), "oauth.client_metadata_unavailable")
	eq(t, docs.hits.Load(), int64(1), "a literal loopback address is refused before the proxy")
	eq(t, len(proxy.seen()), 1)
}

func TestMCPOAuthClientMetadataHostRemovedFromTheList(t *testing.T) {
	t.Parallel()
	docs := newDocumentServer(t)
	app := documentApp(t, docs)
	bob := bobSession(app)
	clientID := docs.url("/client.json")
	p := app.startAuthorization(clientID)
	q := app.approve(bob, p.requestID, nil)
	tok, err := p.cfg.Exchange(context.Background(), q.Get("code"), oauth2.VerifierOption(p.verifier),
		oauth2.SetAuthURLParam("resource", oauthResource))
	must(t, err)

	app.restart("MCP_OAUTH_CLIENT_METADATA_HOSTS", "")
	r := app.form("/api/oauth/token", url.Values{"grant_type": {"refresh_token"}, "refresh_token": {tok.RefreshToken}, "client_id": {clientID}})
	eq(t, r.status, 401, r.text())
	eq(t, at(r.json(t), "error"), any("invalid_client"))
	eq(t, app.authorizeRefusal(clientID, oauthRedirect), "oauth.client_not_allowed")
}
