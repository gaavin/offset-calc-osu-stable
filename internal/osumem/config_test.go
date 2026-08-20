package osumem

import (
	"encoding/binary"
	"io"
	"testing"
)

type memProc struct {
	mem  map[int64][]byte
	maps []Region
}

func (p memProc) Pid() int     { return 1 }
func (p memProc) Alive() bool  { return true }
func (p memProc) Close() error { return nil }
func (p memProc) Maps() ([]Region, error) {
	if p.maps != nil {
		return p.maps, nil
	}
	return nil, nil
}

func (p memProc) ReadAt(b []byte, off int64) (int, error) {
	if len(b) == 0 {
		return 0, nil
	}
	if chunk, ok := p.mem[off]; ok {
		n := copy(b, chunk)
		if n < len(b) {
			return n, io.ErrUnexpectedEOF
		}
		return n, nil
	}
	for start, data := range p.mem {
		if off >= start && off < start+int64(len(data)) {
			n := copy(b, data[off-start:])
			if n < len(b) {
				return n, io.ErrUnexpectedEOF
			}
			return n, nil
		}
	}
	return 0, io.ErrUnexpectedEOF
}

func putI32(p memProc, addr int64, v int32) {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], uint32(v))
	p.mem[addr] = append([]byte(nil), b[:]...)
}

func TestReadSharpString32BitLayout(t *testing.T) {
	p := memProc{mem: map[int64][]byte{}}
	const addr int64 = 0x1000
	putI32(p, addr+4, 6)
	raw := make([]byte, 12)
	for i, r := range []rune("Offset") {
		binary.LittleEndian.PutUint16(raw[i*2:], uint16(r))
	}
	p.mem[addr+8] = raw
	got, err := readSharpString(p, addr)
	if err != nil {
		t.Fatal(err)
	}
	if got != "Offset" {
		t.Fatalf("got %q", got)
	}
}

func TestFindConfigOffsetIndex(t *testing.T) {
	p := memProc{mem: map[int64][]byte{}}
	const (
		config int64 = 0x2000
		table  int64 = 0x3000
		key    int64 = 0x4000
	)
	putI32(p, config+0x8, int32(table))
	putI32(p, config+0x1c, 1)
	putI32(p, table+0x8, int32(key))
	putI32(p, key+4, 6)
	raw := make([]byte, 12)
	for i, r := range []rune("Offset") {
		binary.LittleEndian.PutUint16(raw[i*2:], uint16(r))
	}
	p.mem[key+8] = raw
	idx, err := findConfigOffsetIndex(p, config)
	if err != nil {
		t.Fatal(err)
	}
	if idx != 0 {
		t.Fatalf("idx %d", idx)
	}
}
