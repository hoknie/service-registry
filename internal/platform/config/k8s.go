package config

type K8sConfig struct {
	K8sClientConfig
	PollConcurrency       uint32 `env:"K8S_POLL_CONCURRENCY" envDefault:"2" validate:"min=1,max=32"`
	HistoryConfirmSecs    uint32 `env:"K8S_HISTORY_CONFIRM_SECS" envDefault:"600" validate:"max=86400"`
	WorkloadRetentionDays uint32 `env:"K8S_WORKLOAD_RETENTION_DAYS" envDefault:"7" validate:"min=1,max=365"`
	AgentReportMaxBytes   int64  `env:"K8S_AGENT_REPORT_MAX_BYTES" envDefault:"67108864" validate:"min=1048576,max=268435456"`
}
