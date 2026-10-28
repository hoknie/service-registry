package types

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"testing"
	"time"

	catalogtypes "svc-registry/internal/feature/catalog/types"
	reposynctypes "svc-registry/internal/feature/reposync/types"
)

func testCA(t *testing.T) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "k8s"}, NotBefore: time.Now(), NotAfter: time.Now().Add(time.Hour)}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}

func TestNewClusterRules(t *testing.T) {
	ca := testCA(t)
	token := &catalogtypes.CredentialsInput{SecretID: new("0199c000-0000-7000-8000-000000000001")}
	ok := ClusterInput{Name: new(" prod-eu "), Environment: new("Production"), APIURL: new("https://k8s.example:6443/"),
		CAPEM: &ca, Credentials: token}
	s, creds, err := ValidateCluster(DefaultSettings(), false, ok)
	if err != nil || s.Name != "prod-eu" || s.Environment != "production" || *s.APIURL != "https://k8s.example:6443" ||
		s.IntervalSecs != 60 || !s.Enabled || creds == nil || creds.String() != "0199c000-0000-7000-8000-000000000001" {
		t.Fatalf("%+v %+v %v", s, creds, err)
	}
	inCluster, creds, err := ValidateCluster(DefaultSettings(), false, ClusterInput{Name: new("self"), Environment: new("staging"), InCluster: new(true)})
	if err != nil || !inCluster.InCluster || creds != nil {
		t.Fatal(inCluster, err)
	}
	with := func(f func(*ClusterInput)) ClusterInput { c := ok; f(&c); return c }
	tests := []struct {
		name string
		in   ClusterInput
		want error
	}{
		{"no name", with(func(c *ClusterInput) { c.Name = nil }), InvalidClusterName},
		{"no environment", with(func(c *ClusterInput) { c.Environment = nil }), InvalidEnvironmentKey},
		{"http", with(func(c *ClusterInput) { c.APIURL = new("http://k8s") }), InvalidAPIURL},
		{"no address", with(func(c *ClusterInput) { c.APIURL = nil }), InvalidAPIURL},
		{"address in cluster", with(func(c *ClusterInput) { c.InCluster = new(true) }), InvalidAPIURL},
		{"credentials in cluster", with(func(c *ClusterInput) { c.InCluster = new(true); c.APIURL = nil }), reposynctypes.InvalidCredentials},
		{"no credentials", with(func(c *ClusterInput) { c.Credentials = nil }), reposynctypes.InvalidCredentials},
		{"inline token", with(func(c *ClusterInput) { c.Credentials = &catalogtypes.CredentialsInput{Inline: true} }), catalogtypes.InvalidCredentialsInline},
		{"bad ca", with(func(c *ClusterInput) { c.CAPEM = new("not pem") }), InvalidCA},
		{"namespace", with(func(c *ClusterInput) { c.Namespaces = new([]string{"Bad_NS"}) }), InvalidNamespaces},
		{"interval", with(func(c *ClusterInput) { c.IntervalSecs = new(int64(14)) }), InvalidPollInterval},
		{"rule", with(func(c *ClusterInput) { c.Rules = new([]Rule{{Label: "app", Project: "acme/{namespace}/UPPER"}}) }), InvalidClusterRules},
		{"rule label", with(func(c *ClusterInput) { c.Rules = new([]Rule{{Label: "bad label", Project: "acme"}}) }), InvalidClusterRules},
	}
	for _, tt := range tests {
		if _, _, err := ValidateCluster(DefaultSettings(), false, tt.in); !errors.Is(err, tt.want) {
			t.Errorf("%s: %v", tt.name, err)
		}
	}
	if _, _, err := ValidateCluster(DefaultSettings(), false, ClusterInput{Name: new("x"), Environment: new("p"), InCluster: new(true), APIURL: new("https://x")}); !errors.Is(err, InvalidAPIURL) {
		t.Fatal(err)
	}
	if _, creds, err := ValidateCluster(s, true, ClusterInput{IntervalSecs: new(int64(30))}); err != nil || creds != nil {
		t.Fatal(creds, err)
	}
	rules, err := ValidateRules([]Rule{{Label: "app.kubernetes.io/name", Project: "/acme/{namespace}/{label:app.kubernetes.io/name}/"}})
	if err != nil || rules[0].Project != "acme/{namespace}/{label:app.kubernetes.io/name}" {
		t.Fatal(rules, err)
	}
	ns, err := ValidateNamespaces([]string{" a ", "a", "b-1"})
	if err != nil || len(ns) != 2 {
		t.Fatal(ns, err)
	}
}

func TestAgentClusterRules(t *testing.T) {
	ca := testCA(t)
	agent, creds, err := ValidateCluster(DefaultSettings(), false, ClusterInput{Mode: new("agent"), Name: new("edge-1"), Environment: new("production")})
	if err != nil || agent.Mode != ModeAgent || agent.APIURL != nil || creds != nil {
		t.Fatalf("%+v %v %v", agent, creds, err)
	}
	polled, _, err := ValidateCluster(DefaultSettings(), false, ClusterInput{Name: new("prod"), Environment: new("p"),
		APIURL: new("https://k8s.example"), CAPEM: &ca, Credentials: &catalogtypes.CredentialsInput{SecretID: new("0199c000-0000-7000-8000-000000000001")}})
	if err != nil || polled.Mode != ModePoll {
		t.Fatal(polled, err)
	}
	switched, creds, err := ValidateCluster(polled, true, ClusterInput{Mode: new("agent")})
	if err != nil || switched.APIURL != nil || switched.CAPEM != nil || creds != nil {
		t.Fatalf("switch to agent: %+v %v", switched, err)
	}
	if _, _, err := ValidateCluster(agent, false, ClusterInput{Mode: new("poll")}); !errors.Is(err, InvalidAPIURL) {
		t.Fatalf("back to poll without address: %v", err)
	}
	base := ClusterInput{Mode: new("agent"), Name: new("x"), Environment: new("p")}
	with := func(f func(*ClusterInput)) ClusterInput { c := base; f(&c); return c }
	for name, tt := range map[string]struct {
		in   ClusterInput
		want error
	}{
		"unknown mode": {with(func(c *ClusterInput) { c.Mode = new("push") }), InvalidClusterMode},
		"address":      {with(func(c *ClusterInput) { c.APIURL = new("https://k8s.example:6443") }), InvalidAPIURL},
		"in cluster":   {with(func(c *ClusterInput) { c.InCluster = new(true) }), InvalidClusterMode},
		"ca":           {with(func(c *ClusterInput) { c.CAPEM = &ca }), InvalidCA},
		"credentials": {with(func(c *ClusterInput) {
			c.Credentials = &catalogtypes.CredentialsInput{SecretID: new("0199c000-0000-7000-8000-000000000001")}
		}), reposynctypes.InvalidCredentials},
	} {
		if _, _, err := ValidateCluster(DefaultSettings(), false, tt.in); !errors.Is(err, tt.want) {
			t.Errorf("%s: %v", name, err)
		}
	}
}
