package tui

import (
	"strings"
	"testing"
)

func TestHistogramMedianColumn(t *testing.T) {
	errs := []float64{-5, -2, 0, 3, 5, 8}
	rows := histogram(errs, 1.5, 20, 4)
	if len(rows) != 4 {
		t.Fatalf("rows=%d", len(rows))
	}
	combined := strings.Join(rows, "")
	if !strings.Contains(combined, "┊") {
		t.Fatalf("expected median marker in %q", combined)
	}
}

func TestOffsetScaleMarksBoth(t *testing.T) {
	line, cur, rec := offsetScale(-30, -32, 24)
	if cur == rec {
		t.Fatalf("expected distinct marks on %q", line)
	}
	rs := []rune(line)
	if rs[cur] != '●' {
		t.Fatalf("current mark at %d in %q", cur, line)
	}
	if rs[rec] != '◆' {
		t.Fatalf("recommended mark at %d in %q", rec, line)
	}
}

func TestMedianColumnOutOfRange(t *testing.T) {
	if got := medianColumn(999, 10, 20); got != -1 {
		t.Fatalf("got %d", got)
	}
}

func TestHistogramSpanUsesWidth(t *testing.T) {
	if got := histogramSpan(nil, 44); got != 25 {
		t.Fatalf("width 44 span=%v", got)
	}
	if got := histogramSpan([]float64{90}, 44); got != 95 {
		t.Fatalf("data span=%v", got)
	}
}
