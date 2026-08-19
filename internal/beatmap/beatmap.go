package beatmap

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

const (
	objCircle  = 1
	objSlider  = 2
	objSpinner = 8
)

type Kind int

const (
	KindCircle Kind = iota
	KindSliderHead
)

type Object struct {
	X, Y, Time float64
	Kind       Kind
}

type Beatmap struct {
	Path     string
	Title    string
	Artist   string
	Creator  string
	Version  string
	Mode     int
	CS       float64
	OD       float64
	Objects  []Object
}

func (b Beatmap) DisplayName() string {
	diff := b.Version
	if diff == "" {
		diff = "unknown"
	}
	if b.Artist == "" && b.Title == "" {
		return diff
	}
	return fmt.Sprintf("%s - %s [%s]", b.Artist, b.Title, diff)
}

func ParseFile(path string) (*Beatmap, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	bm, err := Parse(f)
	if err != nil {
		return nil, err
	}
	bm.Path = path
	return bm, nil
}

func Parse(r io.Reader) (*Beatmap, error) {
	bm := &Beatmap{CS: 5, OD: 5}
	section := ""
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		if strings.HasPrefix(line, "\ufeff") {
			line = strings.TrimPrefix(line, "\ufeff")
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.ToLower(line)
			continue
		}
		switch section {
		case "[general]":
			if k, v, ok := splitKV(line); ok && strings.EqualFold(k, "Mode") {
				bm.Mode, _ = strconv.Atoi(strings.TrimSpace(v))
			}
		case "[metadata]":
			k, v, ok := splitKV(line)
			if !ok {
				continue
			}
			switch strings.ToLower(k) {
			case "title":
				if bm.Title == "" {
					bm.Title = v
				}
			case "titleunicode":
				if v != "" {
					bm.Title = v
				}
			case "artist":
				if bm.Artist == "" {
					bm.Artist = v
				}
			case "artistunicode":
				if v != "" {
					bm.Artist = v
				}
			case "creator":
				bm.Creator = v
			case "version":
				bm.Version = v
			}
		case "[difficulty]":
			k, v, ok := splitKV(line)
			if !ok {
				continue
			}
			switch strings.ToLower(k) {
			case "circlesize":
				bm.CS, _ = strconv.ParseFloat(v, 64)
			case "overalldifficulty":
				bm.OD, _ = strconv.ParseFloat(v, 64)
			}
		case "[hitobjects]":
			obj, ok := parseHitObject(line)
			if ok {
				bm.Objects = append(bm.Objects, obj)
			}
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return bm, nil
}

func splitKV(line string) (string, string, bool) {
	i := strings.IndexByte(line, ':')
	if i < 0 {
		return "", "", false
	}
	return strings.TrimSpace(line[:i]), strings.TrimSpace(line[i+1:]), true
}

func parseHitObject(line string) (Object, bool) {
	parts := strings.Split(line, ",")
	if len(parts) < 5 {
		return Object{}, false
	}
	x, err1 := strconv.ParseFloat(parts[0], 64)
	y, err2 := strconv.ParseFloat(parts[1], 64)
	t, err3 := strconv.ParseFloat(parts[2], 64)
	typ, err4 := strconv.Atoi(strings.TrimSpace(parts[3]))
	if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
		return Object{}, false
	}
	if typ&objSpinner != 0 {
		return Object{}, false
	}
	obj := Object{X: x, Y: y, Time: t}
	if typ&objSlider != 0 {
		obj.Kind = KindSliderHead
		return obj, true
	}
	if typ&objCircle != 0 {
		obj.Kind = KindCircle
		return obj, true
	}
	return Object{}, false
}
