package stable

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteOffsetCRLF(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "osu!.exe"), []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(dir, "osu!.tester.cfg")
	body := "BeatmapDirectory = Songs\r\nOffset = 0\r\nUsername = tester\r\n"
	if err := os.WriteFile(cfg, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "Songs"), 0o755); err != nil {
		t.Fatal(err)
	}

	inst, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if inst.Offset != 0 {
		t.Fatalf("offset: %d", inst.Offset)
	}
	if err := inst.WriteOffset(-36); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	s := string(got)
	if !strings.Contains(s, "Offset = -36") {
		t.Fatalf("missing new offset:\n%s", s)
	}
	if !strings.Contains(s, "\r\n") {
		t.Fatalf("lost CRLF:\n%q", s)
	}
	if strings.Contains(s, "keyIncreaseAudioOffset = -36") {
		t.Fatal("rewrote the wrong Offset-like key")
	}
}

func TestDetectExplicit(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "osu!.exe"), []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "osu!.max.cfg"), []byte("Offset = -40\nBeatmapDirectory = Songs\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	inst, err := Detect(dir)
	if err != nil {
		t.Fatal(err)
	}
	if inst.Offset != -40 {
		t.Fatalf("got offset %d", inst.Offset)
	}
}
