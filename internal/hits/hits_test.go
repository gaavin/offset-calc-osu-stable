package hits

import (
	"math"
	"testing"
)

func TestMedianEvenOdd(t *testing.T) {
	if g := Median([]float64{1, 3, 2}); g != 2 {
		t.Fatalf("odd median: got %v", g)
	}
	if g := Median([]float64{1, 2, 3, 4}); g != 2.5 {
		t.Fatalf("even median: got %v", g)
	}
}

func TestSuggestedOffset(t *testing.T) {
	if g := SuggestedOffset(0, 5); g != -5 {
		t.Fatalf("late hits: got %v", g)
	}
	if g := SuggestedOffset(-30, -5); g != -25 {
		t.Fatalf("early hits: got %v", g)
	}
}

func TestUnstableRate(t *testing.T) {
	ur := UnstableRate([]float64{10, -10, 10, -10})
	if math.Abs(ur-100) > 1e-6 {
		t.Fatalf("ur=%v", ur)
	}
}
