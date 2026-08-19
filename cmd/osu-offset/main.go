package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/gaavin/offset-calc-osu-stable/internal/beatmap"
	"github.com/gaavin/offset-calc-osu-stable/internal/hits"
	"github.com/gaavin/offset-calc-osu-stable/internal/osr"
	"github.com/gaavin/offset-calc-osu-stable/internal/stable"
)

var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "osu-offset: %v\n", err)
		os.Exit(1)
	}
}

type play struct {
	Replay    *osr.Replay
	Path      string
	Beatmap   *beatmap.Beatmap
	Hits      hits.Result
	Suggested float64
	Skip      string
}

func run() error {
	dir := flag.String("dir", "", "osu!stable directory or osu!.exe (auto-detected on Windows, macOS, Linux, NixOS)")
	plays := flag.Int("plays", 50, "max recent plays to consider (lazer keeps 50)")
	minHits := flag.Int("min-hits", 50, "minimum timed hits per play (lazer uses 50)")
	apply := flag.Bool("apply", false, "write the recommended Offset into the osu! config")
	jsonOut := flag.Bool("json", false, "print JSON instead of text")
	verbose := flag.Bool("verbose", false, "show skipped plays")
	debugPaths := flag.Bool("debug-paths", false, "print install-path candidates and exit")
	dampen := flag.Bool("ur-dampen", false, "apply lazers per-beatmap UR damping (off for global offset)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `osu-offset %s — recommend a universal offset for osu!stable

Uses the same idea as osu!lazer: median hit error from recent standard
plays, then averages those into one suggested global Offset.

`, version)
		flag.PrintDefaults()
	}
	flag.Parse()

	if *debugPaths {
		return stable.PrintDebug(os.Stdout)
	}

	inst, err := stable.Detect(*dir)
	if err != nil {
		return err
	}

	reps, err := loadReplays(inst)
	if err != nil {
		return err
	}
	if len(reps) == 0 {
		return fmt.Errorf("no .osr replays in Data/r or Replays — play some maps with replay saving on")
	}

	if !*jsonOut {
		fmt.Fprintf(os.Stderr, "indexing beatmaps in %s …\n", inst.Songs)
	}
	idx, err := stable.IndexSongs(inst.Songs)
	if err != nil {
		return fmt.Errorf("index songs: %w", err)
	}

	var used []play
	var skipped []play
	seen := map[string]bool{}

	for _, item := range reps {
		if item.rep.ReplayMD5 != "" && seen[item.rep.ReplayMD5] {
			continue
		}
		if item.rep.ReplayMD5 != "" {
			seen[item.rep.ReplayMD5] = true
		}

		p := play{Replay: item.rep, Path: item.path}
		if reason := item.rep.SkipReason(); reason != "" {
			p.Skip = reason
			skipped = append(skipped, p)
			continue
		}
		osuPath, ok := idx[item.rep.BeatmapMD5]
		if !ok {
			p.Skip = "beatmap not in Songs (hash mismatch / deleted)"
			skipped = append(skipped, p)
			continue
		}
		bm, err := beatmap.ParseFile(osuPath)
		if err != nil {
			p.Skip = fmt.Sprintf("parse beatmap: %v", err)
			skipped = append(skipped, p)
			continue
		}
		if bm.Mode != 0 {
			p.Skip = "beatmap is not osu!standard"
			skipped = append(skipped, p)
			continue
		}
		p.Beatmap = bm
		p.Hits = hits.Reconstruct(bm, item.rep)
		if len(p.Hits.Errors) < *minHits {
			p.Skip = fmt.Sprintf("only %d timed hits (need %d)", len(p.Hits.Errors), *minHits)
			skipped = append(skipped, p)
			continue
		}
		p.Suggested = hits.SuggestedFromPlay(float64(inst.Offset), p.Hits.Median, p.Hits.UR, *dampen)
		used = append(used, p)
		if len(used) >= *plays {
			break
		}
	}

	out := report{
		OsuDir:        inst.Root,
		Config:        inst.Cfg,
		CurrentOffset: inst.Offset,
		PlaysScanned:  len(seen),
	}

	if len(used) == 0 {
		out.Note = "no plays had enough timed hits to calibrate"
		out.PlaysConsidered = 0
		if *verbose {
			out.Skipped = skippedJSON(skipped)
		}
		if *jsonOut {
			return printJSON(out)
		}
		printHeader(inst)
		fmt.Println("No usable plays. Need osu!standard scores with at least", *minHits, "timed hits")
		fmt.Println("(circles + slider heads). Lazer uses the same 50-hit minimum.")
		if *verbose {
			printSkipped(skipped)
		} else {
			fmt.Println("Re-run with -verbose to see why plays were skipped.")
		}
		return fmt.Errorf("nothing to calibrate from")
	}

	suggestions := make([]float64, 0, len(used))
	medians := make([]float64, 0, len(used))
	for _, p := range used {
		suggestions = append(suggestions, p.Suggested)
		medians = append(medians, p.Hits.Median)
		name := p.Beatmap.DisplayName()
		out.Plays = append(out.Plays, playJSON{
			Time:       p.Replay.Timestamp.Format(time.RFC3339),
			Player:     p.Replay.Player,
			Beatmap:    name,
			Mods:       p.Replay.ModString(),
			Hits:       len(p.Hits.Errors),
			Median:     round1(p.Hits.Median),
			Mean:       round1(p.Hits.Mean),
			UR:         round1(p.Hits.UR),
			Suggested:  round1(p.Suggested),
			UsedCursor: p.Hits.UsedCursor,
		})
	}
	avg := hits.Mean(suggestions)
	rec := hits.RoundOffset(avg)
	out.Recommended = &rec
	out.PlaysConsidered = len(used)
	out.MeanMedianError = round1(hits.Mean(medians))
	if *verbose {
		out.Skipped = skippedJSON(skipped)
	}

	if *jsonOut {
		return printJSON(out)
	}

	printHeader(inst)
	fmt.Printf("Current Offset:     %d ms\n", inst.Offset)
	fmt.Printf("Plays used:         %d (of %d unique replays, min %d timed hits)\n\n", len(used), len(seen), *minHits)

	fmt.Printf("%-19s  %-8s %7s %7s %6s  %s\n", "WHEN", "MODS", "MEDIAN", "UR", "n", "MAP")
	for _, p := range used {
		when := p.Replay.Timestamp.Local().Format("2006-01-02 15:04")
		fmt.Printf("%-19s  %-8s %+7.1f %+7.1f %6d  %s\n",
			when, p.Replay.ModString(), p.Hits.Median, p.Hits.UR, len(p.Hits.Errors), p.Beatmap.DisplayName())
	}

	meanErr := hits.Mean(medians)
	fmt.Printf("\nRecommended Offset: %d ms\n", rec)
	delta := rec - inst.Offset
	switch {
	case delta == 0:
		fmt.Println("That matches your current Offset — leave it.")
	case meanErr < 0:
		fmt.Printf("Hits are ~%.1f ms early. Raise Offset by %d ms (Options → Audio → Offset).\n", -round1(meanErr), delta)
	default:
		fmt.Printf("Hits are ~%.1f ms late. Lower Offset by %d ms (Options → Audio → Offset).\n", round1(meanErr), -delta)
	}
	fmt.Println("Wine audio latency is usually corrected with a negative Offset.")

	if *verbose {
		printSkipped(skipped)
	}

	if *apply {
		if err := inst.WriteOffset(rec); err != nil {
			return fmt.Errorf("write config: %w", err)
		}
		fmt.Printf("\nWrote Offset = %d to %s\n", rec, inst.Cfg)
		fmt.Println("Close osu! before applying, or the client may overwrite the file on exit.")
	}

	return nil
}

type replayFile struct {
	path string
	rep  *osr.Replay
}

func loadReplays(inst *stable.Install) ([]replayFile, error) {
	type cand struct {
		path string
		mod  time.Time
	}
	var cands []cand
	for _, dir := range inst.ReplayDirs() {
		matches, err := filepath.Glob(filepath.Join(dir, "*.osr"))
		if err != nil {
			return nil, err
		}
		for _, p := range matches {
			st, err := os.Stat(p)
			if err != nil {
				continue
			}
			cands = append(cands, cand{path: p, mod: st.ModTime()})
		}
	}
	sort.Slice(cands, func(i, j int) bool {
		return cands[i].mod.After(cands[j].mod)
	})

	var loaded []replayFile
	for _, c := range cands {
		rep, err := osr.ParseFile(c.path)
		if err != nil {
			continue
		}
		if rep.Timestamp.IsZero() {
			rep.Timestamp = c.mod.UTC()
		}
		loaded = append(loaded, replayFile{path: c.path, rep: rep})
	}
	sort.Slice(loaded, func(i, j int) bool {
		return loaded[i].rep.Timestamp.After(loaded[j].rep.Timestamp)
	})
	return loaded, nil
}

func printHeader(inst *stable.Install) {
	fmt.Println("osu!stable offset calculator (lazer-style median hit error)")
	fmt.Printf("Install:            %s\n", inst.Root)
	fmt.Printf("Config:             %s\n", inst.Cfg)
}

func printSkipped(skipped []play) {
	if len(skipped) == 0 {
		return
	}
	fmt.Println("\nSkipped:")
	for _, p := range skipped {
		when := p.Replay.Timestamp.Local().Format("2006-01-02 15:04")
		fmt.Printf("  %s  %s\n", when, p.Skip)
	}
}

func skippedJSON(skipped []play) []skipJSON {
	out := make([]skipJSON, 0, len(skipped))
	for _, p := range skipped {
		out = append(out, skipJSON{
			Time:   p.Replay.Timestamp.Format(time.RFC3339),
			Reason: p.Skip,
			Path:   p.Path,
		})
	}
	return out
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func round1(v float64) float64 {
	return math.Round(v*10) / 10
}

type report struct {
	OsuDir          string     `json:"osu_dir"`
	Config          string     `json:"config"`
	CurrentOffset   int        `json:"current_offset"`
	PlaysConsidered int        `json:"plays_considered"`
	PlaysScanned    int        `json:"plays_scanned"`
	Recommended     *int       `json:"recommended_offset"`
	MeanMedianError float64    `json:"mean_median_hit_error_ms"`
	Note            string     `json:"note,omitempty"`
	Plays           []playJSON `json:"plays"`
	Skipped         []skipJSON `json:"skipped,omitempty"`
}

type playJSON struct {
	Time       string  `json:"time"`
	Player     string  `json:"player"`
	Beatmap    string  `json:"beatmap"`
	Mods       string  `json:"mods"`
	Hits       int     `json:"timed_hits"`
	Median     float64 `json:"median_ms"`
	Mean       float64 `json:"mean_ms"`
	UR         float64 `json:"unstable_rate"`
	Suggested  float64 `json:"suggested_offset"`
	UsedCursor bool    `json:"used_cursor"`
}

type skipJSON struct {
	Time   string `json:"time"`
	Reason string `json:"reason"`
	Path   string `json:"path"`
}
