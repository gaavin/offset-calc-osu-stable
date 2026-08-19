package osr

import (
	"bytes"
	"testing"
	"time"
)

func TestReadOsuString(t *testing.T) {
	// empty
	r := bytes.NewReader([]byte{0x00})
	s, err := readOsuString(r)
	if err != nil || s != "" {
		t.Fatalf("empty: %q %v", s, err)
	}

	// "hi" = 0x0b, uleb 2, 'h','i'
	r = bytes.NewReader([]byte{0x0b, 0x02, 'h', 'i'})
	s, err = readOsuString(r)
	if err != nil || s != "hi" {
		t.Fatalf("hi: %q %v", s, err)
	}
}

func TestTimeFromTicks(t *testing.T) {
	// Unix epoch in .NET ticks: 621355968000000000
	got := timeFromTicks(621355968000000000)
	if !got.Equal(time.Unix(0, 0).UTC()) {
		t.Fatalf("got %v", got)
	}
}

func TestSkipReason(t *testing.T) {
	r := &Replay{Mode: ModeTaiko, Frames: []Frame{{}}}
	if r.SkipReason() == "" {
		t.Fatal("taiko should skip")
	}
	r = &Replay{Mode: ModeOsu, Mods: ModRelax, Frames: []Frame{{}}}
	if r.SkipReason() != "relax" {
		t.Fatalf("got %q", r.SkipReason())
	}
	r = &Replay{Mode: ModeOsu, Frames: []Frame{{Delta: 1}}}
	if r.SkipReason() != "" {
		t.Fatalf("standard should pass, got %q", r.SkipReason())
	}
}

func TestModString(t *testing.T) {
	r := &Replay{}
	if r.ModString() != "NM" {
		t.Fatal(r.ModString())
	}
	r.Mods = ModHidden | ModDoubleTime
	if r.ModString() != "HDDT" {
		t.Fatalf("got %s", r.ModString())
	}
	r.Mods = ModNightcore | ModDoubleTime
	if r.ModString() != "NC" {
		t.Fatalf("got %s", r.ModString())
	}
}
