package types

import (
	"errors"

	"github.com/google/uuid"
)

type Trigger string

const (
	TriggerSchedule Trigger = "schedule"
	TriggerManual   Trigger = "manual"
	TriggerWebhook  Trigger = "webhook"
	TriggerCLI      Trigger = "cli"
)

type RunStatus string

const (
	RunRunning     RunStatus = "running"
	RunSucceeded   RunStatus = "succeeded"
	RunFailed      RunStatus = "failed"
	RunRateLimited RunStatus = "rate_limited"
)

var ErrRunFailed = errors.New("forge sync run failed")

type Problem struct {
	FullPath string `json:"full_path"`
	Code     string `json:"code"`
}

const MaxProblems = 100

type Counts struct {
	Created  int
	Updated  int
	Orphaned int
	Skipped  int
}

type Run struct {
	ID           uuid.UUID
	ConnectionID uuid.UUID
	Trigger      Trigger
	Status       RunStatus
	StartedAt    string
	FinishedAt   *string
	Counts       Counts
	ErrorCode    *string
	ErrorMessage *string
	Problems     []Problem
}

type RunResult struct {
	Status       RunStatus
	Counts       Counts
	ErrorCode    *string
	ErrorMessage *string
	Problems     []Problem
}

func (r RunResult) Stored() RunResult {
	if r.Problems == nil {
		r.Problems = []Problem{}
	}
	r.Problems = r.Problems[:min(len(r.Problems), MaxProblems)]
	return r
}

type Claimed struct {
	ID      uuid.UUID
	Trigger Trigger
}
