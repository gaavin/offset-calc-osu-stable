package stable

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ResolveRoot accepts an osu!stable folder, osu!.exe, or a wrapper state
// dir (nix-osu-stable location) and returns the directory that contains osu!.exe.
func ResolveRoot(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		return "", fmt.Errorf("empty path")
	}
	p = expandHome(p)
	p = filepath.Clean(p)

	st, err := os.Stat(p)
	if err != nil {
		return "", err
	}
	if !st.IsDir() {
		if strings.EqualFold(filepath.Base(p), "osu!.exe") {
			p = filepath.Dir(p)
		} else {
			return "", fmt.Errorf("%s is not an osu!stable folder", p)
		}
	}

	for _, dir := range []string{
		p,
		filepath.Join(p, "osu"),
		filepath.Join(p, "osu!"),
	} {
		if hasOsuExe(dir) {
			return dir, nil
		}
	}
	return "", fmt.Errorf("%s does not look like an osu!stable folder (missing osu!.exe)", p)
}

func hasOsuExe(dir string) bool {
	st, err := os.Stat(filepath.Join(dir, "osu!.exe"))
	return err == nil && !st.IsDir()
}

func expandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") || strings.HasPrefix(p, `~\`) {
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			return p
		}
		if p == "~" {
			return home
		}
		return filepath.Join(home, strings.TrimLeft(p[1:], `/\`))
	}
	return os.ExpandEnv(p)
}

// Detect finds an osu!stable install. explicit overrides auto-detection
// (-dir, or a path that is the game folder / osu!.exe / nix-osu-stable location).
func Detect(explicit string) (*Install, error) {
	if explicit != "" {
		return Open(explicit)
	}

	var tried []string
	seen := map[string]bool{}
	try := func(p string) *Install {
		p = strings.TrimSpace(p)
		if p == "" {
			return nil
		}
		p = expandHome(p)
		key := filepath.Clean(p)
		if seen[key] {
			return nil
		}
		seen[key] = true
		tried = append(tried, p)
		inst, err := Open(p)
		if err == nil {
			return inst
		}
		return nil
	}

	for _, key := range []string{"OSU_STABLE_DIR", "OSU_DIR", "OSUPATH"} {
		if inst := try(os.Getenv(key)); inst != nil {
			return inst, nil
		}
	}

	for _, p := range Candidates() {
		if inst := try(p); inst != nil {
			return inst, nil
		}
	}

	var b strings.Builder
	b.WriteString("osu!stable not found. Pass -dir to the folder that contains osu!.exe")
	if len(tried) > 0 {
		b.WriteString("\nLooked at:")
		limit := len(tried)
		if limit > 24 {
			limit = 24
		}
		for _, p := range tried[:limit] {
			b.WriteString("\n  ")
			b.WriteString(p)
		}
		if len(tried) > limit {
			fmt.Fprintf(&b, "\n  … %d more", len(tried)-limit)
		}
	}
	return nil, fmt.Errorf("%s", b.String())
}

// Candidate is one path that Detect will try, with a short reason.
type Candidate struct {
	Path   string
	Source string
}

func PrintDebug(w io.Writer) error {
	fmt.Fprintln(w, "osu-offset path probe")
	fmt.Fprintf(w, "GOOS=%s HOME=%s\n", currentGOOS(), detectHome())
	fmt.Fprintln(w)
	for _, c := range debugCandidates() {
		status := "missing"
		if root, err := ResolveRoot(c.Path); err == nil {
			status = "ok  " + root
		}
		fmt.Fprintf(w, "[%s] %s\n  %s\n", status, c.Source, c.Path)
	}
	return nil
}

func currentGOOS() string {
	return detectGOOS()
}
