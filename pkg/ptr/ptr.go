package ptr

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
	return new(s)
}
