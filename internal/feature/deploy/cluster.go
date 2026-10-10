package deploy

import (
	"crypto/x509"
	"encoding/pem"
	"net/url"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"

	"svc-registry/internal/feature/forge"
)

const (
	Table             = "clusters"
	ColumnCredentials = "credentials_enc"
)

const (
	DefaultInterval = 60
	MinInterval     = 15
	MaxInterval     = 3600
	MaxNamespaces   = 100
	MaxRules        = 20
)

type Status string

const (
	StatusOK    Status = "ok"
	StatusError Status = "error"
	StatusNever Status = "never"
)

type Rule struct {
	Label   string `json:"label"`
	Project string `json:"project"`
}

type Settings struct {
	Name         string
	Environment  string
	InCluster    bool
	APIURL       *string
	CAPEM        *string
	Namespaces   []string
	Rules        []Rule
	IntervalSecs int32
	Enabled      bool
}

type Cluster struct {
	ID uuid.UUID
	Settings
	Credentials  *forge.Credentials
	Status       Status
	LastError    *Failure
	LastPolledAt *string
	NextPollAt   string
	Workloads    int64
	Unmatched    int64
	CreatedAt    string
	UpdatedAt    string
}

func DefaultSettings() Settings {
	return Settings{IntervalSecs: DefaultInterval, Enabled: true, Namespaces: []string{}, Rules: []Rule{}}
}

var (
	namespaceRe   = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)
	labelKeyRe    = regexp.MustCompile(`^(?:[a-z0-9](?:[a-z0-9.-]{0,251}[a-z0-9])?/)?[A-Za-z0-9](?:[A-Za-z0-9._-]{0,61}[A-Za-z0-9])?$`)
	placeholderRe = regexp.MustCompile(`\{(namespace|name|label:[^{}]+)\}`)
	slugCharsRe   = regexp.MustCompile(`^[a-z0-9-]+$`)
)

func ValidateClusterName(raw string) (string, error) {
	n := strings.TrimFunc(raw, unicode.IsSpace)
	if c := utf8.RuneCountInString(n); c < 1 || c > 100 || strings.IndexFunc(n, unicode.IsControl) >= 0 {
		return "", InvalidClusterName
	}
	return n, nil
}

func ValidateAPIURL(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	u, err := url.Parse(s)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || len(s) > 2048 ||
		u.RawQuery != "" || u.Fragment != "" {
		return "", InvalidAPIURL
	}
	return strings.TrimSuffix(s, "/"), nil
}

func ValidateCA(raw string) (string, error) {
	rest := []byte(strings.TrimSpace(raw))
	found := false
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		if _, err := x509.ParseCertificate(block.Bytes); err != nil {
			return "", InvalidCA
		}
		found = true
	}
	if !found {
		return "", InvalidCA
	}
	return strings.TrimSpace(raw) + "\n", nil
}

func ValidateNamespaces(raw []string) ([]string, error) {
	if len(raw) > MaxNamespaces {
		return nil, InvalidNamespaces
	}
	out := []string{}
	for _, n := range raw {
		n = strings.TrimSpace(n)
		if !namespaceRe.MatchString(n) {
			return nil, InvalidNamespaces
		}
		if !contains(out, n) {
			out = append(out, n)
		}
	}
	return out, nil
}

func ValidateRules(raw []Rule) ([]Rule, error) {
	if len(raw) > MaxRules {
		return nil, InvalidClusterRules
	}
	out := []Rule{}
	for _, r := range raw {
		r.Label = strings.TrimSpace(r.Label)
		r.Project = strings.Trim(strings.TrimSpace(r.Project), "/")
		if !labelKeyRe.MatchString(r.Label) || r.Project == "" || len(r.Project) > 1024 {
			return nil, InvalidClusterRules
		}
		for _, seg := range strings.Split(placeholderRe.ReplaceAllString(r.Project, "x"), "/") {
			if !slugCharsRe.MatchString(seg) {
				return nil, InvalidClusterRules
			}
		}
		out = append(out, r)
	}
	return out, nil
}

func ValidateInterval(raw int64) (int32, error) {
	if raw < MinInterval || raw > MaxInterval {
		return 0, InvalidPollInterval
	}
	return int32(raw), nil
}

func ValidateCluster(base Settings, hasCredentials bool, in ClusterInput) (Settings, *forge.ValidCredentials, error) {
	s := base
	var err error
	if in.Name != nil {
		if s.Name, err = ValidateClusterName(*in.Name); err != nil {
			return Settings{}, nil, err
		}
	}
	if in.Environment != nil {
		if s.Environment, err = EnvironmentKey(*in.Environment); err != nil {
			return Settings{}, nil, err
		}
	}
	if in.InCluster != nil {
		s.InCluster = *in.InCluster
		if s.InCluster {
			s.APIURL = nil
			hasCredentials = false
		}
	}
	if in.APIURL != nil {
		if *in.APIURL == "" {
			s.APIURL = nil
		} else {
			u, err := ValidateAPIURL(*in.APIURL)
			if err != nil {
				return Settings{}, nil, err
			}
			s.APIURL = &u
		}
	}
	if in.CAPEM != nil {
		if strings.TrimSpace(*in.CAPEM) == "" {
			s.CAPEM = nil
		} else {
			ca, err := ValidateCA(*in.CAPEM)
			if err != nil {
				return Settings{}, nil, err
			}
			s.CAPEM = &ca
		}
	}
	if in.Namespaces != nil {
		if s.Namespaces, err = ValidateNamespaces(*in.Namespaces); err != nil {
			return Settings{}, nil, err
		}
	}
	if in.Rules != nil {
		if s.Rules, err = ValidateRules(*in.Rules); err != nil {
			return Settings{}, nil, err
		}
	}
	if in.IntervalSecs != nil {
		if s.IntervalSecs, err = ValidateInterval(*in.IntervalSecs); err != nil {
			return Settings{}, nil, err
		}
	}
	if in.Enabled != nil {
		s.Enabled = *in.Enabled
	}
	if s.Name == "" {
		return Settings{}, nil, InvalidClusterName
	}
	if s.Environment == "" {
		return Settings{}, nil, InvalidEnvironmentKey
	}
	if s.InCluster {
		if s.APIURL != nil {
			return Settings{}, nil, InvalidAPIURL
		}
		if in.Credentials != nil {
			return Settings{}, nil, forge.InvalidCredentials
		}
		return s, nil, nil
	}
	if s.APIURL == nil {
		return Settings{}, nil, InvalidAPIURL
	}
	if in.Credentials == nil {
		if !hasCredentials {
			return Settings{}, nil, forge.InvalidCredentials
		}
		return s, nil, nil
	}
	creds, err := forge.ValidateCredentials(in.Credentials)
	if err != nil {
		return Settings{}, nil, err
	}
	return s, &creds, nil
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
