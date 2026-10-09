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
	K8s          K8sConfig
	Knowledge    KnowledgeConfig
	Uploads      UploadsConfig
	Search       SearchConfig
	OAuth        OAuthConfig
	Bootstrap    BootstrapConfig
}

func (c *Config) check() Errors {
	var errs Errors
	for _, s := range []checker{&c.DB, &c.Web, &c.Log, &c.Session, &c.Bootstrap, &c.Secrets, &c.Outbound, &c.LinkCheck, &c.OAuth, &c.Knowledge, &c.Search, &c.Uploads} {
		errs = append(errs, s.check()...)
	}
	return errs
}

func FromEnv() (Config, error) { return Load[Config](ProcessEnv()) }

func (c *Config) checkEnv(vars Env) Errors {
	errs := c.OAuth.checkEnv(vars)
	if len(c.OAuth.Providers) > 0 && c.Web.PublicURL == "" {
		errs = append(errs, &Error{Var: "PUBLIC_URL", Reason: "is required for the callback addresses of OAUTH_PROVIDERS"})
	}
	return errs
}
