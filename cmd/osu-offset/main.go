package main

import (
	"encoding/json"
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
	watch := flag.Bool("watch", false, "keep sampling after each play instead of exiting")
	poll := flag.Duration("poll", 50*time.Millisecond, "how often to read osu! memory")
	debugPaths := flag.Bool("debug-paths", false, "print install-path candidates and exit")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `osu-offset %s — recommend a universal Offset from live osu!stable hit error

Attaches to a running osu!.exe and reads the current play's hit-error list
from memory (the same values as the in-game error bar). That uses the Offset
and audio setup you have right now, unlike old .osr replays.

Play a map, then set Options → Audio → Offset to the printed value.

`, version)
		flag.PrintDefaults()
	}
	flag.Parse()

	if *debugPaths {
		return stable.PrintDebug(os.Stdout)
	}

	inst, instErr := stable.Detect(*dir)
	if instErr != nil && *apply {
		return instErr
	}

	proc, err := osumem.OpenOsu()
	if err != nil {
		return err
	}
	defer proc.Close()

	rd, err := osumem.Attach(proc)
	if err != nil {
		return fmt.Errorf("scan osu! memory (is this osu!stable, not lazer?): %w", err)
	}

	if !*jsonOut {
		fmt.Fprintf(os.Stderr, "osu-offset %s — reading live hit error from osu!.exe pid %d\n", version, rd.Pid())
		if inst != nil {
			fmt.Fprintf(os.Stderr, "Current Offset: %d ms (%s)\n", inst.Offset, inst.Cfg)
		} else {
			fmt.Fprintf(os.Stderr, "Could not find osu! config (%v); Offset assumed 0.\n", instErr)
		}
		fmt.Fprintf(os.Stderr, "Enter a map. Recommendation prints when the play ends (need ≥ %d hits).\n", *minHits)
	}

	ctxStop := make(chan os.Signal, 1)
	signal.Notify(ctxStop, os.Interrupt, syscall.SIGTERM)

	var (
		inPlay bool
		best   []int32
	)

	tick := time.NewTicker(*poll)
	defer tick.Stop()

	for {
		select {
		case <-ctxStop:
			if len(best) >= *minHits {
				return finish(inst, best, *apply, *jsonOut)
			}
			if len(best) == 0 {
				return fmt.Errorf("interrupted before any hits")
			}
			return fmt.Errorf("interrupted with only %d hits (need %d)", len(best), *minHits)
		case <-tick.C:
		}

		st, err := rd.Status()
		if err != nil {
			return fmt.Errorf("read status: %w", err)
		}

		playing := st == osumem.StatusPlaying && !rd.WatchingReplay()
		if playing {
			if !inPlay {
				inPlay = true
				best = nil
				if !*jsonOut {
					fmt.Fprintln(os.Stderr, "play started")
				}
			}
			if mode, err := rd.Mode(); err == nil && mode != 0 {
				if !*jsonOut {
					fmt.Fprintf(os.Stderr, "\r  skipping non-standard mode %d          ", mode)
				}
				best = nil
				continue
			}
			errs, err := rd.HitErrors()
			if err == nil && len(errs) >= len(best) {
				best = append([]int32(nil), errs...)
			}
			if !*jsonOut && len(best) > 0 {
				cur := 0
				if inst != nil {
					cur = inst.Offset
				}
				med := hits.Median(hits.Int32ToFloat(best))
				rec := hits.RoundOffset(hits.SuggestedOffset(float64(cur), med))
				fmt.Fprintf(os.Stderr, "\r  %d hits  median %+.1f ms  → Offset %d     ", len(best), med, rec)
			}
			continue
		}

		if inPlay {
			inPlay = false
			if !*jsonOut {
				fmt.Fprintln(os.Stderr)
			}
			if len(best) < *minHits {
				if !*jsonOut {
					fmt.Fprintf(os.Stderr, "play ended with %d hits (need %d); waiting for another map\n", len(best), *minHits)
				}
				best = nil
				continue
			}
			if *watch {
				if err := finish(inst, best, false, *jsonOut); err != nil {
					return err
				}
				best = nil
				if !*jsonOut {
					fmt.Fprintln(os.Stderr, "waiting for another play (-watch)")
				}
				continue
			}
			return finish(inst, best, *apply, *jsonOut)
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
