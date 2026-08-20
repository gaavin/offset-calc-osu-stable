package osumem

import "testing"

func TestParseAndMatch(t *testing.T) {
	p, err := ParsePattern("48 83 F8 04 73 1E")
	if err != nil {
		t.Fatal(err)
	}
	buf := []byte{0, 0x48, 0x83, 0xF8, 0x04, 0x73, 0x1E, 9}
	if at := findIn(buf, p); at != 1 {
		t.Fatalf("got %d", at)
	}
	p, err = ParsePattern("7D 15 A1 ?? ?? ?? ?? 85 C0")
	if err != nil {
		t.Fatal(err)
	}
	buf = []byte{0x7D, 0x15, 0xA1, 0x11, 0x22, 0x33, 0x44, 0x85, 0xC0}
	if at := findIn(buf, p); at != 0 {
		t.Fatalf("wildcard got %d", at)
	}
}

func TestFindAllIn(t *testing.T) {
	p, err := ParsePattern("F8 01 74 04 83 65")
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 32)
	copy(buf[2:], p.raw)
	copy(buf[20:], p.raw)
	got := findAllIn(buf, p)
	if len(got) != 2 || got[0] != 2 || got[1] != 20 {
		t.Fatalf("got %v", got)
	}
}

func TestScanAllMultiple(t *testing.T) {
	pat := []byte{0xF8, 0x01, 0x74, 0x04, 0x83, 0x65}
	buf := make([]byte, 256)
	copy(buf[16:], pat)
	copy(buf[80:], pat)
	proc := memProc{
		mem:  map[int64][]byte{0x1000: buf},
		maps: []Region{{Start: 0x1000, Size: int64(len(buf)), Exec: true, Name: "osu!.exe"}},
	}
	addrs, err := ScanAll(proc, "F8 01 74 04 83 65", 8)
	if err != nil {
		t.Fatal(err)
	}
	if len(addrs) != 2 || addrs[0] != 0x1010 || addrs[1] != 0x1050 {
		t.Fatalf("addrs=%v", addrs)
	}
	first, err := Scan(proc, "F8 01 74 04 83 65")
	if err != nil {
		t.Fatal(err)
	}
	if first != addrs[0] {
		t.Fatalf("Scan=%d ScanAll[0]=%d", first, addrs[0])
	}
}
