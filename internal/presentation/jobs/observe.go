package jobs

import (
	"time"
)

func observe(deps Deps, job string, run func() error) error {
	start := time.Now()
	err := run()
	outcome := "succeeded"
	if err != nil {
		outcome = "failed"
	}
	deps.Metrics.JobRun(job, outcome, time.Since(start))
	return err
}

func observeCount[T any](deps Deps, job string, run func() (T, error)) (T, error) {
	var n T
	err := observe(deps, job, func() (err error) {
		n, err = run()
		return err
	})
	return n, err
}

func panicked(deps Deps, job string) func(time.Duration) {
	return func(d time.Duration) { deps.Metrics.JobRun(job, "panicked", d) }
}
