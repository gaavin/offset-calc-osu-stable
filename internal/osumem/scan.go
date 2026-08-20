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

func findAllIn(buf []byte, p Pattern) []int {
	var out []int
	from := 0
	for {
		rel := findIn(buf[from:], p)
		if rel < 0 {
			return out
		}
		at := from + rel
		out = append(out, at)
		from = at + 1
	}
}

func Scan(p Process, pattern string) (int64, error) {
	addrs, err := scanMatches(p, pattern, 1)
	if err != nil {
		return 0, err
	}
	return addrs[0], nil
}

func ScanAll(p Process, pattern string, limit int) ([]int64, error) {
	if limit <= 0 {
		limit = 32
	}
	return scanMatches(p, pattern, limit)
}

func scanMatches(p Process, pattern string, limit int) ([]int64, error) {
	pat, err := ParsePattern(pattern)
	if err != nil {
		return nil, err
	}
	maps, err := p.Maps()
	if err != nil {
		return nil, err
	}
	try := func(want func(Region) bool) []int64 {
		var out []int64
		for _, reg := range maps {
			if len(out) >= limit || !reg.Exec || !want(reg) {
				continue
			}
			out = scanRegionAll(p, reg, pat, out, limit)
		}
		return out
	}
	namedOsu := func(reg Region) bool {
		return strings.Contains(strings.ToLower(reg.Name), "osu!")
	}
	anon := func(reg Region) bool {
		n := strings.TrimSpace(reg.Name)
		return n == "" || strings.HasPrefix(n, "[")
	}
	anyExec := func(Region) bool { return true }
	// Code signatures live in executable pages. Scanning every readable Wine
	// mapping can take minutes and looks like a hang. Prefer osu!-named
	// modules; only fall through if that class has no hits.
	if out := try(namedOsu); len(out) > 0 {
		return out, nil
	}
	if out := try(anon); len(out) > 0 {
		return out, nil
	}
	if out := try(anyExec); len(out) > 0 {
		return out, nil
	}
	return nil, fmt.Errorf("no memory matched the pattern: %s", pattern)
}

func scanRegionAll(p Process, reg Region, pat Pattern, dst []int64, limit int) []int64 {
	const chunk = 64 * 1024
	overlap := len(pat.raw) - 1
	if overlap < 0 {
		overlap = 0
	}
	buf := make([]byte, chunk)
	var last int64 = -1
	for off := int64(0); off < reg.Size && len(dst) < limit; {
		nwant := int64(chunk)
		if off+nwant > reg.Size {
			nwant = reg.Size - off
		}
		if nwant <= 0 {
			break
		}
		n, _ := p.ReadAt(buf[:nwant], reg.Start+off)
		if n < len(pat.raw) {
			off += nwant
			continue
		}
		for _, at := range findAllIn(buf[:n], pat) {
			addr := reg.Start + off + int64(at)
			if addr <= last {
				continue
			}
			dst = append(dst, addr)
			last = addr
			if len(dst) >= limit {
				return dst
			}
		}
		step := int64(n - overlap)
		if step < 1 {
			step = 1
		}
		off += step
	}
	return dst
}
