package ptr

func To[T any](v T) *T { return &v }

func Value[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}
	return *p
}

func NonEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
