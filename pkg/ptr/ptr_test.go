package ptr

import "testing"

func TestPointers(t *testing.T) {
	t.Parallel()
	if *To(3) != 3 {
		t.Fatal("To must point at the value")
	}
	if Value[string](nil) != "" || Value(To("a")) != "a" {
		t.Fatal("Value must read the value or the zero value")
	}
	if NonEmpty("") != nil || *NonEmpty("x") != "x" {
		t.Fatal("NonEmpty must give nil for an empty string")
	}
}
