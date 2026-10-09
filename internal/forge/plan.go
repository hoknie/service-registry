package forge

import (
	"math"
	"strings"
)

func Slugify(name string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			dash = false
		} else if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	s := strings.TrimRight(b.String(), "-")
	if len(s) > 63 {
		s = strings.TrimRight(s[:63], "-")
	}
	if s == "" {
		return "repo"
	}
	return s
}

func Subgroups(owner, fullPath string, mirror bool) []string {
	if !mirror {
		return nil
	}
	rest := fullPath
	if len(rest) > len(owner) && strings.EqualFold(rest[:len(owner)], owner) && rest[len(owner)] == '/' {
		rest = rest[len(owner)+1:]
	}
	segments := strings.Split(rest, "/")
	if len(segments) <= 1 {
		return nil
	}
	return segments[:len(segments)-1]
}

func GroupPath(owner string, subgroups []string, n int) string {
	return owner + "/" + strings.Join(subgroups[:n], "/")
}

type Target struct {
	Remote RemoteRepo
	Link   *Link
}

func Plan(remotes []RemoteRepo, links []Link) (targets []Target, orphans []Link) {
	byID := make(map[string]*Link, len(links))
	for i := range links {
		byID[links[i].ExternalID] = &links[i]
	}
	seen := map[string]bool{}
	for _, r := range remotes {
		if seen[r.ExternalID] {
			continue
		}
		seen[r.ExternalID] = true
		targets = append(targets, Target{Remote: r, Link: byID[r.ExternalID]})
	}
	for _, l := range links {
		if !seen[l.ExternalID] && !l.Orphaned {
			orphans = append(orphans, l)
		}
	}
	return targets, orphans
}

func NeedsDetails(t Target) bool {
	if t.Link == nil || t.Link.Orphaned || t.Link.SourceUpdatedAt == nil {
		return true
	}
	changed := t.Remote.ChangedAt()
	return changed == nil || changed.After(*t.Link.SourceUpdatedAt)
}

func Percentages(sizes map[string]float64) map[string]float64 {
	var total float64
	for _, v := range sizes {
		total += v
	}
	out := make(map[string]float64, len(sizes))
	if total <= 0 {
		return out
	}
	for k, v := range sizes {
		if p := math.Round(v/total*1000) / 10; p > 0 {
			out[k] = p
		}
	}
	return out
}
