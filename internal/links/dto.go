package links

import "github.com/google/uuid"

type CreateKind struct {
	Key      string
	Names    map[string]string
	Icon     *string
	Position *int64
}

type UpdateKind struct {
	Names    map[string]string
	Icon     *string
	Position *int64
}

type NewKind struct {
	ID       uuid.UUID
	Key      string
	Names    Names
	Icon     Icon
	Position int32
}

type KindChanges struct {
	Names    Names
	Icon     *Icon
	Position *int32
}

type PutTemplate struct {
	KindKey  string
	Template *string
	Disabled *bool
	Position *int64
}

type NewTemplate struct {
	ID       uuid.UUID
	NodeID   uuid.UUID
	LinkKey  string
	KindKey  string
	Template *string
	Disabled bool
	Position int32
}

type Preview struct {
	Template    string
	ProjectID   *uuid.UUID
	Branch      *string
	Environment *string
}

type LinkQuery struct {
	Branch      string
	Environment string
}
