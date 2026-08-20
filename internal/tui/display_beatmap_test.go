package tui

import (
	"bytes"
	"strings"
	"testing"
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

func TestPlayStartedWithoutTitle(t *testing.T) {
	var stderr bytes.Buffer
	d := New(false, &bytes.Buffer{}, &stderr)
	d.PlayStarted("")
	got := stderr.String()
	if !strings.Contains(got, "play started") {
		t.Fatalf("stderr=%q", got)
	}
}
