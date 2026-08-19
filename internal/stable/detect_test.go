package stable

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveRootNixLocation(t *testing.T) {
	loc := t.TempDir()
	osu := filepath.Join(loc, "osu")
	if err := os.Mkdir(osu, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(osu, "osu!.exe"), []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(osu, "osu!.max.cfg"), []byte("Offset = 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	root, err := ResolveRoot(loc)
	if err != nil {
		t.Fatal(err)
	}
	if root != osu {
		t.Fatalf("got %s want %s", root, osu)
	}

	root, err = ResolveRoot(filepath.Join(osu, "osu!.exe"))
	if err != nil {
		t.Fatal(err)
	}
	if root != osu {
		t.Fatalf("exe: got %s", root)
	}
}

func TestUnixStaticIncludesNixOsuStable(t *testing.T) {
	home := filepath.Join(string(filepath.Separator), "home", "max")
	xdg := filepath.Join(home, ".local", "share")
	got := unixStaticCandidates(home, xdg)
	for _, w := range []string{
		filepath.Join(xdg, "nix-osu-stable", "osu"),
		filepath.Join(xdg, "nix-osu-stable"),
		filepath.Join(xdg, "osu-wine"),
	} {
		if !containsPath(got, w) {
			t.Fatalf("missing %s in %v", w, got)
		}
	}
}

func TestWindowsStaticDefault(t *testing.T) {
	got := windowsStaticCandidates(
		filepath.Join("C:", "Users", "me", "AppData", "Local"),
		filepath.Join("C:", "Users", "me"),
		filepath.Join("C:", "Program Files"),
		filepath.Join("C:", "Program Files (x86)"),
		filepath.Join("C:", "Users", "me"),
	)
	want := filepath.Join("C:", "Users", "me", "AppData", "Local", "osu!")
	if !containsPath(got, want) {
		t.Fatalf("missing default %s in %v", want, got)
	}
}

func TestDarwinOfficialWrapper(t *testing.T) {
	got := darwinStaticCandidates("/Users/me")
	if !containsPath(got, "/Applications/osu!.app/Contents/Resources/drive_c/osu!") {
		t.Fatalf("missing official mac path: %v", got)
	}
}

func TestDetectEnvOverride(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "osu!.exe"), []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "osu!.max.cfg"), []byte("Offset = -12\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OSU_STABLE_DIR", dir)
	inst, err := Detect("")
	if err != nil {
		t.Fatal(err)
	}
	if inst.Offset != -12 {
		t.Fatalf("offset %d", inst.Offset)
	}
}

func containsPath(paths []string, want string) bool {
	for _, p := range paths {
		if p == want || strings.EqualFold(p, want) {
			return true
		}
	}
	return false
}
