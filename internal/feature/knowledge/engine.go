package knowledge

import (
	"context"
	"slices"

	"github.com/google/uuid"
)

type Mode string

const (
	ModeText     Mode = "text"
	ModeSemantic Mode = "semantic"
	ModeHybrid   Mode = "hybrid"
)

func ParseMode(raw string) (Mode, bool) {
	switch m := Mode(raw); m {
	case ModeText, ModeSemantic, ModeHybrid:
		return m, true
	}
	return "", false
}

type Engine interface {
	Name() string
	Modes() []Mode
	DefaultMode() Mode
	Handles(m Mode) bool
	Search(ctx context.Context, q EngineQuery) ([]Hit, bool, error)
}

type TextOnly struct{}

func (TextOnly) Name() string      { return "postgres" }
func (TextOnly) Modes() []Mode     { return []Mode{ModeText} }
func (TextOnly) DefaultMode() Mode { return ModeText }
func (TextOnly) Handles(Mode) bool { return false }
func (TextOnly) Search(context.Context, EngineQuery) ([]Hit, bool, error) {
	return nil, false, ErrSearchUnavailable
}

type EngineQuery struct {
	Search
	Mode     Mode
	Vector   []float32
	Projects []uuid.UUID
	Branches map[uuid.UUID]string
}

type ExternalIndex interface {
	Sync(ctx context.Context, project uuid.UUID, docs []IndexDoc) error
	Retain(ctx context.Context, projects []uuid.UUID) error
	Target() string
}

type IndexDoc struct {
	ProjectID   uuid.UUID
	ProjectPath string
	ProjectName string
	Branch      string
	Commit      string
	Path        string
	Kind        Kind
	Ord         int
	Text        string
	Vector      []float32
}

func Offers(e Engine, m Mode) bool { return slices.Contains(e.Modes(), m) }

type Span struct{ Start, End int }

func Chunk(content string, maxChars int) []Span {
	runes := []rune(content)
	if len(runes) == 0 {
		return nil
	}
	var cuts []int
	for i := 0; i < len(runes); i++ {
		if i > 0 && runes[i-1] == '\n' && runes[i] == '#' {
			cuts = append(cuts, i)
		}
	}
	var spans []Span
	start := 0
	flush := func(end int) {
		for start < end {
			stop := min(end, start+maxChars)
			if stop < end {
				if p := lastBreak(runes, start, stop); p > start {
					stop = p
				}
			}
			if hasText(runes[start:stop]) {
				spans = append(spans, Span{start, stop})
			}
			start = stop
		}
	}
	for _, c := range cuts {
		flush(c)
	}
	flush(len(runes))
	return spans
}

func lastBreak(runes []rune, from, to int) int {
	for i := to - 1; i > from+1; i-- {
		if runes[i] == '\n' && runes[i-1] == '\n' {
			return i + 1
		}
	}
	for i := to - 1; i > from; i-- {
		if runes[i] == '\n' || runes[i] == ' ' {
			return i + 1
		}
	}
	return to
}

func hasText(r []rune) bool {
	for _, c := range r {
		if c != ' ' && c != '\n' && c != '\t' && c != '\r' {
			return true
		}
	}
	return false
}

func Slice(content string, s Span) string {
	runes := []rune(content)
	return string(runes[min(s.Start, len(runes)):min(s.End, len(runes))])
}

func HitKey(h Hit) string { return h.ProjectID.String() + "\x00" + h.Branch + "\x00" + h.Path }

func FuseRRF(text, semantic []Hit, limit int) []Hit {
	const k = 60
	score := map[string]float64{}
	first := map[string]Hit{}
	var order []string
	add := func(list []Hit, prefer bool) {
		for i, h := range list {
			key := HitKey(h)
			if _, ok := first[key]; !ok {
				first[key] = h
				order = append(order, key)
			} else if prefer {
				first[key] = h
			}
			score[key] += 1.0 / float64(k+i+1)
		}
	}
	add(semantic, false)
	add(text, true)
	slices.SortStableFunc(order, func(a, b string) int {
		switch {
		case score[a] > score[b]:
			return -1
		case score[a] < score[b]:
			return 1
		}
		return 0
	})
	out := make([]Hit, 0, min(limit, len(order)))
	for _, key := range order {
		if len(out) == limit {
			break
		}
		out = append(out, first[key])
	}
	return out
}
