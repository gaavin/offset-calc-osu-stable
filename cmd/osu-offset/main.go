package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gaavin/offset-calc-osu-stable/internal/hits"
	"github.com/gaavin/offset-calc-osu-stable/internal/osumem"
	"github.com/gaavin/offset-calc-osu-stable/internal/tui"
)

var version = "dev"

var errInterrupted = errors.New("interrupted before any hits")

type jsonResult struct {
	Hits              int     `json:"hits"`
	MedianMs          float64 `json:"median_ms"`
	MeanMs            float64 `json:"mean_ms"`
	UnstableRate      float64 `json:"unstable_rate"`
	CurrentOffset     int32   `json:"current_offset"`
	RecommendedOffset int     `json:"recommended_offset"`
	Map               string  `json:"map,omitempty"`
}

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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if !*jsonOut {
		ui.Banner(version, *minHits)
	}

	produced := false
	waitingLogged := false
	for {
		if ctx.Err() != nil {
			if produced {
				return nil
			}
			return errInterrupted
		}

		proc, err := osumem.OpenOsu()
		if err != nil {
			if !waitingLogged && !*jsonOut {
				ui.Waiting("waiting for osu!.exe …")
				waitingLogged = true
			}
			if err := waitOrStop(ctx, time.Second); err != nil {
				if produced {
					return nil
				}
				return errInterrupted
			}
			continue
		}
		rd, err := osumem.Attach(proc)
		if err != nil {
			_ = proc.Close()
			if !waitingLogged && !*jsonOut {
				ui.Waiting("osu!.exe found; waiting until memory signatures are readable …")
				waitingLogged = true
			}
			if err := waitOrStop(ctx, time.Second); err != nil {
				if produced {
					return nil
				}
				return errInterrupted
			}
			continue
		}
		waitingLogged = false
		if !*jsonOut {
			ui.Attached(rd.Pid())
		}

		had, err := sampleSession(sessionOpts{
			ctx:          ctx,
			rd:           rd,
			minHits:      *minHits,
			poll:         *poll,
			jsonOut:      *jsonOut,
			keepWatching: keepWatching,
			ui:           ui,
		})
		_ = rd.Close()
		produced = produced || had
		if errors.Is(err, osumem.ErrGone) {
			if !*jsonOut {
				ui.ProcessExited()
			}
			waitingLogged = true
			continue
		}
		if err != nil && produced && errors.Is(err, errInterrupted) {
			return nil
		}
		return err
	}
}

func waitOrStop(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

type sessionOpts struct {
	ctx          context.Context
	rd           *osumem.Reader
	minHits      int
	poll         time.Duration
	jsonOut      bool
	keepWatching bool
	ui           *tui.Display
}

func sampleSession(o sessionOpts) (hadResult bool, err error) {
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
		case <-o.ctx.Done():
			if len(best) >= o.minHits {
				return true, finish(o.rd, best, mapTitle, o.jsonOut, o.ui)
			}
			if len(best) == 0 {
				return hadResult, errInterrupted
			}
			return hadResult, fmt.Errorf("interrupted with only %d hits (need %d)", len(best), o.minHits)
		case <-tick.C:
		}

		if !o.rd.Alive() {
			return hadResult, osumem.ErrGone
		}

		st, err := o.rd.Status()
		if err != nil {
			if !o.rd.Alive() {
				return hadResult, osumem.ErrGone
			}
			continue
		}

		playing := st == osumem.StatusPlaying && !o.rd.WatchingReplay()
		if playing {
			if !inPlay {
				inPlay = true
				best = nil
				mapTitle = readMapTitle(o.rd)
				if !o.jsonOut {
					o.ui.PlayStarted(mapTitle)
				}
			}
			if mapTitle == "" {
				mapTitle = readMapTitle(o.rd)
				maybeWarnBeatmap(o, &beatmapWarned, mapTitle)
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
				mapTitle = readMapTitle(o.rd)
				maybeWarnBeatmap(o, &beatmapWarned, mapTitle)
			}
			if len(best) < o.minHits {
				if !o.jsonOut {
					o.ui.PlayEndedShort(len(best), o.minHits, mapTitle)
				}
				best = nil
				mapTitle = ""
				continue
			}
			if err := finish(o.rd, best, mapTitle, o.jsonOut, o.ui); err != nil {
				return hadResult, err
			}
			hadResult = true
			best = nil
			mapTitle = ""
			if !o.keepWatching {
				return true, nil
			}
			if !o.jsonOut {
				o.ui.WatchingAnother()
			}
		}
	}
}

func finish(rd *osumem.Reader, raw []int32, mapTitle string, jsonOut bool, ui *tui.Display) error {
	cur, err := rd.FinishOffset()
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
		out := jsonResult{
			Hits:              len(errors),
			MedianMs:          hits.Round1(med),
			MeanMs:            hits.Round1(mean),
			UnstableRate:      hits.Round1(ur),
			CurrentOffset:     cur,
			RecommendedOffset: rec,
			Map:               mapTitle,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}

	ui.PlayEndedClear()
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
	if *warned || o.jsonOut || mapTitle != "" {
		return
	}
	*warned = true
	o.ui.BeatmapUnavailable()
}
