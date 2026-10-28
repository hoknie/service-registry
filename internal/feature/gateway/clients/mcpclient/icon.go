package mcpclient

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	gatewayports "svc-registry/internal/feature/gateway/ports"
	gatewaytypes "svc-registry/internal/feature/gateway/types"
)

const maxIcon = 64 << 10

var iconTypes = map[string]string{
	"image/png": "image/png", "image/jpeg": "image/jpeg", "image/gif": "image/gif", "image/webp": "image/webp",
	"image/svg+xml": "image/svg+xml", "image/x-icon": "image/x-icon", "image/vnd.microsoft.icon": "image/x-icon",
}

func (c *Client) icon(ctx context.Context, t gatewayports.Target, declared []sdk.Icon) *gatewaytypes.IconFile {
	for _, i := range declared {
		if f := c.iconFrom(ctx, t, i.Source, i.MIMEType); f != nil {
			return f
		}
	}
	u, err := url.Parse(t.URL)
	if err != nil {
		return nil
	}
	return c.iconFrom(ctx, t, (&url.URL{Scheme: u.Scheme, Host: u.Host, Path: "/favicon.ico"}).String(), "")
}

func (c *Client) iconFrom(ctx context.Context, t gatewayports.Target, src, declared string) *gatewaytypes.IconFile {
	if rest, ok := strings.CutPrefix(src, "data:"); ok {
		meta, payload, ok := strings.Cut(rest, ",")
		kind, isBase64 := strings.CutSuffix(meta, ";base64")
		if !ok || !isBase64 {
			return nil
		}
		data, err := base64.StdEncoding.DecodeString(payload)
		if err != nil {
			return nil
		}
		return iconOf(data, kind)
	}
	u, err := url.Parse(src)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return nil
	}
	same := u.Host == hostOf(t.URL)
	plain := t
	if !same {
		plain.AuthHeader, plain.AuthValue = "", ""
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil
	}
	resp, err := c.httpClient(plain).Do(req)
	if err != nil {
		return nil
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxIcon+1))
	if err != nil || len(data) > maxIcon {
		return nil
	}
	kind := resp.Header.Get("Content-Type")
	if kind == "" {
		kind = declared
	}
	return iconOf(data, kind)
}

func iconOf(data []byte, declared string) *gatewaytypes.IconFile {
	if len(data) == 0 || len(data) > maxIcon {
		return nil
	}
	kind, _, _ := mime.ParseMediaType(declared)
	if t, ok := iconTypes[kind]; ok && (t != "image/svg+xml" || looksSVG(data)) {
		return &gatewaytypes.IconFile{Type: t, Data: data}
	}
	if looksSVG(data) {
		return &gatewaytypes.IconFile{Type: "image/svg+xml", Data: data}
	}
	if t, ok := iconTypes[http.DetectContentType(data)]; ok {
		return &gatewaytypes.IconFile{Type: t, Data: data}
	}
	if bytes.HasPrefix(data, []byte{0, 0, 1, 0}) {
		return &gatewaytypes.IconFile{Type: "image/x-icon", Data: data}
	}
	return nil
}

func looksSVG(data []byte) bool {
	head := bytes.ToLower(bytes.TrimSpace(data[:min(len(data), 512)]))
	return bytes.HasPrefix(head, []byte("<svg")) || (bytes.HasPrefix(head, []byte("<?xml")) && bytes.Contains(head, []byte("<svg")))
}

func hostOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return u.Host
}
