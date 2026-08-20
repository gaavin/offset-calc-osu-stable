package osumem

import (
	"encoding/binary"
	"testing"
)

func putString(p memProc, addr int64, s string) {
	putI32(p, addr+4, int32(len(s)))
	raw := make([]byte, len(s)*2)
	for i, r := range s {
		binary.LittleEndian.PutUint16(raw[i*2:], uint16(r))
	}
	p.mem[addr+8] = raw
}

func TestBeatmapDisplay(t *testing.T) {
	tests := []struct {
		b    Beatmap
		want string
	}{
		{Beatmap{Artist: "Camellia", Title: "GHOST", Version: "Hyper"}, "Camellia - GHOST [Hyper]"},
		{Beatmap{Title: "Title Only", Version: "Hard"}, "Title Only [Hard]"},
		{Beatmap{Artist: "Artist Only"}, "Artist Only"},
		{Beatmap{}, ""},
	}
	for _, tc := range tests {
		if got := tc.b.Display(); got != tc.want {
			t.Fatalf("%+v: got %q want %q", tc.b, got, tc.want)
		}
	}
}

func TestReaderBeatmap(t *testing.T) {
	const (
		base    int64 = 0x5000
		beatmap int64 = 0x6000
		artist  int64 = 0x7000
		title   int64 = 0x7100
		version int64 = 0x7200
	)

	p := memProc{mem: map[int64][]byte{}}
	putI32(p, base-0xc, int32(beatmap))
	putI32(p, beatmap+0x18, int32(artist))
	putI32(p, beatmap+0x24, int32(title))
	putI32(p, beatmap+0xac, int32(version))
	putString(p, artist, "Camellia")
	putString(p, title, "GHOST")
	putString(p, version, "Hyper")

	rd := &Reader{proc: p, baseAddr: base}
	got, err := rd.Beatmap()
	if err != nil {
		t.Fatal(err)
	}
	if want := "Camellia - GHOST [Hyper]"; got.Display() != want {
		t.Fatalf("got %q want %q", got.Display(), want)
	}
}
