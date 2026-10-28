package config

type JobsConfig struct {
	Enabled              bool   `env:"BACKGROUND_JOBS_ENABLED" envDefault:"true"`
	RepoSyncConcurrency  uint32 `env:"REPO_SYNC_CONCURRENCY" envDefault:"2" validate:"min=1,max=32"`
	RepoSyncIntervalSecs uint32 `env:"REPO_SYNC_INTERVAL_SECS" envDefault:"900" validate:"min=60,max=86400"`
	RepoSyncRunsKept     uint32 `env:"REPO_SYNC_RUNS_KEPT" envDefault:"20" validate:"min=1,max=1000"`
}

func (c *JobsConfig) checkEnv(vars Env) Errors {
	var errs Errors
	for _, name := range []string{"CONCURRENCY", "INTERVAL_SECS", "RUNS_KEPT"} {
		if _, ok := vars["FORGE_SYNC_"+name]; ok {
			errs = append(errs, &Error{Var: "FORGE_SYNC_" + name, Reason: "was renamed to REPO_SYNC_" + name})
		}
	}
	return errs
}
