package webui

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"os"
	"strings"
	"sync"

	"golang.org/x/net/html"
)

const (
	cspScriptSelf = "default-src 'self'; script-src 'self'"
	cspRest       = "; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; font-src 'self'; connect-src 'self'; " +
		"object-src 'none'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'"
)

type cachedPolicy struct {
	etag   string
	policy string
}

var filePolicies sync.Map

func policyFor(body []byte) string {
	var b strings.Builder
	b.WriteString(cspScriptSelf)
	for _, h := range inlineScriptHashes(body) {
		b.WriteString(" 'sha256-")
		b.WriteString(h)
		b.WriteString("'")
	}
	b.WriteString(cspRest)
	return b.String()
}

func policyForFile(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return policyFor(nil)
	}
	tag := etag(info.Size(), info.ModTime(), "")
	if v, ok := filePolicies.Load(path); ok {
		if p := v.(cachedPolicy); p.etag == tag {
			return p.policy
		}
	}
	body, err := os.ReadFile(path) // #nosec G304 -- path is resolved inside the export root by lookup
	if err != nil {
		return policyFor(nil)
	}
	policy := policyFor(body)
	filePolicies.Store(path, cachedPolicy{etag: tag, policy: policy})
	return policy
}

func inlineScriptHashes(body []byte) []string {
	var hashes []string
	seen := map[string]bool{}
	z := html.NewTokenizer(bytes.NewReader(body))
	inline, inScript := false, false
	var text []byte
	flush := func() {
		if !inScript || !inline {
			return
		}
		sum := sha256.Sum256(text)
		h := base64.StdEncoding.EncodeToString(sum[:])
		if !seen[h] {
			seen[h] = true
			hashes = append(hashes, h)
		}
	}
	for {
		switch z.Next() {
		case html.ErrorToken:
			flush()
			return hashes
		case html.StartTagToken:
			name, hasAttr := z.TagName()
			if string(name) != "script" {
				continue
			}
			inScript, inline, text = true, !hasSrc(z, hasAttr), nil
		case html.TextToken:
			if inScript {
				text = append(text, z.Text()...)
			}
		case html.EndTagToken:
			if name, _ := z.TagName(); string(name) == "script" {
				flush()
				inScript = false
			}
		}
	}
}

func hasSrc(z *html.Tokenizer, more bool) bool {
	for more {
		var key []byte
		key, _, more = z.TagAttr()
		if string(key) == "src" {
			return true
		}
	}
	return false
}
