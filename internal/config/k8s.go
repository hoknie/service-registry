package config

type K8sConfig struct {
	PollConcurrency       uint32 `env:"K8S_POLL_CONCURRENCY" envDefault:"2" validate:"min=1,max=32"`
	RequestTimeoutSecs    uint32 `env:"K8S_REQUEST_TIMEOUT_SECS" envDefault:"10" validate:"min=1,max=120"`
	ListLimit             uint32 `env:"K8S_LIST_LIMIT" envDefault:"500" validate:"min=50,max=5000"`
	HistoryConfirmSecs    uint32 `env:"K8S_HISTORY_CONFIRM_SECS" envDefault:"600" validate:"max=86400"`
	WorkloadRetentionDays uint32 `env:"K8S_WORKLOAD_RETENTION_DAYS" envDefault:"7" validate:"min=1,max=365"`
}
