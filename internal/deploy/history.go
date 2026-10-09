package deploy

import "time"

type Pending struct {
	Version string
	Since   time.Time
}

func NextPending(current, observed *string, prev *Pending, now time.Time) *Pending {
	if observed == nil || (current != nil && *current == *observed) {
		return nil
	}
	if prev != nil && prev.Version == *observed {
		return prev
	}
	return &Pending{Version: *observed, Since: now}
}

func Due(p *Pending, now time.Time, confirm time.Duration) bool {
	return p != nil && !now.Before(p.Since.Add(confirm))
}
