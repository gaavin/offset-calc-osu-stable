package osumem

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"strings"
)

type Pattern struct {
	raw  []byte
	mask []byte
}

func ParsePattern(s string) (Pattern, error) {
	var raw, mask []byte
	for _, tok := range strings.Fields(s) {
		if tok == "??" || tok == "?" {
			raw = append(raw, 0)
			mask = append(mask, 0)
			continue
		}
		b, err := hex.DecodeString(tok)
		if err != nil || len(b) != 1 {
			return Pattern{}, fmt.Errorf("bad pattern byte %q", tok)
		}
		raw = append(raw, b[0])
		mask = append(mask, 0xff)
	}
	if len(raw) == 0 {
		return Pattern{}, fmt.Errorf("empty pattern")
	}
	return Pattern{raw: raw, mask: mask}, nil
}

func matchAt(buf []byte, p Pattern) bool {
	if len(buf) < len(p.raw) {
		return false
	}
	for i := range p.raw {
		if p.mask[i] == 0 {
			continue
		}
		if buf[i] != p.raw[i] {
			return false
		}
	}
	return true
}

func findIn(buf []byte, p Pattern) int {
	needle := byte(0)
	off := 0
	found := false
	for i, m := range p.mask {
		if m != 0 {
			needle = p.raw[i]
			off = i
			found = true
			break
		}
	}
	if !found {
		return 0
	}
	from := 0
	for {
		i := bytes.IndexByte(buf[from:], needle)
		if i < 0 {
			return -1
		}
		at := from + i - off
		if at >= 0 && at+len(p.raw) <= len(buf) && matchAt(buf[at:], p) {
			return at
		}
		from += i + 1
	}
}

func Scan(p Process, pattern string) (int64, error) {
	pat, err := ParsePattern(pattern)
	if err != nil {
		return 0, err
	}
	maps, err := p.Maps()
	if err != nil {
		return 0, err
	}
	try := func(onlyExec bool, preferOsu bool) (int64, bool) {
		for _, reg := range maps {
			if onlyExec && !reg.Exec {
				continue
			}
			if preferOsu && !strings.Contains(strings.ToLower(reg.Name), "osu!") {
				continue
			}
			if addr, ok := scanRegion(p, reg, pat); ok {
				return addr, true
			}
		}
		return 0, false
	}
	if addr, ok := try(true, true); ok {
		return addr, nil
	}
	if addr, ok := try(true, false); ok {
		return addr, nil
	}
	if addr, ok := try(false, false); ok {
		return addr, nil
	}
	return 0, fmt.Errorf("no memory matched the pattern: %s", pattern)
}

func scanRegion(p Process, reg Region, pat Pattern) (int64, bool) {
	const chunk = 64 * 1024
	overlap := len(pat.raw) - 1
	if overlap < 0 {
		overlap = 0
	}
	buf := make([]byte, chunk)
	for off := int64(0); off < reg.Size; {
		nwant := int64(chunk)
		if off+nwant > reg.Size {
			nwant = reg.Size - off
		}
		n, err := p.ReadAt(buf[:nwant], reg.Start+off)
		if err != nil || n < len(pat.raw) {
			off += nwant
			if nwant == 0 {
				break
			}
			continue
		}
		if at := findIn(buf[:n], pat); at >= 0 {
			return reg.Start + off + int64(at), true
		}
		step := int64(n - overlap)
		if step < 1 {
			step = 1
		}
		off += step
	}
	return 0, false
}
