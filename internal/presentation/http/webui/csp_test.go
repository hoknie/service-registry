package webui

import (
	"crypto/sha256"
	"encoding/base64"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

const themeScript = `(function(){try{var t=localStorage.getItem("svcr-theme");var d=t==="dark"||(t!=="light"&&matchMedia("(prefers-color-scheme: dark)").matches);var r=document.documentElement;r.dataset.theme=d?"dark":"light";r.style.colorScheme=d?"dark":"light"}catch(e){}})();(function(){try{var s=localStorage.getItem("svcr-sidebar");if(s==="expanded"||s==="collapsed")document.documentElement.dataset.sidebar=s}catch(e){}})()`

func TestInlineScriptHashes(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		html string
		want []string
	}{
		"no scripts":    {`<!doctype html><html><head><title>x</title></head><body></body></html>`, nil},
		"theme script":  {`<html><head><script>` + themeScript + `</script></head></html>`, []string{"PAW1urHP2OX5WM1AjbdHhnpR5YzgBadMZbcr4+ZdSew="}},
		"src script":    {`<script src="/_next/static/chunks/a.js" async=""></script>`, nil},
		"known text":    {`<script>alert(1)</script>`, []string{"bhHHL3z2vDgxUt0W3dWQOrprscmda2Y5pLsLg4GF+pI="}},
		"duplicates":    {`<script>alert(1)</script><p>x</p><script>alert(1)</script>`, []string{"bhHHL3z2vDgxUt0W3dWQOrprscmda2Y5pLsLg4GF+pI="}},
		"order":         {`<script>(self.__next_f=self.__next_f||[]).push([0])</script><script>alert(1)</script>`, []string{"OBTN3RiyCV4Bq7dFqZ5a2pAXjnCcCYeTJMO2I/LYKeo=", "bhHHL3z2vDgxUt0W3dWQOrprscmda2Y5pLsLg4GF+pI="}},
		"entities kept": {`<script>var a="&amp;<b>";</script>`, []string{hashOf(`var a="&amp;<b>";`)}},
		"escaped close": {`<script>var s="<\/script>";</script>`, []string{hashOf(`var s="<\/script>";`)}},
	} {
		got := inlineScriptHashes([]byte(tc.html))
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: %q, want %q", name, got, tc.want)
		}
	}
}

func TestPolicyForHasNoUnsafeScripts(t *testing.T) {
	t.Parallel()
	got := policyFor([]byte(`<script>alert(1)</script>`))
	want := "default-src 'self'; script-src 'self' 'sha256-bhHHL3z2vDgxUt0W3dWQOrprscmda2Y5pLsLg4GF+pI='; style-src 'self' 'unsafe-inline'; " +
		"img-src 'self' data: https:; font-src 'self'; connect-src 'self'; object-src 'none'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'"
	if got != want {
		t.Fatalf("policy %q", got)
	}
	if empty := policyFor(nil); !strings.Contains(empty, "script-src 'self';") {
		t.Fatalf("empty policy %q", empty)
	}
}

func TestPolicyForFileFollowsContent(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "page.html")
	if err := os.WriteFile(path, []byte(`<script>alert(1)</script>`), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := policyForFile(path); !strings.Contains(got, "bhHHL3z2vDgxUt0W3dWQOrprscmda2Y5pLsLg4GF+pI=") {
		t.Fatalf("first policy %q", got)
	}
	if err := os.WriteFile(path, []byte(`<script>alert(22)</script>`), 0o600); err != nil {
		t.Fatal(err)
	}
	later := time.Now().Add(time.Minute)
	if err := os.Chtimes(path, later, later); err != nil {
		t.Fatal(err)
	}
	got := policyForFile(path)
	if strings.Contains(got, "bhHHL3z2vDgxUt0W3dWQOrprscmda2Y5pLsLg4GF+pI=") || !strings.Contains(got, hashOf(`alert(22)`)) {
		t.Fatalf("policy after rewrite %q", got)
	}
}

func hashOf(s string) string {
	sum := sha256.Sum256([]byte(s))
	return base64.StdEncoding.EncodeToString(sum[:])
}
