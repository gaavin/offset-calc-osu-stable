package hits

import (
	"math"
	"sort"

	"github.com/gaavin/offset-calc-osu-stable/internal/beatmap"
	"github.com/gaavin/offset-calc-osu-stable/internal/osr"
)

const (
	playfieldHeight = 384.0
	urCutoff        = 90.0
	urExpFactor     = -0.0116
)

type Click struct {
	Time, X, Y float64
}

type Result struct {
	Errors     []float64
	Median     float64
	Mean       float64
	UR         float64
	UsedCursor bool
}

func ApplyMods(cs, od float64, mods uint32) (float64, float64) {
	if mods&osr.ModEasy != 0 {
		cs *= 0.5
		od *= 0.5
	}
	if mods&osr.ModHardRock != 0 {
		cs = math.Min(10, cs*1.3)
		od = math.Min(10, od*1.4)
	}
	return cs, od
}

// Window50 is the osu!standard 50 hit-window half-width in milliseconds.
func Window50(od float64) float64 {
	return math.Trunc(200 - 10*od)
}

// CircleRadius is the object radius in osu!pixels (stable formula).
func CircleRadius(cs float64) float64 {
	scale := (1.0 - 0.7*(cs-5)/5) / 2
	return 64 * scale * 1.00041
}

func Clicks(frames []osr.Frame) []Click {
	var t float64
	var prevK1, prevK2 bool
	clicks := make([]Click, 0, 256)
	for _, f := range frames {
		t += f.Delta
		k1 := f.Keys&(osr.KeyM1|osr.KeyK1) != 0
		k2 := f.Keys&(osr.KeyM2|osr.KeyK2) != 0
		if k1 && !prevK1 {
			clicks = append(clicks, Click{Time: t, X: f.X, Y: f.Y})
		}
		if k2 && !prevK2 {
			clicks = append(clicks, Click{Time: t, X: f.X, Y: f.Y})
		}
		prevK1, prevK2 = k1, k2
	}
	return clicks
}

func Reconstruct(bm *beatmap.Beatmap, rep *osr.Replay) Result {
	cs, od := ApplyMods(bm.CS, bm.OD, rep.Mods)
	w50 := Window50(od)
	if w50 < 1 {
		w50 = 1
	}
	radius := CircleRadius(cs)
	objects := bm.Objects
	if rep.Mods&osr.ModHardRock != 0 {
		flipped := make([]beatmap.Object, len(objects))
		for i, o := range objects {
			o.Y = playfieldHeight - o.Y
			flipped[i] = o
		}
		objects = flipped
	}
	clicks := Clicks(rep.Frames)

	errors, ok := match(objects, clicks, w50, radius)
	usedCursor := true
	if !ok {
		errors, _ = match(objects, clicks, w50, math.Inf(1))
		usedCursor = false
	}
	return summarize(errors, usedCursor)
}

func match(objects []beatmap.Object, clicks []Click, w50, radius float64) ([]float64, bool) {
	errors := make([]float64, 0, len(objects))
	ci := 0
	for _, obj := range objects {
		start := obj.Time - w50
		end := obj.Time + w50
		for ci < len(clicks) && clicks[ci].Time < start {
			ci++
		}
		hit := false
		for j := ci; j < len(clicks) && clicks[j].Time <= end; j++ {
			dx := clicks[j].X - obj.X
			dy := clicks[j].Y - obj.Y
			if dx*dx+dy*dy <= radius*radius {
				errors = append(errors, clicks[j].Time-obj.Time)
				ci = j + 1
				hit = true
				break
			}
		}
		if !hit {
			for ci < len(clicks) && clicks[ci].Time <= end {
				ci++
			}
		}
	}
	if len(objects) == 0 {
		return errors, false
	}
	// Cursor matching is trusted when we recovered a reasonable fraction of notes.
	ok := float64(len(errors)) >= 0.5*float64(len(objects)) && len(errors) >= 10
	return errors, ok
}

func summarize(errors []float64, usedCursor bool) Result {
	res := Result{Errors: errors, UsedCursor: usedCursor}
	if len(errors) == 0 {
		return res
	}
	sorted := append([]float64(nil), errors...)
	sort.Float64s(sorted)
	res.Median = medianSorted(sorted)
	var sum float64
	for _, e := range errors {
		sum += e
	}
	res.Mean = sum / float64(len(errors))
	res.UR = unstableRate(errors, res.Mean)
	return res
}

func medianSorted(sorted []float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	mid := n / 2
	if n%2 == 0 {
		return (sorted[mid-1] + sorted[mid]) / 2
	}
	return sorted[mid]
}

func unstableRate(errors []float64, mean float64) float64 {
	n := float64(len(errors))
	if n == 0 {
		return 0
	}
	var ss float64
	for _, e := range errors {
		d := e - mean
		ss += d * d
	}
	return 10 * math.Sqrt(ss/n)
}

// SuggestedFromPlay mirrors osu!lazer's per-play offset suggestion:
// currentOffset - median, with UR damping above 90 for beatmap calibration.
// Global calibration in lazer skips UR damping; pass dampen=false for that.
func SuggestedFromPlay(currentOffset, median, ur float64, dampen bool) float64 {
	adj := median
	if dampen && ur >= urCutoff {
		adj *= math.Exp(urExpFactor * (ur - urCutoff))
	}
	return currentOffset - adj
}

func Median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	s := append([]float64(nil), values...)
	sort.Float64s(s)
	return medianSorted(s)
}

func Mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var s float64
	for _, v := range values {
		s += v
	}
	return s / float64(len(values))
}

func RoundOffset(v float64) int {
	return int(math.Round(v))
}
