package config

type Config struct {
	HTTP         HTTPConfig
	DB           DbConfig
	Web          WebConfig
	Log          LogConfig
	Session      SessionConfig
	PasswordHash PasswordHashConfig
	LoginLimit   LoginLimitConfig
	Ingest       IngestConfig
	Pat          PatConfig
	Secrets      SecretsConfig
	Outbound     OutboundConfig
	Jobs         JobsConfig
	Branches     BranchesConfig
	LinkCheck    LinkCheckConfig
	Notify       NotifyConfig
	K8s          K8sConfig
	Knowledge    KnowledgeConfig
	Uploads      UploadsConfig
	Search       SearchConfig
	OAuth        OAuthConfig
	MCPOAuth     MCPOAuthConfig
	Metrics      MetricsConfig
	Bootstrap    BootstrapConfig
}

func (c *Config) check() Errors {
	var errs Errors
	for _, s := range []checker{&c.DB, &c.Web, &c.Log, &c.Session, &c.Bootstrap, &c.Secrets, &c.Outbound, &c.LinkCheck, &c.Notify, &c.OAuth, &c.MCPOAuth, &c.Knowledge, &c.Search, &c.Uploads, &c.Metrics} {
		errs = append(errs, s.check()...)
	}
	if c.MCPOAuth.Enabled && c.Web.PublicURL == "" {
		errs = append(errs, &Error{Var: "PUBLIC_URL", Reason: "is required by MCP_OAUTH=on for the issuer and resource addresses"})
	}
	return errs
}

func FromEnv() (Config, error) { return Load[Config](ProcessEnv()) }

func (c *Config) checkEnv(vars Env) Errors {
	errs := c.OAuth.checkEnv(vars)
	if len(c.OAuth.Providers) > 0 && c.Web.PublicURL == "" {
		errs = append(errs, &Error{Var: "PUBLIC_URL", Reason: "is required for the callback addresses of OAUTH_PROVIDERS"})
	}
	if c.MCPOAuth.Enabled && c.Web.PublicURL == "" {
		errs = append(errs, &Error{Var: "PUBLIC_URL", Reason: "is required by MCP_OAUTH=on for the issuer and resource addresses"})
	}
	return errs
}
