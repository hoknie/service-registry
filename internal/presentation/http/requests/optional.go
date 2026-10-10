package requests

import "encoding/json"

type Optional[T any] struct {
	Present bool
	Value   *T
}

func (o *Optional[T]) UnmarshalJSON(b []byte) error {
	o.Present = true
	if string(b) == "null" {
		o.Value = nil
		return nil
	}
	var v T
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	o.Value = &v
	return nil
}
