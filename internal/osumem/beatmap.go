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
	beatmapAddr, err := ReadPtr32(r.proc, r.baseAddr-0xc)
	if err != nil || beatmapAddr == 0 {
		return Beatmap{}, fmt.Errorf("beatmap pointer")
	}

	readField := func(off int64) (string, error) {
		ptr, err := ReadPtr32(r.proc, beatmapAddr+off)
		if err != nil || ptr == 0 {
			return "", err
		}
		return readSharpString(r.proc, ptr)
	}

	artist, _ := readField(0x18)
	title, _ := readField(0x24)
	version, _ := readField(0xac)

	b := Beatmap{Artist: artist, Title: title, Version: version}
	if b.Display() == "" {
		return Beatmap{}, fmt.Errorf("beatmap metadata empty")
	}
	return b, nil
}
