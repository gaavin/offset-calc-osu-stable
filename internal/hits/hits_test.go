package hits

import (
	"math"
	"testing"

	"github.com/gaavin/offset-calc-osu-stable/internal/beatmap"
	"github.com/gaavin/offset-calc-osu-stable/internal/osr"
)

func TestMedianEvenOdd(t *testing.T) {
	if g := Median([]float64{1, 3, 2}); g != 2 {
		t.Fatalf("odd median: got %v", g)
	}
	if g := Median([]float64{1, 2, 3, 4}); g != 2.5 {
		t.Fatalf("even median: got %v", g)
	}
}

func TestSuggestedFromPlay(t *testing.T) {
	// Hitting 5ms late (positive median) with offset 0 → suggest -5.
	if g := SuggestedFromPlay(0, 5, 50, false); g != -5 {
		t.Fatalf("late hits: got %v", g)
	}
	// Hitting 5ms early with current -30 → suggest -25.
	if g := SuggestedFromPlay(-30, -5, 50, false); g != -25 {
		t.Fatalf("early hits: got %v", g)
	}
}

func TestSuggestedURDampen(t *testing.T) {
	undamped := SuggestedFromPlay(0, 10, 90, true)
	if undamped != -10 {
		t.Fatalf("at cutoff UR should be undamped, got %v", undamped)
	}
	damped := SuggestedFromPlay(0, 10, 190, true)
	if math.Abs(damped) >= 10 {
		t.Fatalf("high UR should shrink the adjustment, got %v", damped)
	}
}

func TestWindow50(t *testing.T) {
	if g := Window50(10); g != 100 {
		t.Fatalf("OD10 window50: got %v", g)
	}
	if g := Window50(5); g != 150 {
		t.Fatalf("OD5 window50: got %v", g)
	}
}

func TestCircleRadiusCS5(t *testing.T) {
	r := CircleRadius(5)
	if math.Abs(r-32.01312) > 0.01 {
		t.Fatalf("CS5 radius: got %v", r)
	}
}

func TestApplyMods(t *testing.T) {
	cs, od := ApplyMods(5, 5, osr.ModHardRock)
	if math.Abs(cs-6.5) > 1e-9 || math.Abs(od-7) > 1e-9 {
		t.Fatalf("HR: cs=%v od=%v", cs, od)
	}
	cs, od = ApplyMods(10, 10, osr.ModHardRock)
	if cs != 10 || od != 10 {
		t.Fatalf("HR cap: cs=%v od=%v", cs, od)
	}
	cs, od = ApplyMods(8, 8, osr.ModEasy)
	if cs != 4 || od != 4 {
		t.Fatalf("EZ: cs=%v od=%v", cs, od)
	}
}

func TestClicksRisingEdge(t *testing.T) {
	frames := []osr.Frame{
		{Delta: 100, Keys: 0},
		{Delta: 10, Keys: 5}, // K1+M1 down
		{Delta: 10, Keys: 5}, // held
		{Delta: 10, Keys: 0}, // up
		{Delta: 10, Keys: 5}, // second tap
	}
	c := Clicks(frames)
	if len(c) != 2 {
		t.Fatalf("want 2 clicks, got %d", len(c))
	}
	if c[0].Time != 110 || c[1].Time != 140 {
		t.Fatalf("click times: %+v", c)
	}
}

func TestReconstructLateBias(t *testing.T) {
	bm := &beatmap.Beatmap{CS: 4, OD: 8}
	for i := 0; i < 60; i++ {
		t0 := float64(1000 + i*400)
		bm.Objects = append(bm.Objects, beatmap.Object{X: 256, Y: 192, Time: t0})
	}
	var frames []osr.Frame
	tCursor := 0.0
	press := func(at, x, y float64) {
		frames = append(frames, osr.Frame{Delta: at - tCursor, X: x, Y: y, Keys: 0})
		tCursor = at
		frames = append(frames, osr.Frame{Delta: 1, X: x, Y: y, Keys: 5})
		tCursor += 1
		frames = append(frames, osr.Frame{Delta: 20, X: x, Y: y, Keys: 0})
		tCursor += 20
	}
	for i := 0; i < 60; i++ {
		objT := float64(1000 + i*400)
		press(objT+8, 256, 192) // 8ms late (plus 0 from the dummy delta frame)
	}
	rep := &osr.Replay{Frames: frames}
	res := Reconstruct(bm, rep)
	if len(res.Errors) < 50 {
		t.Fatalf("expected ~60 hits, got %d", len(res.Errors))
	}
	if math.Abs(res.Median-8) > 2 {
		t.Fatalf("median should be ~+8ms (late), got %v (n=%d errors=%v)", res.Median, len(res.Errors), res.Errors[:min(5, len(res.Errors))])
	}
	sug := SuggestedFromPlay(0, res.Median, res.UR, false)
	if sug > 0 {
		t.Fatalf("late hits should suggest a negative offset, got %v", sug)
	}
}

func TestRoundOffset(t *testing.T) {
	if RoundOffset(3.6) != 4 {
		t.Fatal("round 3.6")
	}
	if RoundOffset(-3.6) != -4 {
		t.Fatal("round -3.6")
	}
}
