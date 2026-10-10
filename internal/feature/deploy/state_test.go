package deploy

import (
	"testing"
	"time"
)

func TestRolloutStates(t *testing.T) {
	now := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	base := Workload{Kind: KindDeployment, Generation: 3, ObservedGeneration: 3, Desired: 3, Ready: 3, Updated: 3,
		Containers: []Container{{Name: "app", Image: "api:1"}}}
	pods := []Pod{{Containers: []PodContainer{{Name: "app", ImageID: "docker-pullable://api@sha256:d1", Restarts: 2}}},
		{Containers: []PodContainer{{Name: "app", Restarts: 1}}}}
	s, images := ObserveState(base, pods, nil, State{}, now, time.Minute)
	if *s.Rollout != RolloutComplete || *s.Restarts != 3 || s.Replicas.Ready != 3 || s.ProgressingSince != nil {
		t.Fatalf("%+v", s)
	}
	if len(images) != 1 || *images[0].Digest != "sha256:d1" {
		t.Fatalf("%+v", images)
	}
	rolling := base
	rolling.Updated = 1
	s, _ = ObserveState(rolling, nil, nil, State{}, now, time.Minute)
	if *s.Rollout != RolloutProgressing || *s.ProgressingSince != "2026-10-09T12:00:00Z" {
		t.Fatalf("%+v", s)
	}
	s, _ = ObserveState(rolling, nil, nil, s, now.Add(3*time.Minute), time.Minute)
	if *s.Rollout != RolloutStalled {
		t.Fatalf("stalled after two intervals: %+v", s)
	}
	deadline := rolling
	deadline.Conditions = []Condition{{Type: "Progressing", Status: "False", Reason: "ProgressDeadlineExceeded"}}
	if s, _ = ObserveState(deadline, nil, nil, State{}, now, time.Minute); *s.Rollout != RolloutStalled {
		t.Fatalf("%+v", s)
	}
}

func TestCronJobState(t *testing.T) {
	now := time.Now()
	old, recent := now.Add(-2*time.Hour), now.Add(-time.Hour)
	cron := Workload{Kind: KindCronJob, Schedule: "0 * * * *", Containers: []Container{{Name: "job", Image: "j:1"}}}
	s, _ := ObserveState(cron, nil, []Job{{StartedAt: &old, Succeeded: 1}, {StartedAt: &recent, Failed: 1}}, State{}, now, time.Minute)
	if s.Replicas != nil || *s.Schedule != "0 * * * *" || *s.Suspended || s.LastRun.Status != "failed" {
		t.Fatalf("%+v", s)
	}
	if s, _ = ObserveState(cron, nil, nil, State{}, now, time.Minute); s.LastRun != nil {
		t.Fatalf("never ran: %+v", s.LastRun)
	}
}
