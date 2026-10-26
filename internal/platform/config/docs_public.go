package config

type DocsPublicConfig struct {
	RatePerMinute uint32 `env:"DOCS_PUBLIC_RATE_PER_MIN" envDefault:"300" validate:"min=10,max=100000"`
}
