package tui

import (
	"bytes"
	"strings"
	"testing"

	"github.com/pterm/pterm"
)

func TestBeatmapUnavailablePlain(t *testing.T) {
	var stderr bytes.Buffer
	d := New(false, &bytes.Buffer{}, &stderr)
	d.BeatmapUnavailable()
	got := stderr.String()
	if !strings.Contains(got, "map title unavailable") {
		t.Fatalf("stderr=%q", got)
	}
}

func TestRenderLiveIncludesMapTitle(t *testing.T) {
	got := renderLive(PlayStats{
		HitCount:  10,
		Errors:    []float64{-5, 0, 5},
		Median:    0,
		MinHits:   50,
		HasOffset: true,
		CurOffset: -30,
		Recommend: -24,
		MapTitle:  "Camellia - GHOST [Hyper]",
	})
	if !strings.Contains(pterm.RemoveColorFromString(got), "Camellia - GHOST [Hyper]") {
		t.Fatalf("missing map title:\n%s", got)
	}
}

func TestPlayStartedWithoutTitle(t *testing.T) {
	var stderr bytes.Buffer
	d := New(false, &bytes.Buffer{}, &stderr)
	d.PlayStarted("")
	got := stderr.String()
	if !strings.Contains(got, "play started") {
		t.Fatalf("stderr=%q", got)
	}
}
