package deploy

import (
	"sort"
	"strings"
	"time"
)

func ObserveState(w Workload, pods []Pod, jobs []Job, prev State, now time.Time, interval time.Duration) (State, []Image) {
	images := imagesOf(w, pods)
	if w.Kind == KindCronJob {
		schedule, suspended := w.Schedule, w.Suspended
		return State{Schedule: &schedule, Suspended: &suspended, LastRun: lastRun(jobs)}, images
	}
	var restarts int32
	for _, p := range pods {
		for _, c := range p.Containers {
			restarts += c.Restarts
		}
	}
	s := State{Replicas: &Replicas{Desired: w.Desired, Ready: w.Ready, Updated: w.Updated}, Restarts: &restarts}
	rollout := RolloutProgressing
	switch {
	case w.ObservedGeneration >= w.Generation && w.Updated >= w.Desired && w.Ready >= w.Desired:
		rollout = RolloutComplete
	case deadlineExceeded(w):
		rollout = RolloutStalled
	default:
		since := now
		if prev.ProgressingSince != nil {
			if t, err := time.Parse(time.RFC3339, *prev.ProgressingSince); err == nil {
				since = t
			}
		}
		text := since.UTC().Format(time.RFC3339)
		s.ProgressingSince = &text
		if now.Sub(since) > 2*interval {
			rollout = RolloutStalled
		}
	}
	s.Rollout = &rollout
	return s, images
}

func deadlineExceeded(w Workload) bool {
	for _, c := range w.Conditions {
		if c.Type == "Progressing" && c.Reason == "ProgressDeadlineExceeded" {
			return true
		}
	}
	return false
}

func imagesOf(w Workload, pods []Pod) []Image {
	out := make([]Image, 0, len(w.Containers))
	for _, c := range w.Containers {
		img := Image{Container: c.Name, Image: c.Image}
	pods:
		for _, p := range pods {
			for _, pc := range p.Containers {
				if pc.Name != c.Name {
					continue
				}
				if _, d, ok := strings.Cut(pc.ImageID, "@"); ok && d != "" {
					img.Digest = &d
					break pods
				}
			}
		}
		out = append(out, img)
	}
	return out
}

func lastRun(jobs []Job) *LastRun {
	if len(jobs) == 0 {
		return nil
	}
	sorted := append([]Job(nil), jobs...)
	sort.SliceStable(sorted, func(i, j int) bool {
		a, b := sorted[i].StartedAt, sorted[j].StartedAt
		if a == nil || b == nil {
			return b == nil && a != nil
		}
		return a.After(*b)
	})
	j := sorted[0]
	r := &LastRun{Status: "active"}
	switch {
	case j.Active > 0:
	case j.Failed > 0:
		r.Status = "failed"
	case j.Succeeded > 0:
		r.Status = "succeeded"
	}
	if j.StartedAt != nil {
		t := j.StartedAt.UTC().Format(time.RFC3339)
		r.StartedAt = &t
	}
	return r
}
