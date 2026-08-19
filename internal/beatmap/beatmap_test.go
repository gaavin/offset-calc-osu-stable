package beatmap

import (
	"strings"
	"testing"
)

func TestParseMinimal(t *testing.T) {
	src := strings.NewReader(`osu file format v14
[General]
Mode: 0
[Metadata]
Title: Test
Artist: Unit
Creator: Me
Version: Hard
[Difficulty]
CircleSize: 4.5
OverallDifficulty: 8
[HitObjects]
256,192,1000,1,0,0:0:0:0:
100,100,1400,2,0,L|200:100,1,100
256,192,2000,8,0,2500
`)
	bm, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	if bm.Title != "Test" || bm.Artist != "Unit" || bm.Version != "Hard" {
		t.Fatalf("metadata: %+v", bm)
	}
	if bm.CS != 4.5 || bm.OD != 8 {
		t.Fatalf("diff: cs=%v od=%v", bm.CS, bm.OD)
	}
	if len(bm.Objects) != 2 {
		t.Fatalf("want circle+slider, skip spinner, got %d", len(bm.Objects))
	}
	if bm.Objects[0].Kind != KindCircle || bm.Objects[0].Time != 1000 {
		t.Fatalf("circle: %+v", bm.Objects[0])
	}
	if bm.Objects[1].Kind != KindSliderHead || bm.Objects[1].Time != 1400 {
		t.Fatalf("slider: %+v", bm.Objects[1])
	}
	if bm.DisplayName() != "Unit - Test [Hard]" {
		t.Fatalf("display: %q", bm.DisplayName())
	}
}
