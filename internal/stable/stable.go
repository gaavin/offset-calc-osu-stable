package stable

import (
	"bufio"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Install struct {
	Root   string
	Songs  string
	Cfg    string
	Offset int
}

func Open(root string) (*Install, error) {
	resolved, err := ResolveRoot(root)
	if err != nil {
		return nil, err
	}
	root = resolved
	inst := &Install{Root: root, Songs: filepath.Join(root, "Songs")}
	cfg, err := findUserCfg(root)
	if err != nil {
		return nil, err
	}
	inst.Cfg = cfg
	offset, songs, err := parseCfg(cfg)
	if err != nil {
		return nil, err
	}
	inst.Offset = offset
	if songs != "" {
		if filepath.IsAbs(songs) {
			inst.Songs = songs
		} else {
			inst.Songs = filepath.Join(root, songs)
		}
	}
	return inst, nil
}

func findUserCfg(root string) (string, error) {
	matches, err := filepath.Glob(filepath.Join(root, "osu!.*.cfg"))
	if err != nil {
		return "", err
	}
	type cand struct {
		path string
		mod  time.Time
		user bool
	}
	wantNames := userCfgNames()
	var users []cand
	for _, m := range matches {
		base := filepath.Base(m)
		if strings.EqualFold(base, "osu!.cfg") {
			continue
		}
		st, err := os.Stat(m)
		mod := time.Time{}
		if err == nil {
			mod = st.ModTime()
		}
		users = append(users, cand{
			path: m,
			mod:  mod,
			user: wantNames[strings.ToLower(base)],
		})
	}
	sort.Slice(users, func(i, j int) bool {
		if users[i].user != users[j].user {
			return users[i].user
		}
		return users[i].mod.After(users[j].mod)
	})
	if len(users) > 0 {
		return users[0].path, nil
	}
	fallback := filepath.Join(root, "osu!.cfg")
	if _, err := os.Stat(fallback); err == nil {
		return fallback, nil
	}
	return "", fmt.Errorf("no osu! user config in %s", root)
}

func userCfgNames() map[string]bool {
	out := map[string]bool{}
	for _, v := range []string{
		os.Getenv("USER"),
		os.Getenv("USERNAME"),
		os.Getenv("LOGNAME"),
	} {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		out[strings.ToLower("osu!."+v+".cfg")] = true
	}
	return out
}

func parseCfg(path string) (offset int, songs string, err error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, "", err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(strings.TrimSuffix(sc.Text(), "\r"))
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		switch k {
		case "Offset":
			fmt.Sscanf(v, "%d", &offset)
		case "BeatmapDirectory":
			songs = v
		}
	}
	return offset, songs, sc.Err()
}

func (inst *Install) WriteOffset(offset int) error {
	data, err := os.ReadFile(inst.Cfg)
	if err != nil {
		return err
	}
	crlf := strings.Contains(string(data), "\r\n")
	nl := "\n"
	if crlf {
		nl = "\r\n"
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(text, "\n")
	replaced := false
	for i, line := range lines {
		trim := strings.TrimSpace(line)
		k, _, ok := strings.Cut(trim, "=")
		if ok && strings.TrimSpace(k) == "Offset" {
			lines[i] = fmt.Sprintf("Offset = %d", offset)
			replaced = true
		}
	}
	if !replaced {
		if len(lines) == 0 || lines[len(lines)-1] != "" {
			lines = append(lines, fmt.Sprintf("Offset = %d", offset))
		} else {
			lines[len(lines)-1] = fmt.Sprintf("Offset = %d", offset)
			lines = append(lines, "")
		}
	}
	return os.WriteFile(inst.Cfg, []byte(strings.Join(lines, nl)), 0o644)
}

func (inst *Install) ReplayDirs() []string {
	return []string{
		filepath.Join(inst.Root, "Data", "r"),
		filepath.Join(inst.Root, "Replays"),
	}
}

func HashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func IndexSongs(songsDir string) (map[string]string, error) {
	idx := make(map[string]string)
	err := filepath.Walk(songsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if !strings.EqualFold(filepath.Ext(path), ".osu") {
			return nil
		}
		sum, err := HashFile(path)
		if err != nil {
			return nil
		}
		idx[sum] = path
		return nil
	})
	if err != nil {
		return nil, err
	}
	return idx, nil
}
