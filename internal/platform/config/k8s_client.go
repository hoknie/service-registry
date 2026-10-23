package config

type K8sClientConfig struct {
	RequestTimeoutSecs uint32 `env:"K8S_REQUEST_TIMEOUT_SECS" envDefault:"10" validate:"min=1,max=120"`
	ListLimit          uint32 `env:"K8S_LIST_LIMIT" envDefault:"500" validate:"min=50,max=5000"`
}
