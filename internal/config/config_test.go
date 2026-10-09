package config_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"svc-registry/internal/config"
)

const dbURL = "postgres://u:p@localhost:5440/registry"

func lookup(pairs ...string) config.Env {
	m := config.Env{}
	for i := 0; i+1 < len(pairs); i += 2 {
		m[pairs[i]] = pairs[i+1]
	}
	return m
}

func vars(t *testing.T, err error) []string {
	t.Helper()
	var errs config.Errors
	if !errors.As(err, &errs) {
		t.Fatalf("want config.Errors, got %v", err)
	}
	got := errs.Vars()
	sort.Strings(got)
	return got
}

func TestDefaultsApplyWhenOnlyRequiredVarsAreSet(t *testing.T) {
	cfg, err := config.Load[config.Config](lookup("DATABASE_URL", dbURL))
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.HTTP.Addr.String(); got != "0.0.0.0:8080" {
		t.Errorf("addr %s", got)
	}
	checks := []struct {
		name      string
		got, want any
	}{
		{"shutdown", cfg.HTTP.ShutdownTimeoutSecs, uint64(10)},
		{"max conns", cfg.DB.MaxConnections, uint32(10)},
		{"acquire", cfg.DB.AcquireTimeoutSecs, uint64(3)},
		{"dist", cfg.Web.DistDir, "web/out"},
		{"log", cfg.Log.Filter, "info"},
		{"secure", cfg.Session.CookieSecure, true},
		{"idle", cfg.Session.IdleTimeoutSecs, uint64(43_200)},
		{"absolute", cfg.Session.AbsoluteTimeoutSecs, uint64(604_800)},
		{"hash", cfg.PasswordHash, config.PasswordHashConfig{MemoryKiB: 19_456, Iterations: 2, Parallelism: 1}},
		{"max failures", cfg.LoginLimit.MaxFailures, uint32(5)},
		{"window", cfg.LoginLimit.WindowSecs, uint64(900)},
		{"retention", cfg.Ingest.RetentionDays, uint32(90)},
		{"rate", cfg.Ingest.RateLimitPerMinute, uint32(600)},
		{"pat lifetime", cfg.Pat.MaxLifetimeDays, uint32(365)},
		{"link check", cfg.LinkCheck, config.LinkCheckConfig{IntervalSecs: 3600, TimeoutSecs: 5, Concurrency: 4,
			AllowPrivate: true, History: 20, TargetTTLDays: 30}},
		{"k8s", cfg.K8s, config.K8sConfig{PollConcurrency: 2, RequestTimeoutSecs: 10, ListLimit: 500,
			HistoryConfirmSecs: 600, WorkloadRetentionDays: 7}},
		{"bootstrap", cfg.Bootstrap.Admin == nil, true},
	}
	for _, c := range checks {
		if !reflect.DeepEqual(c.got, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, c.got, c.want)
		}
	}
}

func TestMissingDatabaseURLIsReported(t *testing.T) {
	_, err := config.Load[config.Config](lookup())
	if got := vars(t, err); !reflect.DeepEqual(got, []string{"DATABASE_URL"}) {
		t.Fatalf("vars %v", got)
	}
	if !strings.Contains(err.Error(), "DATABASE_URL is required") {
		t.Fatal(err)
	}
}

func TestEveryInvalidVariableIsReportedAtOnce(t *testing.T) {
	_, err := config.Load[config.Config](lookup(
		"DATABASE_URL", "mysql://x",
		"HTTP_ADDR", "not-an-addr",
		"DATABASE_MAX_CONNECTIONS", "0",
		"SHUTDOWN_TIMEOUT_SECS", "999",
		"LOG_LEVEL", "loud",
	))
	want := []string{"DATABASE_MAX_CONNECTIONS", "DATABASE_URL", "HTTP_ADDR", "LOG_LEVEL", "SHUTDOWN_TIMEOUT_SECS"}
	if got := vars(t, err); !reflect.DeepEqual(got, want) {
		t.Fatalf("vars %v", got)
	}
}

func TestNonNumericValueNamesTheVariable(t *testing.T) {
	_, err := config.Load[config.DbConfig](lookup("DATABASE_URL", dbURL, "DATABASE_ACQUIRE_TIMEOUT_SECS", "soon"))
	if got := vars(t, err); !reflect.DeepEqual(got, []string{"DATABASE_ACQUIRE_TIMEOUT_SECS"}) {
		t.Fatalf("vars %v", got)
	}
	if !strings.Contains(err.Error(), "expected an integer") {
		t.Fatal(err)
	}
}

func TestWebDistDir(t *testing.T) {
	tests := []struct{ raw, want string }{
		{" /srv/web ", "/srv/web"},
		{"../ui/out", "../ui/out"},
		{"", "web/out"},
		{"  ", "web/out"},
	}
	for _, tt := range tests {
		web, err := config.Load[config.WebConfig](lookup("WEB_DIST_DIR", tt.raw))
		if err != nil || web.DistDir != tt.want {
			t.Errorf("%q: got %q, %v", tt.raw, web.DistDir, err)
		}
	}
}

func TestLogLevelAcceptsTargetDirectives(t *testing.T) {
	cfg, err := config.Load[config.Config](lookup("DATABASE_URL", dbURL, "LOG_LEVEL", "info,pgx=warn"))
	if err != nil || cfg.Log.Filter != "info,pgx=warn" {
		t.Fatalf("%q %v", cfg.Log.Filter, err)
	}
}

func TestSessionSettingsAreParsedAndChecked(t *testing.T) {
	s, err := config.Load[config.SessionConfig](lookup(
		"SESSION_COOKIE_SECURE", "false",
		"SESSION_IDLE_TIMEOUT_SECS", "600",
		"SESSION_ABSOLUTE_TIMEOUT_SECS", "3600",
	))
	if err != nil || s.CookieSecure || s.IdleTimeoutSecs != 600 || s.AbsoluteTimeoutSecs != 3600 {
		t.Fatalf("%+v %v", s, err)
	}
	_, err = config.Load[config.SessionConfig](lookup("SESSION_COOKIE_SECURE", "yes"))
	if got := vars(t, err); !reflect.DeepEqual(got, []string{"SESSION_COOKIE_SECURE"}) {
		t.Fatalf("vars %v", got)
	}
	_, err = config.Load[config.SessionConfig](lookup("SESSION_IDLE_TIMEOUT_SECS", "1"))
	if got := vars(t, err); !reflect.DeepEqual(got, []string{"SESSION_IDLE_TIMEOUT_SECS"}) {
		t.Fatalf("vars %v", got)
	}
}

func TestAbsoluteTimeoutShorterThanIdleIsRejected(t *testing.T) {
	_, err := config.Load[config.Config](lookup("DATABASE_URL", dbURL,
		"SESSION_IDLE_TIMEOUT_SECS", "7200", "SESSION_ABSOLUTE_TIMEOUT_SECS", "3600"))
	if got := vars(t, err); !reflect.DeepEqual(got, []string{"SESSION_ABSOLUTE_TIMEOUT_SECS"}) {
		t.Fatalf("vars %v", got)
	}
}

func TestBoundsOfNumericSections(t *testing.T) {
	tests := []struct {
		name  string
		parse func(config.Env) error
		valid []string
		bad   []string
		want  []string
	}{
		{
			name:  "password hash",
			parse: func(l config.Env) error { _, err := config.Load[config.PasswordHashConfig](l); return err },
			valid: []string{"PASSWORD_HASH_MEMORY_KIB", "65536", "PASSWORD_HASH_ITERATIONS", "3", "PASSWORD_HASH_PARALLELISM", "4"},
			bad:   []string{"PASSWORD_HASH_MEMORY_KIB", "512", "PASSWORD_HASH_ITERATIONS", "0", "PASSWORD_HASH_PARALLELISM", "64"},
			want:  []string{"PASSWORD_HASH_ITERATIONS", "PASSWORD_HASH_MEMORY_KIB", "PASSWORD_HASH_PARALLELISM"},
		},
		{
			name:  "login limit",
			parse: func(l config.Env) error { _, err := config.Load[config.LoginLimitConfig](l); return err },
			valid: []string{"LOGIN_MAX_FAILURES", "3", "LOGIN_FAILURE_WINDOW_SECS", "60"},
			bad:   []string{"LOGIN_MAX_FAILURES", "0", "LOGIN_FAILURE_WINDOW_SECS", "100000"},
			want:  []string{"LOGIN_FAILURE_WINDOW_SECS", "LOGIN_MAX_FAILURES"},
		},
		{
			name:  "ingest",
			parse: func(l config.Env) error { _, err := config.Load[config.IngestConfig](l); return err },
			valid: []string{"INGEST_EVENT_RETENTION_DAYS", "30", "INGEST_RATE_LIMIT_PER_MINUTE", "2"},
			bad:   []string{"INGEST_EVENT_RETENTION_DAYS", "0", "INGEST_RATE_LIMIT_PER_MINUTE", "100001"},
			want:  []string{"INGEST_EVENT_RETENTION_DAYS", "INGEST_RATE_LIMIT_PER_MINUTE"},
		},
		{
			name:  "personal access tokens",
			parse: func(l config.Env) error { _, err := config.Load[config.PatConfig](l); return err },
			valid: []string{"PAT_MAX_LIFETIME_DAYS", "3650"},
			bad:   []string{"PAT_MAX_LIFETIME_DAYS", "3651"},
			want:  []string{"PAT_MAX_LIFETIME_DAYS"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.parse(lookup(tt.valid...)); err != nil {
				t.Fatal(err)
			}
			if got := vars(t, tt.parse(lookup(tt.bad...))); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("vars %v", got)
			}
		})
	}
	p, _ := config.Load[config.PasswordHashConfig](lookup("PASSWORD_HASH_MEMORY_KIB", "65536", "PASSWORD_HASH_ITERATIONS", "3", "PASSWORD_HASH_PARALLELISM", "4"))
	if p != (config.PasswordHashConfig{MemoryKiB: 65_536, Iterations: 3, Parallelism: 4}) {
		t.Fatalf("%+v", p)
	}
	_, err := config.Load[config.Config](lookup("DATABASE_URL", dbURL, "INGEST_EVENT_RETENTION_DAYS", "0"))
	if got := vars(t, err); !reflect.DeepEqual(got, []string{"INGEST_EVENT_RETENTION_DAYS"}) {
		t.Fatalf("vars %v", got)
	}
	_, err = config.Load[config.Config](lookup("DATABASE_URL", dbURL, "PAT_MAX_LIFETIME_DAYS", "0"))
	if got := vars(t, err); !reflect.DeepEqual(got, []string{"PAT_MAX_LIFETIME_DAYS"}) {
		t.Fatalf("vars %v", got)
	}
}

func TestBootstrapAdminNeedsBothVariables(t *testing.T) {
	b, err := config.Load[config.BootstrapConfig](lookup("BOOTSTRAP_ADMIN_EMAIL", "admin@example.com", "BOOTSTRAP_ADMIN_PASSWORD", "bootstrap pass 1"))
	if err != nil || b.Admin == nil || b.Admin.Email != "admin@example.com" {
		t.Fatalf("%+v %v", b, err)
	}
	for _, format := range []string{"%v", "%+v", "%#v", "%s"} {
		if s := fmt.Sprintf(format, *b.Admin); strings.Contains(s, "bootstrap pass 1") {
			t.Errorf("%s leaks the password: %s", format, s)
		}
	}
	_, err = config.Load[config.Config](lookup("DATABASE_URL", dbURL, "BOOTSTRAP_ADMIN_EMAIL", "a@example.com"))
	if got := vars(t, err); !reflect.DeepEqual(got, []string{"BOOTSTRAP_ADMIN_PASSWORD"}) {
		t.Fatalf("vars %v", got)
	}
	_, err = config.Load[config.BootstrapConfig](lookup("BOOTSTRAP_ADMIN_PASSWORD", "x"))
	if got := vars(t, err); !reflect.DeepEqual(got, []string{"BOOTSTRAP_ADMIN_EMAIL"}) {
		t.Fatalf("vars %v", got)
	}
}

func TestUserPasswordIsReadUntrimmed(t *testing.T) {
	if p, _ := config.Load[config.UserPasswordConfig](lookup("USER_PASSWORD", "  ")); p.Password != "" {
		t.Fatal("blank counts as unset")
	}
	if p, _ := config.Load[config.UserPasswordConfig](lookup("USER_PASSWORD", " spaced pw ")); p.Password != " spaced pw " {
		t.Fatalf("%q", p.Password)
	}
}

func TestHTTPAddrForms(t *testing.T) {
	for raw, ok := range map[string]bool{"127.0.0.1:9000": true, "[::1]:8080": true, "nowhere": false, "localhost:80": false, "1.2.3.4": false} {
		_, err := config.Load[config.HTTPConfig](lookup("HTTP_ADDR", raw))
		if (err == nil) != ok {
			t.Errorf("%s: %v", raw, err)
		}
	}
}

func TestForgeSettingsDefaults(t *testing.T) {
	cfg, err := config.Load[config.Config](lookup("DATABASE_URL", dbURL))
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Jobs.Enabled || cfg.Jobs.ForgeConcurrency != 2 || cfg.Jobs.ForgeIntervalSecs != 900 || cfg.Jobs.ForgeRunsKept != 20 {
		t.Errorf("jobs %+v", cfg.Jobs)
	}
	if cfg.Outbound.TimeoutSecs != 30 || cfg.Outbound.CAPEM != nil || len(cfg.Secrets.Keys) != 0 || cfg.Web.PublicURL != "" {
		t.Errorf("outbound %+v secrets %v public %q", cfg.Outbound, cfg.Secrets, cfg.Web.PublicURL)
	}
}

func TestSecretsKeysAreParsedAndNeverEchoed(t *testing.T) {
	k1 := "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="
	s, err := config.Load[config.SecretsConfig](lookup("SECRETS_KEYS", "k2:"+k1+", k1:"+k1))
	if err != nil || len(s.Keys) != 2 || s.Keys[0].ID != "k2" || s.Keys[1].ID != "k1" {
		t.Fatalf("%v %v", s, err)
	}
	if strings.Contains(fmt.Sprintf("%v %#v %+v", s, s, s.Keys[0]), k1) {
		t.Error("key printed")
	}
	for _, bad := range []string{"k1:c2hvcnQ=", "nocolon", "K1:" + k1, "k1:" + k1 + ",k1:" + k1, "k1:%%%"} {
		_, err := config.Load[config.SecretsConfig](lookup("SECRETS_KEYS", bad))
		if got := vars(t, err); !reflect.DeepEqual(got, []string{"SECRETS_KEYS"}) {
			t.Fatalf("%q: %v", bad, got)
		}
		if strings.Contains(err.Error(), "c2hvcnQ") || strings.Contains(err.Error(), k1) {
			t.Errorf("%q: value in %v", bad, err)
		}
	}
}

func TestOutboundSettings(t *testing.T) {
	_, err := config.Load[config.OutboundConfig](lookup("HTTP_CA_FILE", "/nonexistent.pem"))
	if got := vars(t, err); !reflect.DeepEqual(got, []string{"HTTP_CA_FILE"}) {
		t.Fatalf("vars %v", got)
	}
	notPEM := filepath.Join(t.TempDir(), "x.pem")
	if err := os.WriteFile(notPEM, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = config.Load[config.OutboundConfig](lookup("HTTP_CA_FILE", notPEM))
	if got := vars(t, err); !reflect.DeepEqual(got, []string{"HTTP_CA_FILE"}) {
		t.Fatalf("vars %v", got)
	}
	o, err := config.Load[config.OutboundConfig](lookup("https_proxy", "http://lower:1", "NO_PROXY", "internal.example", "HTTP_PROXY", "http://upper:2", "http_proxy", "http://lower:2"))
	if err != nil || o.HTTPSProxy != "http://lower:1" || o.HTTPProxy != "http://upper:2" || o.NoProxy != "internal.example" {
		t.Fatalf("%+v %v", o, err)
	}
	_, err = config.Load[config.OutboundConfig](lookup("HTTPS_PROXY", "::not a url", "HTTP_CLIENT_TIMEOUT_SECS", "0"))
	if got := vars(t, err); !reflect.DeepEqual(got, []string{"HTTPS_PROXY", "HTTP_CLIENT_TIMEOUT_SECS"}) {
		t.Fatalf("vars %v", got)
	}
}

func TestPublicURL(t *testing.T) {
	w, err := config.Load[config.WebConfig](lookup("PUBLIC_URL", " https://registry.example/sub "))
	if err != nil || w.PublicURL != "https://registry.example/sub" {
		t.Fatalf("%q %v", w.PublicURL, err)
	}
	for _, bad := range []string{"registry.example", "ftp://x", "https://x/", "https://x?a=1", "https://u:p@x"} {
		_, err := config.Load[config.WebConfig](lookup("PUBLIC_URL", bad))
		if got := vars(t, err); !reflect.DeepEqual(got, []string{"PUBLIC_URL"}) {
			t.Errorf("%q: %v", bad, got)
		}
	}
}

func TestJobsBounds(t *testing.T) {
	_, err := config.Load[config.JobsConfig](lookup("FORGE_SYNC_CONCURRENCY", "0", "FORGE_SYNC_INTERVAL_SECS", "59",
		"FORGE_SYNC_RUNS_KEPT", "1001", "BACKGROUND_JOBS_ENABLED", "maybe"))
	want := []string{"BACKGROUND_JOBS_ENABLED", "FORGE_SYNC_CONCURRENCY", "FORGE_SYNC_INTERVAL_SECS", "FORGE_SYNC_RUNS_KEPT"}
	if got := vars(t, err); !reflect.DeepEqual(got, want) {
		t.Fatalf("vars %v", got)
	}
}

func TestSecretReferencesResolve(t *testing.T) {
	t.Setenv("SVCR_TEST_SECRET", " s3cret ")
	if v, err := config.ResolveSecretRef("env:SVCR_TEST_SECRET"); err != nil || v != "s3cret" {
		t.Fatalf("%q %v", v, err)
	}
	if _, err := config.ResolveSecretRef("env:SVCR_TEST_MISSING"); err == nil {
		t.Error("missing variable resolved")
	}
	file := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(file, []byte("tok-1\nignored\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if v, err := config.ResolveSecretRef("file:" + file); err != nil || v != "tok-1" {
		t.Fatalf("%q %v", v, err)
	}
	for _, bad := range []string{"file:/nonexistent", "vault:x", "plain"} {
		if _, err := config.ResolveSecretRef(bad); err == nil {
			t.Errorf("%q resolved", bad)
		}
	}
}

func TestBranchSettings(t *testing.T) {
	b, err := config.Load[config.BranchesConfig](lookup())
	if err != nil || b != (config.BranchesConfig{RetentionDays: 30, StaleDays: 90, SyncMaxPerRepo: 500}) {
		t.Fatalf("%+v %v", b, err)
	}
	_, err = config.Load[config.BranchesConfig](lookup("BRANCH_RETENTION_DAYS", "0", "BRANCH_STALE_DAYS", "3651", "BRANCH_SYNC_MAX_PER_REPO", "10001"))
	want := []string{"BRANCH_RETENTION_DAYS", "BRANCH_STALE_DAYS", "BRANCH_SYNC_MAX_PER_REPO"}
	if got := vars(t, err); !reflect.DeepEqual(got, want) {
		t.Fatalf("vars %v", got)
	}
}

func TestLinkCheckSettings(t *testing.T) {
	c, err := config.Load[config.LinkCheckConfig](lookup("LINK_CHECK_ALLOW_HOSTS", " *.Corp.example, grafana.internal ,",
		"LINK_CHECK_DENY_HOSTS", "evil.example", "LINK_CHECK_ALLOW_PRIVATE", "false"))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(c.AllowHosts, []string{"*.corp.example", "grafana.internal"}) || !reflect.DeepEqual(c.DenyHosts, []string{"evil.example"}) || c.AllowPrivate {
		t.Fatalf("%+v", c)
	}
	_, err = config.Load[config.LinkCheckConfig](lookup("LINK_CHECK_INTERVAL_SECS", "299", "LINK_CHECK_TIMEOUT_SECS", "0",
		"LINK_CHECK_CONCURRENCY", "65", "LINK_CHECK_HISTORY", "0", "LINK_CHECK_TARGET_TTL_DAYS", "3651",
		"LINK_CHECK_ALLOW_PRIVATE", "maybe", "LINK_CHECK_ALLOW_HOSTS", "*.", "LINK_CHECK_DENY_HOSTS", "bad host,*.ok.example"))
	want := []string{"LINK_CHECK_ALLOW_HOSTS", "LINK_CHECK_ALLOW_PRIVATE", "LINK_CHECK_CONCURRENCY", "LINK_CHECK_DENY_HOSTS",
		"LINK_CHECK_HISTORY", "LINK_CHECK_INTERVAL_SECS", "LINK_CHECK_TARGET_TTL_DAYS", "LINK_CHECK_TIMEOUT_SECS"}
	if got := vars(t, err); !reflect.DeepEqual(got, want) {
		t.Fatalf("vars %v", got)
	}
	_, err = config.Load[config.Config](lookup("DATABASE_URL", dbURL, "LINK_CHECK_DENY_HOSTS", "bad host,*.ok.example"))
	if got := vars(t, err); !reflect.DeepEqual(got, []string{"LINK_CHECK_DENY_HOSTS"}) {
		t.Fatalf("vars %v", got)
	}
}

func TestK8sSettings(t *testing.T) {
	if _, err := config.Load[config.K8sConfig](lookup("K8S_HISTORY_CONFIRM_SECS", "0", "K8S_LIST_LIMIT", "5000")); err != nil {
		t.Fatal(err)
	}
	_, err := config.Load[config.K8sConfig](lookup("K8S_POLL_CONCURRENCY", "0", "K8S_REQUEST_TIMEOUT_SECS", "121",
		"K8S_LIST_LIMIT", "49", "K8S_HISTORY_CONFIRM_SECS", "86401", "K8S_WORKLOAD_RETENTION_DAYS", "0"))
	want := []string{"K8S_HISTORY_CONFIRM_SECS", "K8S_LIST_LIMIT", "K8S_POLL_CONCURRENCY", "K8S_REQUEST_TIMEOUT_SECS", "K8S_WORKLOAD_RETENTION_DAYS"}
	if got := vars(t, err); !reflect.DeepEqual(got, want) {
		t.Fatalf("vars %v", got)
	}
	_, err = config.Load[config.Config](lookup("DATABASE_URL", dbURL, "K8S_POLL_CONCURRENCY", "0"))
	if got := vars(t, err); !reflect.DeepEqual(got, []string{"K8S_POLL_CONCURRENCY"}) {
		t.Fatalf("vars %v", got)
	}
}

func TestKnowledgeSettings(t *testing.T) {
	cfg, err := config.Load[config.KnowledgeConfig](lookup())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.IntervalSecs != 300 || cfg.RetrySecs != 900 || cfg.CollectConcurrency != 2 || cfg.Keep != 5 || cfg.ScanHistory != 20 ||
		cfg.MaxFileBytes != 524288 || cfg.MaxFiles != 2000 || cfg.MaxSnapshotBytes != 16777216 || cfg.MCPMaxResultBytes != 262144 {
		t.Fatalf("defaults %+v", cfg)
	}
	if _, err := config.Load[config.KnowledgeConfig](lookup("KNOWLEDGE_KEEP", "0", "KNOWLEDGE_MAX_FILES", "50000")); err != nil {
		t.Fatal(err)
	}
	_, err = config.Load[config.KnowledgeConfig](lookup("KNOWLEDGE_INTERVAL_SECS", "29", "KNOWLEDGE_RETRY_SECS", "59",
		"KNOWLEDGE_COLLECT_CONCURRENCY", "17", "KNOWLEDGE_KEEP", "101", "KNOWLEDGE_MAX_FILES", "0",
		"KNOWLEDGE_MAX_SNAPSHOT_BYTES", "1", "KNOWLEDGE_MCP_MAX_RESULT_BYTES", "4095", "KNOWLEDGE_SCAN_HISTORY", "0"))
	want := []string{"KNOWLEDGE_COLLECT_CONCURRENCY", "KNOWLEDGE_INTERVAL_SECS", "KNOWLEDGE_KEEP", "KNOWLEDGE_MAX_FILES",
		"KNOWLEDGE_MAX_SNAPSHOT_BYTES", "KNOWLEDGE_MCP_MAX_RESULT_BYTES", "KNOWLEDGE_RETRY_SECS", "KNOWLEDGE_SCAN_HISTORY"}
	if got := vars(t, err); !reflect.DeepEqual(got, want) {
		t.Fatalf("vars %v", got)
	}
	_, err = config.Load[config.Config](lookup("DATABASE_URL", dbURL, "KNOWLEDGE_MAX_FILE_BYTES", "100"))
	if got := vars(t, err); !reflect.DeepEqual(got, []string{"KNOWLEDGE_MAX_FILE_BYTES"}) {
		t.Fatalf("vars %v", got)
	}
}

func TestKnowledgeLocalRoots(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	cfg, err := config.Load[config.KnowledgeConfig](lookup())
	if err != nil || cfg.LocalRoots != nil {
		t.Fatalf("%v %v", cfg.LocalRoots, err)
	}
	cfg, err = config.Load[config.KnowledgeConfig](lookup("KNOWLEDGE_LOCAL_ROOTS", a+", "+b))
	if err != nil || len(cfg.LocalRoots) != 2 {
		t.Fatalf("%v %v", cfg.LocalRoots, err)
	}
	for _, bad := range []string{"docs", a + "/missing", a + "," + filepath.Join(a, "nope")} {
		_, err := config.Load[config.Config](lookup("DATABASE_URL", dbURL, "KNOWLEDGE_LOCAL_ROOTS", bad))
		if got := vars(t, err); !reflect.DeepEqual(got, []string{"KNOWLEDGE_LOCAL_ROOTS"}) {
			t.Fatalf("%s: %v", bad, got)
		}
	}
}

func TestOAuthProviders(t *testing.T) {
	cfg, err := config.Load[config.Config](lookup("DATABASE_URL", dbURL, "PUBLIC_URL", "https://registry.example",
		"OAUTH_PROVIDERS", "corp, gitlab", "OAUTH_CORP_KIND", "oidc", "OAUTH_CORP_ISSUER", "https://sso.example/realms/main/",
		"OAUTH_CORP_CLIENT_ID", "reg", "OAUTH_CORP_CLIENT_SECRET", "env:X", "OAUTH_CORP_DISPLAY_NAME", "Corporate SSO",
		"OAUTH_CORP_AUTO_PROVISION", "true", "OAUTH_CORP_ALLOWED_DOMAINS", "Example.com, @corp.example",
		"OAUTH_CORP_GROUP_MAP", "platform-team=>Platform; *",
		"OAUTH_GITLAB_KIND", "gitlab", "OAUTH_GITLAB_CLIENT_ID", "id", "OAUTH_GITLAB_CLIENT_SECRET", "s"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.OAuth.PasswordLogin != "all" || len(cfg.OAuth.Providers) != 2 {
		t.Fatalf("%+v", cfg.OAuth)
	}
	corp, gl := cfg.OAuth.Providers[0], cfg.OAuth.Providers[1]
	if corp.Issuer != "https://sso.example/realms/main" || corp.DisplayName != "Corporate SSO" || !corp.AutoProvision ||
		!reflect.DeepEqual(corp.AllowedDomains, []string{"example.com", "corp.example"}) || corp.GroupsClaim != "groups" ||
		!reflect.DeepEqual(corp.GroupMap, []config.GroupRule{{Value: "platform-team", Group: "Platform"}, {Value: "*"}}) ||
		!reflect.DeepEqual(corp.Scopes, []string{"openid", "profile", "email"}) {
		t.Fatalf("%+v", corp)
	}
	if gl.Issuer != "https://gitlab.com" || gl.DisplayName != "gitlab" || gl.GroupsClaim != "groups_direct" ||
		!reflect.DeepEqual(gl.Scopes, []string{"openid", "profile", "email", "read_user"}) {
		t.Fatalf("%+v", gl)
	}
	t.Setenv("SVCR_TEST_OAUTH_SECRET", "s3cret")
	if v, err := (config.OAuthProvider{ClientSecret: "env:SVCR_TEST_OAUTH_SECRET"}).ResolveSecret(); err != nil || v != "s3cret" {
		t.Fatal(v, err)
	}

	_, err = config.Load[config.Config](lookup("DATABASE_URL", dbURL, "OAUTH_PROVIDERS", "corp,Bad Key",
		"OAUTH_CORP_ISSUER", "http://sso.example", "OAUTH_CORP_SCOPES", "profile", "OAUTH_CORP_GROUP_MAP", "x=>",
		"OAUTH_CORP_AUTO_PROVISION", "maybe", "AUTH_PASSWORD_LOGIN", "some"))
	want := []string{"AUTH_PASSWORD_LOGIN", "OAUTH_CORP_AUTO_PROVISION", "OAUTH_CORP_CLIENT_ID", "OAUTH_CORP_CLIENT_SECRET",
		"OAUTH_CORP_GROUP_MAP", "OAUTH_CORP_ISSUER", "OAUTH_CORP_KIND", "OAUTH_CORP_SCOPES", "OAUTH_PROVIDERS", "PUBLIC_URL"}
	if got := vars(t, err); !reflect.DeepEqual(got, want) {
		t.Fatalf("vars %v", got)
	}
	_, err = config.Load[config.Config](lookup("DATABASE_URL", dbURL, "AUTH_PASSWORD_LOGIN", "off"))
	if got := vars(t, err); !reflect.DeepEqual(got, []string{"AUTH_PASSWORD_LOGIN"}) {
		t.Fatalf("vars %v", got)
	}
	_, err = config.Load[config.Config](lookup("DATABASE_URL", dbURL, "PUBLIC_URL", "https://r.example", "OAUTH_PROVIDERS", "corp"))
	if got := vars(t, err); !reflect.DeepEqual(got, []string{"OAUTH_CORP_CLIENT_ID", "OAUTH_CORP_CLIENT_SECRET", "OAUTH_CORP_ISSUER", "OAUTH_CORP_KIND"}) {
		t.Fatalf("vars %v", got)
	}
}

func TestSearchSettings(t *testing.T) {
	cfg, err := config.Load[config.SearchConfig](lookup())
	if err != nil || cfg.Engine != "postgres" || cfg.EmbeddingsBatch != 32 || cfg.ChunkChars != 1500 || cfg.Embeddings() {
		t.Fatalf("defaults %+v %v", cfg, err)
	}
	embed := []string{"EMBEDDINGS_URL", "http://ollama:11434/v1", "EMBEDDINGS_MODEL", "nomic-embed-text", "EMBEDDINGS_DIMENSIONS", "768"}
	for _, env := range [][]string{
		append([]string{"KNOWLEDGE_SEARCH_ENGINE", "pgvector"}, embed...),
		append([]string{"KNOWLEDGE_SEARCH_ENGINE", "qdrant", "QDRANT_URL", "qdrant:6334"}, embed...),
		{"KNOWLEDGE_SEARCH_ENGINE", "meilisearch", "MEILISEARCH_URL", "http://meili:7700"},
	} {
		if _, err := config.Load[config.SearchConfig](lookup(env...)); err != nil {
			t.Fatalf("%v: %v", env, err)
		}
	}
	for _, c := range []struct {
		env  []string
		want []string
	}{
		{[]string{"KNOWLEDGE_SEARCH_ENGINE", "milvus"}, []string{"KNOWLEDGE_SEARCH_ENGINE"}},
		{[]string{"KNOWLEDGE_SEARCH_ENGINE", "qdrant"}, []string{"EMBEDDINGS_URL", "QDRANT_URL"}},
		{[]string{"KNOWLEDGE_SEARCH_ENGINE", "pgvector"}, []string{"EMBEDDINGS_URL"}},
		{[]string{"KNOWLEDGE_SEARCH_ENGINE", "meilisearch", "MEILISEARCH_URL", "meili:7700"}, []string{"MEILISEARCH_URL"}},
		{[]string{"EMBEDDINGS_URL", "ftp://x"}, []string{"EMBEDDINGS_DIMENSIONS", "EMBEDDINGS_MODEL", "EMBEDDINGS_URL"}},
		{[]string{"KNOWLEDGE_CHUNK_CHARS", "100", "EMBEDDINGS_BATCH", "0"}, []string{"EMBEDDINGS_BATCH", "KNOWLEDGE_CHUNK_CHARS"}},
	} {
		_, err := config.Load[config.Config](lookup(append([]string{"DATABASE_URL", dbURL}, c.env...)...))
		if got := vars(t, err); !reflect.DeepEqual(got, c.want) {
			t.Fatalf("%v: %v", c.env, got)
		}
	}
}
