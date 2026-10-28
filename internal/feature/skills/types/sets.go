package types

import (
	"slices"

	"github.com/google/uuid"
)

const (
	SetAll      = "all"
	SetSelected = "selected"
	MaxSetItems = 500
)

type Set struct {
	Mode    string
	Folders []uuid.UUID
	Skills  []uuid.UUID
}

type SetItem struct {
	Kind   string
	ID     uuid.UUID
	Path   string
	Name   string
	Skills int64
	Status Status
	Global bool
}

func CleanSet(mode string, folders, skills []uuid.UUID) (Set, error) {
	if mode == "" {
		mode = SetAll
	}
	if mode != SetAll && mode != SetSelected {
		return Set{}, InvalidSet
	}
	if folders == nil {
		folders = []uuid.UUID{}
	}
	if skills == nil {
		skills = []uuid.UUID{}
	}
	all := append(slices.Clone(folders), skills...)
	slices.SortFunc(all, func(a, b uuid.UUID) int { return slices.Compare(a[:], b[:]) })
	if len(all) > MaxSetItems || len(slices.Compact(all)) != len(folders)+len(skills) {
		return Set{}, InvalidSet
	}
	return Set{Mode: mode, Folders: folders, Skills: skills}, nil
}
