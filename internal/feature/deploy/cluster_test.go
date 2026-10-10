package deploy

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

	"svc-registry/internal/feature/catalog"
	"svc-registry/internal/feature/forge"
)

func ptr[T any](v T) *T { return &v }

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
	token := &catalog.CredentialsInput{SecretID: ptr("0199c000-0000-7000-8000-000000000001")}
	ok := ClusterInput{Name: ptr(" prod-eu "), Environment: ptr("Production"), APIURL: ptr("https://k8s.example:6443/"),
		CAPEM: &ca, Credentials: token}
	s, creds, err := ValidateCluster(DefaultSettings(), false, ok)
	if err != nil || s.Name != "prod-eu" || s.Environment != "production" || *s.APIURL != "https://k8s.example:6443" ||
		s.IntervalSecs != 60 || !s.Enabled || creds == nil || creds.String() != "0199c000-0000-7000-8000-000000000001" {
		t.Fatalf("%+v %+v %v", s, creds, err)
	}
	inCluster, creds, err := ValidateCluster(DefaultSettings(), false, ClusterInput{Name: ptr("self"), Environment: ptr("staging"), InCluster: ptr(true)})
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
		{"http", with(func(c *ClusterInput) { c.APIURL = ptr("http://k8s") }), InvalidAPIURL},
		{"no address", with(func(c *ClusterInput) { c.APIURL = nil }), InvalidAPIURL},
		{"address in cluster", with(func(c *ClusterInput) { c.InCluster = ptr(true) }), InvalidAPIURL},
		{"credentials in cluster", with(func(c *ClusterInput) { c.InCluster = ptr(true); c.APIURL = nil }), forge.InvalidCredentials},
		{"no credentials", with(func(c *ClusterInput) { c.Credentials = nil }), forge.InvalidCredentials},
		{"inline token", with(func(c *ClusterInput) { c.Credentials = &catalog.CredentialsInput{Inline: true} }), catalog.InvalidCredentialsInline},
		{"bad ca", with(func(c *ClusterInput) { c.CAPEM = ptr("not pem") }), InvalidCA},
		{"namespace", with(func(c *ClusterInput) { c.Namespaces = ptr([]string{"Bad_NS"}) }), InvalidNamespaces},
		{"interval", with(func(c *ClusterInput) { c.IntervalSecs = ptr(int64(14)) }), InvalidPollInterval},
		{"rule", with(func(c *ClusterInput) { c.Rules = ptr([]Rule{{Label: "app", Project: "acme/{namespace}/UPPER"}}) }), InvalidClusterRules},
		{"rule label", with(func(c *ClusterInput) { c.Rules = ptr([]Rule{{Label: "bad label", Project: "acme"}}) }), InvalidClusterRules},
	}
	for _, tt := range tests {
		if _, _, err := ValidateCluster(DefaultSettings(), false, tt.in); !errors.Is(err, tt.want) {
			t.Errorf("%s: %v", tt.name, err)
		}
	}
	if _, _, err := ValidateCluster(DefaultSettings(), false, ClusterInput{Name: ptr("x"), Environment: ptr("p"), InCluster: ptr(true), APIURL: ptr("https://x")}); !errors.Is(err, InvalidAPIURL) {
		t.Fatal(err)
	}
	if _, creds, err := ValidateCluster(s, true, ClusterInput{IntervalSecs: ptr(int64(30))}); err != nil || creds != nil {
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
