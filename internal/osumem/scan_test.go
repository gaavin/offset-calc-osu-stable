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
