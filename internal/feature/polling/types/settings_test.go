package types

import (
	"errors"
	"testing"
)

func TestMerge(t *testing.T) {
	t.Parallel()
	off, on := ModeOff, ModeOn
	chain := []ChainNode{
		{Name: "project", Own: NodeSettings{Mode: &on}},
		{Name: "folder", Own: NodeSettings{Mode: &off}},
		{Name: "org", Own: NodeSettings{IntervalSecs: new(1800)}},
	}
	tests := []struct {
		name  string
		chain []ChainNode
		want  Effective
		from  From
	}{
		{"defaults", []ChainNode{{}}, Effective{TimeoutSecs: 30, Mode: ModeOn}, From{-1, -1, -1}},
		{"nearest wins per field", chain, Effective{IntervalSecs: new(1800), TimeoutSecs: 30, Mode: ModeOn}, From{2, -1, 0}},
		{"inherited mode", chain[1:], Effective{IntervalSecs: new(1800), TimeoutSecs: 30, Mode: ModeOff}, From{1, -1, 0}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, from := Merge(tt.chain, 30)
			if got.TimeoutSecs != tt.want.TimeoutSecs || got.Mode != tt.want.Mode || from != tt.from ||
				(got.IntervalSecs == nil) != (tt.want.IntervalSecs == nil) ||
				(got.IntervalSecs != nil && *got.IntervalSecs != *tt.want.IntervalSecs) {
				t.Fatalf("Merge = %+v %+v, want %+v %+v", got, from, tt.want, tt.from)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	t.Parallel()
	bad := Mode("sometimes")
	tests := []struct {
		name string
		in   NodeSettings
		ok   bool
	}{
		{"empty", NodeSettings{}, true},
		{"bounds", NodeSettings{IntervalSecs: new(60), TimeoutSecs: new(600)}, true},
		{"interval too small", NodeSettings{IntervalSecs: new(30)}, false},
		{"interval too large", NodeSettings{IntervalSecs: new(86401)}, false},
		{"timeout zero", NodeSettings{TimeoutSecs: new(0)}, false},
		{"unknown mode", NodeSettings{Mode: &bad}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := Validate(tt.in)
			if (err == nil) != tt.ok || (err != nil && !errors.Is(err, InvalidPolling)) {
				t.Fatalf("Validate(%+v) = %v", tt.in, err)
			}
		})
	}
}

func TestEffective(t *testing.T) {
	t.Parallel()
	e := Effective{TimeoutSecs: 5, Mode: ModePublished}
	if e.IntervalOr(900) != 900 || e.Timeout().Seconds() != 5 {
		t.Fatalf("defaults: %d %v", e.IntervalOr(900), e.Timeout())
	}
	if !e.DocsPaused(false) || e.DocsPaused(true) || e.Off() {
		t.Fatal("published mode pauses only unpublished docs")
	}
	e.Mode, e.IntervalSecs = ModeOff, new(120)
	if !e.Off() || !e.DocsPaused(true) || e.IntervalOr(900) != 120 {
		t.Fatal("off pauses everything")
	}
}
