package ptr

import "testing"

func TestPointers(t *testing.T) {
	t.Parallel()
	if Value[string](nil) != "" || Value(new("a")) != "a" {
		t.Fatal("Value must read the value or the zero value")
	}
	if NonEmpty("") != nil || *NonEmpty("x") != "x" {
		t.Fatal("NonEmpty must give nil for an empty string")
	}
}
