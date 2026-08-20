package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gaavin/offset-calc-osu-stable/internal/hits"
	"github.com/gaavin/offset-calc-osu-stable/internal/osumem"
	"github.com/gaavin/offset-calc-osu-stable/internal/tui"
)

var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "osu-offset: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	jsonOut := flag.Bool("json", false, "print JSON instead of text")
	minHits := flag.Int("min-hits", 50, "minimum timed hits before recommending")
	once := flag.Bool("once", false, "exit after the first usable play instead of keeping watch")
	watch := flag.Bool("watch", true, "keep sampling osu! processes (default; use -once to exit)")
	poll := flag.Duration("poll", 50*time.Millisecond, "how often to read osu! memory")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `osu-offset %s — recommend a universal Offset from osu!stable hit error

Watches running osu!.exe processes (Windows, or Wine on Linux / NixOS) and
reads the current play's hit-error list and universal Offset from memory.

Leave it running. Play maps. Each finished play with enough hits prints the
Offset to set in Options → Audio → Offset.

`, version)
		flag.PrintDefaults()
	}
	flag.Parse()
	keepWatching := *watch && !*once
	ui := tui.New(tui.Enabled(*jsonOut), os.Stdout, os.Stderr)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	if !*jsonOut {
		ui.Banner(version, *minHits)
	}

	waitingLogged := false
	for {
		select {
		case <-stop:
			return fmt.Errorf("interrupted before any hits")
		default:
		}

		proc, err := osumem.OpenOsu()
		if err != nil {
			if !waitingLogged && !*jsonOut {
				ui.Waiting("waiting for osu!.exe …")
				waitingLogged = true
			}
			if err := waitOrStop(stop, time.Second); err != nil {
				return fmt.Errorf("interrupted before any hits")
			}
			continue
		}
		rd, err := osumem.Attach(proc)
		if err != nil {
			proc.Close()
			if !waitingLogged && !*jsonOut {
				ui.Waiting("osu!.exe found; waiting until memory signatures are readable …")
				waitingLogged = true
			}
			if err := waitOrStop(stop, time.Second); err != nil {
				return fmt.Errorf("interrupted before any hits")
			}
			continue
		}
		waitingLogged = false
		if !*jsonOut {
			ui.Attached(rd.Pid())
		}

		err = sampleSession(sessionOpts{
			rd:           rd,
			stop:         stop,
			minHits:      *minHits,
			poll:         *poll,
			jsonOut:      *jsonOut,
			keepWatching: keepWatching,
			ui:           ui,
		})
		proc.Close()
		if errors.Is(err, osumem.ErrGone) {
			if !*jsonOut {
				ui.ProcessExited()
			}
			waitingLogged = true
			continue
		}
		return err
	}
}

func waitOrStop(stop <-chan os.Signal, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-stop:
		return fmt.Errorf("stopped")
	case <-t.C:
		return nil
	}
}

type sessionOpts struct {
	rd           *osumem.Reader
	stop         <-chan os.Signal
	minHits      int
	poll         time.Duration
	jsonOut      bool
	keepWatching bool
	ui           *tui.Display
}

func sampleSession(o sessionOpts) error {
	tick := time.NewTicker(o.poll)
	defer tick.Stop()

	var (
		inPlay        bool
		best          []int32
		mapTitle      string
		beatmapWarned bool
	)

	for {
		select {
		case <-o.stop:
			if len(best) >= o.minHits {
				return finish(o.rd, best, mapTitle, o.jsonOut, o.ui)
			}
			if len(best) == 0 {
				return fmt.Errorf("interrupted before any hits")
			}
			return fmt.Errorf("interrupted with only %d hits (need %d)", len(best), o.minHits)
		case <-tick.C:
		}

		if !o.rd.Alive() {
			return osumem.ErrGone
		}

		st, err := o.rd.Status()
		if err != nil {
			if !o.rd.Alive() {
				return osumem.ErrGone
			}
			return fmt.Errorf("read status: %w", err)
		}

		playing := st == osumem.StatusPlaying && !o.rd.WatchingReplay()
		if playing {
			if !inPlay {
				inPlay = true
				best = nil
				mapTitle = readMapTitle(o.rd)
				maybeWarnBeatmap(o, &beatmapWarned, mapTitle)
				if !o.jsonOut {
					o.ui.PlayStarted(mapTitle)
				}
			}
			if mapTitle == "" {
				prev := beatmapWarned
				mapTitle = readMapTitle(o.rd)
				if !prev {
					maybeWarnBeatmap(o, &beatmapWarned, mapTitle)
				}
			}
			if mode, err := o.rd.Mode(); err == nil && mode != 0 {
				if !o.jsonOut {
					o.ui.SkipMode(mode)
				}
				best = nil
				continue
			}
			errs, err := o.rd.HitErrors()
			if err == nil && len(errs) >= len(best) {
				best = append([]int32(nil), errs...)
			}
			if !o.jsonOut && len(best) > 0 {
				med := hits.Median(hits.Int32ToFloat(best))
				cur, err := o.rd.Offset()
				ps := tui.PlayStats{
					HitCount: len(best),
					Errors:   hits.Int32ToFloat(best),
					Median:   med,
					MinHits:  o.minHits,
					MapTitle: mapTitle,
				}
				if err != nil {
					o.ui.UpdatePlay(ps)
				} else {
					ps.HasOffset = true
					ps.CurOffset = int(cur)
					ps.Recommend = hits.RoundOffset(hits.SuggestedOffset(float64(cur), med))
					o.ui.UpdatePlay(ps)
				}
			}
			continue
		}

		if inPlay {
			inPlay = false
			if mapTitle == "" {
				prev := beatmapWarned
				mapTitle = readMapTitle(o.rd)
				if !prev {
					maybeWarnBeatmap(o, &beatmapWarned, mapTitle)
				}
			}
			if len(best) < o.minHits {
				if !o.jsonOut {
					o.ui.PlayEndedShort(len(best), o.minHits, mapTitle)
				}
				best = nil
				mapTitle = ""
				continue
			}
			if o.keepWatching {
				if err := finish(o.rd, best, mapTitle, o.jsonOut, o.ui); err != nil {
					return err
				}
				best = nil
				mapTitle = ""
				if !o.jsonOut {
					o.ui.WatchingAnother()
				}
				continue
			}
			return finish(o.rd, best, mapTitle, o.jsonOut, o.ui)
		}
	}
}

func round1(v float64) float64 { return math.Round(v*10) / 10 }

func finish(rd *osumem.Reader, raw []int32, mapTitle string, jsonOut bool, ui *tui.Display) error {
	cur, err := rd.OffsetWithRetry(15 * time.Second)
	if err != nil {
		return err
	}
	if mapTitle == "" {
		mapTitle = readMapTitle(rd)
	}
	errors := hits.Int32ToFloat(raw)
	med := hits.Median(errors)
	mean := hits.Mean(errors)
	ur := hits.UnstableRate(errors)
	rec := hits.RoundOffset(hits.SuggestedOffset(float64(cur), med))

	if jsonOut {
		out := map[string]any{
			"hits":               len(errors),
			"median_ms":          round1(med),
			"mean_ms":            round1(mean),
			"unstable_rate":      round1(ur),
			"current_offset":     cur,
			"recommended_offset": rec,
		}
		if mapTitle != "" {
			out["map"] = mapTitle
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}

	if !jsonOut {
		ui.PlayEndedClear()
	}
	ui.PrintResult(tui.Result{
		Hits:          len(errors),
		Median:        med,
		Mean:          mean,
		UnstableRate:  ur,
		CurrentOffset: int(cur),
		Recommend:     rec,
		Errors:        errors,
		MapTitle:      mapTitle,
	})
	return nil
}

func readMapTitle(rd *osumem.Reader) string {
	b, err := rd.Beatmap()
	if err != nil {
		return ""
	}
	return b.Display()
}

func maybeWarnBeatmap(o sessionOpts, warned *bool, mapTitle string) {
	if *warned || o.jsonOut || mapTitle != "" || o.rd.HasBeatmapBase() {
		return
	}
	*warned = true
	o.ui.BeatmapUnavailable()
}
