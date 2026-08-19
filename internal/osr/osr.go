package osr

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ulikunitz/xz/lzma"
)

const (
	ModeOsu   = 0
	ModeTaiko = 1
	ModeCatch = 2
	ModeMania = 3
)

const (
	KeyM1    = 1
	KeyM2    = 2
	KeyK1    = 4
	KeyK2    = 8
	KeySmoke = 16
)

const (
	ModNoFail     uint32 = 1 << 0
	ModEasy       uint32 = 1 << 1
	ModHidden     uint32 = 1 << 3
	ModHardRock   uint32 = 1 << 4
	ModDoubleTime uint32 = 1 << 6
	ModRelax      uint32 = 1 << 7
	ModHalfTime   uint32 = 1 << 8
	ModNightcore  uint32 = 1 << 9
	ModFlashlight uint32 = 1 << 10
	ModAutoplay   uint32 = 1 << 11
	ModSpunOut    uint32 = 1 << 12
	ModAutopilot  uint32 = 1 << 13
	ModPerfect    uint32 = 1 << 14
	ModCinema     uint32 = 1 << 22
)

// Replay is a parsed osu!stable .osr file.
type Replay struct {
	Mode       byte
	OsuVersion int32
	BeatmapMD5 string
	Player     string
	ReplayMD5  string
	Count300   uint16
	Count100   uint16
	Count50    uint16
	CountGeki  uint16
	CountKatu  uint16
	CountMiss  uint16
	Score      int32
	MaxCombo   uint16
	Perfect    bool
	Mods       uint32
	Timestamp  time.Time
	ScoreID    int64
	Frames     []Frame
}

// Frame is one replay sample. Delta is milliseconds since the previous frame.
type Frame struct {
	Delta float64
	X     float64
	Y     float64
	Keys  int
}

func ParseFile(path string) (*Replay, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(data)
}

func Parse(data []byte) (*Replay, error) {
	r := bytes.NewReader(data)
	rep := &Replay{}

	var err error
	if rep.Mode, err = readU8(r); err != nil {
		return nil, fmt.Errorf("mode: %w", err)
	}
	if rep.OsuVersion, err = readI32(r); err != nil {
		return nil, fmt.Errorf("version: %w", err)
	}
	if rep.BeatmapMD5, err = readOsuString(r); err != nil {
		return nil, fmt.Errorf("beatmap md5: %w", err)
	}
	if rep.Player, err = readOsuString(r); err != nil {
		return nil, fmt.Errorf("player: %w", err)
	}
	if rep.ReplayMD5, err = readOsuString(r); err != nil {
		return nil, fmt.Errorf("replay md5: %w", err)
	}
	if rep.Count300, err = readU16(r); err != nil {
		return nil, fmt.Errorf("count300: %w", err)
	}
	if rep.Count100, err = readU16(r); err != nil {
		return nil, fmt.Errorf("count100: %w", err)
	}
	if rep.Count50, err = readU16(r); err != nil {
		return nil, fmt.Errorf("count50: %w", err)
	}
	if rep.CountGeki, err = readU16(r); err != nil {
		return nil, fmt.Errorf("geki: %w", err)
	}
	if rep.CountKatu, err = readU16(r); err != nil {
		return nil, fmt.Errorf("katu: %w", err)
	}
	if rep.CountMiss, err = readU16(r); err != nil {
		return nil, fmt.Errorf("miss: %w", err)
	}
	if rep.Score, err = readI32(r); err != nil {
		return nil, fmt.Errorf("score: %w", err)
	}
	if rep.MaxCombo, err = readU16(r); err != nil {
		return nil, fmt.Errorf("combo: %w", err)
	}
	var perfect byte
	if perfect, err = readU8(r); err != nil {
		return nil, fmt.Errorf("perfect: %w", err)
	}
	rep.Perfect = perfect != 0
	if rep.Mods, err = readU32(r); err != nil {
		return nil, fmt.Errorf("mods: %w", err)
	}
	if _, err = readOsuString(r); err != nil {
		return nil, fmt.Errorf("lifebar: %w", err)
	}
	var ticks int64
	if ticks, err = readI64(r); err != nil {
		return nil, fmt.Errorf("timestamp: %w", err)
	}
	rep.Timestamp = timeFromTicks(ticks)

	var compressedLen int32
	if compressedLen, err = readI32(r); err != nil {
		return nil, fmt.Errorf("replay length: %w", err)
	}
	if compressedLen > 0 {
		compressed := make([]byte, compressedLen)
		if _, err = io.ReadFull(r, compressed); err != nil {
			return nil, fmt.Errorf("replay data: %w", err)
		}
		frames, err := parseCompressed(compressed)
		if err != nil {
			return nil, fmt.Errorf("decompress frames: %w", err)
		}
		rep.Frames = frames
	}

	if r.Len() >= 8 {
		if rep.ScoreID, err = readI64(r); err != nil {
			return nil, fmt.Errorf("score id: %w", err)
		}
	}

	return rep, nil
}

func (r *Replay) SkipReason() string {
	if r.Mode != ModeOsu {
		return fmt.Sprintf("not osu!standard (mode %d)", r.Mode)
	}
	if r.Mods&ModRelax != 0 {
		return "relax"
	}
	if r.Mods&ModAutopilot != 0 {
		return "autopilot"
	}
	if r.Mods&ModAutoplay != 0 || r.Mods&ModCinema != 0 {
		return "autoplay"
	}
	if len(r.Frames) == 0 {
		return "no replay frames"
	}
	return ""
}

func (r *Replay) HasDT() bool {
	return r.Mods&(ModDoubleTime|ModNightcore) != 0
}

func (r *Replay) ModString() string {
	var parts []string
	if r.Mods&ModEasy != 0 {
		parts = append(parts, "EZ")
	}
	if r.Mods&ModHidden != 0 {
		parts = append(parts, "HD")
	}
	if r.Mods&ModHardRock != 0 {
		parts = append(parts, "HR")
	}
	if r.Mods&ModNightcore != 0 {
		parts = append(parts, "NC")
	} else if r.Mods&ModDoubleTime != 0 {
		parts = append(parts, "DT")
	}
	if r.Mods&ModHalfTime != 0 {
		parts = append(parts, "HT")
	}
	if r.Mods&ModFlashlight != 0 {
		parts = append(parts, "FL")
	}
	if r.Mods&ModNoFail != 0 {
		parts = append(parts, "NF")
	}
	if len(parts) == 0 {
		return "NM"
	}
	return strings.Join(parts, "")
}

func parseCompressed(data []byte) ([]Frame, error) {
	lz, err := lzma.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	decoded, err := io.ReadAll(lz)
	if err != nil {
		return nil, err
	}
	text := strings.Trim(string(decoded), ",")
	if text == "" {
		return nil, nil
	}
	events := strings.Split(text, ",")
	frames := make([]Frame, 0, len(events))
	for i, ev := range events {
		ev = strings.TrimSpace(ev)
		if ev == "" {
			continue
		}
		parts := strings.Split(ev, "|")
		if len(parts) < 4 {
			continue
		}
		delta, err := strconv.ParseFloat(parts[0], 64)
		if err != nil {
			return nil, fmt.Errorf("frame %d time: %w", i, err)
		}
		if delta == -12345 {
			continue
		}
		x, err := strconv.ParseFloat(parts[1], 64)
		if err != nil {
			return nil, fmt.Errorf("frame %d x: %w", i, err)
		}
		y, err := strconv.ParseFloat(parts[2], 64)
		if err != nil {
			return nil, fmt.Errorf("frame %d y: %w", i, err)
		}
		keys, err := strconv.Atoi(parts[3])
		if err != nil {
			return nil, fmt.Errorf("frame %d keys: %w", i, err)
		}
		frames = append(frames, Frame{Delta: delta, X: x, Y: y, Keys: keys})
	}
	return frames, nil
}

func timeFromTicks(ticks int64) time.Time {
	const ticksPerSecond = 10_000_000
	unix := ticks/ticksPerSecond - 62135596800
	nsec := (ticks % ticksPerSecond) * 100
	return time.Unix(unix, nsec).UTC()
}

func readU8(r *bytes.Reader) (byte, error) {
	return r.ReadByte()
}

func readU16(r *bytes.Reader) (uint16, error) {
	var v uint16
	if err := binary.Read(r, binary.LittleEndian, &v); err != nil {
		return 0, err
	}
	return v, nil
}

func readI32(r *bytes.Reader) (int32, error) {
	var v int32
	if err := binary.Read(r, binary.LittleEndian, &v); err != nil {
		return 0, err
	}
	return v, nil
}

func readU32(r *bytes.Reader) (uint32, error) {
	var v uint32
	if err := binary.Read(r, binary.LittleEndian, &v); err != nil {
		return 0, err
	}
	return v, nil
}

func readI64(r *bytes.Reader) (int64, error) {
	var v int64
	if err := binary.Read(r, binary.LittleEndian, &v); err != nil {
		return 0, err
	}
	return v, nil
}

func readOsuString(r *bytes.Reader) (string, error) {
	kind, err := r.ReadByte()
	if err != nil {
		return "", err
	}
	if kind == 0x00 {
		return "", nil
	}
	if kind != 0x0b {
		return "", fmt.Errorf("invalid string indicator 0x%02x", kind)
	}
	n, err := readULEB128(r)
	if err != nil {
		return "", err
	}
	if n == 0 {
		return "", nil
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", err
	}
	return string(buf), nil
}

func readULEB128(r *bytes.Reader) (uint64, error) {
	var result uint64
	var shift uint
	for {
		b, err := r.ReadByte()
		if err != nil {
			return 0, err
		}
		result |= uint64(b&0x7f) << shift
		if b&0x80 == 0 {
			return result, nil
		}
		shift += 7
		if shift > 63 {
			return 0, fmt.Errorf("uleb128 overflow")
		}
	}
}
