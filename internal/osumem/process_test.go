package osumem

import "testing"

func TestIsOsuStableCmd(t *testing.T) {
	yes := []string{
		`osu!.exe`,
		`C:\users\max\AppData\Local\osu!\osu!.exe`,
		`/home/max/.local/share/nix-osu-stable/osu/osu!.exe`,
		"Z:\\home\\max\\.local\\share\\nix-osu-stable\\osu\\osu!.exe",
	}
	for _, s := range yes {
		if !isOsuStableCmd(s) {
			t.Fatalf("expected match: %q", s)
		}
	}
	no := []string{
		`osu!lazer.exe`,
		`osu!framework`,
		`notepad.exe`,
		``,
	}
	for _, s := range no {
		if isOsuStableCmd(s) {
			t.Fatalf("expected skip: %q", s)
		}
	}
}
