package ingest

import "encoding/json"

type FieldState int

const (
	Absent FieldState = iota
	Null
	WrongType
	Set
)

type Field[T any] struct {
	Value T
	State FieldState
	Raw   json.RawMessage
}

func (f *Field[T]) UnmarshalJSON(b []byte) error {
	f.Raw = append(json.RawMessage(nil), b...)
	if string(b) == "null" {
		f.State = Null
		return nil
	}
	if err := json.Unmarshal(b, &f.Value); err != nil {
		f.State = WrongType
		return nil
	}
	f.State = Set
	return nil
}
