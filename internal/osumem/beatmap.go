package osumem

import (
	"fmt"
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

// HasBeatmapBase reports whether the beatmap metadata signature has been found.
func (r *Reader) HasBeatmapBase() bool {
	return r.ensureBaseAddr()
}

func (r *Reader) ensureBaseAddr() bool {
	if r.baseAddr != 0 {
		return true
	}
	if !r.baseScanAt.IsZero() && time.Since(r.baseScanAt) < 2*time.Second {
		return false
	}
	r.baseScanAt = time.Now()
	base, err := Scan(r.proc, sigBase)
	if err != nil {
		return false
	}
	r.baseAddr = base
	return true
}

func (r *Reader) Beatmap() (Beatmap, error) {
	if !r.ensureBaseAddr() {
		return Beatmap{}, fmt.Errorf("beatmap base not available")
	}
	return readBeatmapAt(r.proc, r.baseAddr)
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
		return Beatmap{}, fmt.Errorf("beatmap metadata empty")
	}
	return b, nil
}
