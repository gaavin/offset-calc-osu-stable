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
	"github.com/gaavin/offset-calc-osu-stable/internal/stable"
)

var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "osu-offset: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	dir := flag.String("dir", "", "osu!stable directory (for current Offset / -apply)")
	apply := flag.Bool("apply", false, "write the recommended Offset into the osu! config")
	jsonOut := flag.Bool("json", false, "print JSON instead of text")
	minHits := flag.Int("min-hits", 50, "minimum timed hits before recommending")
	once := flag.Bool("once", false, "exit after the first usable play instead of keeping watch")
	watch := flag.Bool("watch", true, "keep sampling osu! processes (default; use -once to exit)")
	poll := flag.Duration("poll", 50*time.Millisecond, "how often to read osu! memory")
	debugPaths := flag.Bool("debug-paths", false, "print install-path candidates and exit")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `osu-offset %s — recommend a universal Offset from live osu!stable hit error

Watches running osu!.exe processes (Windows, or Wine on Linux / NixOS) and
reads the current play's hit-error list from memory (the same values as the
in-game error bar). That uses the Offset and audio setup you have right now,
unlike old .osr replays.

Leave it running. Play maps. Each finished play with enough hits prints the
Offset to set in Options → Audio → Offset.

`, version)
		flag.PrintDefaults()
	}
	flag.Parse()
	keepWatching := *watch && !*once

	if *debugPaths {
		return stable.PrintDebug(os.Stdout)
	}

	inst, instErr := stable.Detect(*dir)
	if instErr != nil && *apply {
		return instErr
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	if !*jsonOut {
		fmt.Fprintf(os.Stderr, "osu-offset %s — watching for osu!.exe (live hit error, not replays)\n", version)
		if inst != nil {
			fmt.Fprintf(os.Stderr, "Current Offset: %d ms (%s)\n", inst.Offset, inst.Cfg)
		} else {
			fmt.Fprintf(os.Stderr, "Could not find osu! config (%v); Offset assumed 0.\n", instErr)
		}
		fmt.Fprintf(os.Stderr, "Need ≥ %d timed hits on a standard play. Ctrl+C to stop.\n", *minHits)
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
				fmt.Fprintln(os.Stderr, "waiting for osu!.exe …")
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
				fmt.Fprintln(os.Stderr, "osu!.exe found; waiting until memory signatures are readable …")
				waitingLogged = true
			}
			if err := waitOrStop(stop, time.Second); err != nil {
				return fmt.Errorf("interrupted before any hits")
			}
			continue
		}
		waitingLogged = false
		if !*jsonOut {
			fmt.Fprintf(os.Stderr, "attached to osu!.exe pid %d\n", rd.Pid())
		}

		err = sampleSession(sessionOpts{
			rd:           rd,
			inst:         inst,
			stop:         stop,
			minHits:      *minHits,
			poll:         *poll,
			jsonOut:      *jsonOut,
			keepWatching: keepWatching,
			apply:        *apply && !keepWatching,
		})
		proc.Close()
		if errors.Is(err, osumem.ErrGone) {
			if !*jsonOut {
				fmt.Fprintln(os.Stderr, "osu!.exe exited; watching for it to come back …")
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
	inst         *stable.Install
	stop         <-chan os.Signal
	minHits      int
	poll         time.Duration
	jsonOut      bool
	keepWatching bool
	apply        bool
}

func sampleSession(o sessionOpts) error {
	tick := time.NewTicker(o.poll)
	defer tick.Stop()

	var (
		inPlay bool
		best   []int32
	)

	for {
		select {
		case <-o.stop:
			if len(best) >= o.minHits {
				return finish(o.inst, best, o.apply, o.jsonOut)
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
				if !o.jsonOut {
					fmt.Fprintln(os.Stderr, "play started")
				}
			}
			if mode, err := o.rd.Mode(); err == nil && mode != 0 {
				if !o.jsonOut {
					fmt.Fprintf(os.Stderr, "\r  skipping non-standard mode %d          ", mode)
				}
				best = nil
				continue
			}
			errs, err := o.rd.HitErrors()
			if err == nil && len(errs) >= len(best) {
				best = append([]int32(nil), errs...)
			}
			if !o.jsonOut && len(best) > 0 {
				cur := 0
				if o.inst != nil {
					cur = o.inst.Offset
				}
				med := hits.Median(hits.Int32ToFloat(best))
				rec := hits.RoundOffset(hits.SuggestedOffset(float64(cur), med))
				fmt.Fprintf(os.Stderr, "\r  %d hits  median %+.1f ms  → Offset %d     ", len(best), med, rec)
			}
			continue
		}

		if inPlay {
			inPlay = false
			if !o.jsonOut {
				fmt.Fprintln(os.Stderr)
			}
			if len(best) < o.minHits {
				if !o.jsonOut {
					fmt.Fprintf(os.Stderr, "play ended with %d hits (need %d); waiting for another map\n", len(best), o.minHits)
				}
				best = nil
				continue
			}
			if o.keepWatching {
				if err := finish(o.inst, best, false, o.jsonOut); err != nil {
					return err
				}
				best = nil
				if !o.jsonOut {
					fmt.Fprintln(os.Stderr, "watching for another play")
				}
				continue
			}
			return finish(o.inst, best, o.apply, o.jsonOut)
		}
	}
}

func round1(v float64) float64 { return math.Round(v*10) / 10 }

func finish(inst *stable.Install, raw []int32, apply, jsonOut bool) error {
	errors := hits.Int32ToFloat(raw)
	med := hits.Median(errors)
	mean := hits.Mean(errors)
	ur := hits.UnstableRate(errors)
	cur := 0
	if inst != nil {
		cur = inst.Offset
	}
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
		if inst != nil {
			out["osu_dir"] = inst.Root
			out["config"] = inst.Cfg
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}

	fmt.Printf("Hits:               %d\n", len(errors))
	fmt.Printf("Median hit error:   %+.1f ms\n", med)
	fmt.Printf("Mean hit error:     %+.1f ms\n", mean)
	fmt.Printf("Unstable rate:      %.1f\n", ur)
	fmt.Printf("Current Offset:     %d ms\n", cur)
	fmt.Printf("Recommended Offset: %d ms\n", rec)
	delta := rec - cur
	switch {
	case delta == 0:
		fmt.Println("That matches your current Offset — leave it.")
	case med < 0:
		fmt.Printf("Hits are ~%.1f ms early. Raise Offset by %d ms.\n", -round1(med), delta)
	default:
		fmt.Printf("Hits are ~%.1f ms late. Lower Offset by %d ms.\n", round1(med), -delta)
	}
	fmt.Println("Set it in Options → Audio → Offset. Close osu! before -apply, or the client may overwrite the file.")

	if apply {
		if inst == nil {
			return fmt.Errorf("cannot -apply: osu! config not found")
		}
		if err := inst.WriteOffset(rec); err != nil {
			return fmt.Errorf("write config: %w", err)
		}
		fmt.Printf("Wrote Offset = %d to %s\n", rec, inst.Cfg)
	}
	return nil
}
