package osumem

import (
	"fmt"
	"strings"
	"time"
)

// Beatmap holds metadata for the map currently loaded in osu!stable.
type Beatmap struct {
	Artist  string
	Title   string
	Version string
}

// Display formats beatmap metadata like osu!'s song select line.
func (b Beatmap) Display() string {
	switch {
	case b.Artist != "" && b.Title != "":
		line := b.Artist + " - " + b.Title
		if b.Version != "" {
			line += " [" + b.Version + "]"
		}
		return line
	case b.Title != "":
		if b.Version != "" {
			return b.Title + " [" + b.Version + "]"
		}
		return b.Title
	case b.Artist != "":
		return b.Artist
	default:
		return ""
	}
}

// HasBeatmapBase reports whether a beatmap signature candidate is available.
func (r *Reader) HasBeatmapBase() bool {
	if r.baseAddr != 0 || len(r.baseAddrs) > 0 {
		return true
	}
	return r.ensureBaseAddr()
}

func (r *Reader) ensureBaseAddr() bool {
	if r.baseAddr != 0 {
		return true
	}
	return r.rescanBeatmapBases() == nil
}

func (r *Reader) rescanBeatmapBases() error {
	if !r.baseScanAt.IsZero() && time.Since(r.baseScanAt) < 2*time.Second {
		return fmt.Errorf("beatmap base not available")
	}
	r.baseScanAt = time.Now()
	addrs, err := ScanAll(r.proc, sigBase, 24)
	if err != nil || len(addrs) == 0 {
		return fmt.Errorf("beatmap base not available")
	}
	r.baseAddrs = addrs
	if r.baseAddr == 0 {
		r.baseAddr = addrs[0]
	}
	return nil
}

func (r *Reader) Beatmap() (Beatmap, error) {
	if b, ok := r.beatmapAt(r.baseAddr); ok {
		return b, nil
	}
	for _, addr := range r.baseAddrs {
		if addr == r.baseAddr {
			continue
		}
		if b, ok := r.beatmapAt(addr); ok {
			return b, nil
		}
	}
	if err := r.rescanBeatmapBases(); err != nil {
		if r.baseAddr == 0 && len(r.baseAddrs) == 0 {
			return Beatmap{}, err
		}
		return Beatmap{}, fmt.Errorf("beatmap metadata empty")
	}
	for _, addr := range r.baseAddrs {
		if b, ok := r.beatmapAt(addr); ok {
			return b, nil
		}
	}
	return Beatmap{}, fmt.Errorf("beatmap metadata empty")
}

func (r *Reader) beatmapAt(addr int64) (Beatmap, bool) {
	if addr == 0 {
		return Beatmap{}, false
	}
	b, err := readBeatmapAt(r.proc, addr)
	if err != nil {
		return Beatmap{}, false
	}
	r.baseAddr = addr
	return b, true
}

func readBeatmapAt(proc Process, baseAddr int64) (Beatmap, error) {
	// tosu: beatmapAddr = readPointer(baseAddr - 0xC) = [[baseAddr - 0xC]]
	beatmapAddr, err := readPointer(proc, baseAddr-0xc)
	if err != nil || beatmapAddr == 0 {
		return Beatmap{}, fmt.Errorf("beatmap pointer")
	}

	readField := func(off int64) string {
		ptr, err := ReadPtr32(proc, beatmapAddr+off)
		if err != nil || ptr == 0 {
			return ""
		}
		s, err := readSharpString(proc, ptr)
		if err != nil {
			return ""
		}
		return s
	}

	artist := readField(0x18)
	if artist == "" {
		artist = readField(0x1c) // ArtistUnicode
	}
	title := readField(0x24)
	if title == "" {
		title = readField(0x28) // TitleUnicode
	}
	version := readField(0xac)

	b := Beatmap{Artist: artist, Title: title, Version: version}
	if b.Display() == "" {
		if fn := readField(0x90); looksLikeOsuFile(fn) {
			b.Title = fn[:len(fn)-4]
		}
	}
	if b.Display() == "" {
		return Beatmap{}, fmt.Errorf("beatmap metadata empty")
	}
	return b, nil
}

func looksLikeOsuFile(name string) bool {
	return len(name) > 4 && strings.EqualFold(name[len(name)-4:], ".osu")
}
