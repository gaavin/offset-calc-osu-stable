package osumem

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strings"
)

// ErrNotRunning is returned when no osu!stable process is found.
var ErrNotRunning = errors.New("osu!.exe not running")

// ErrGone is returned when a previously attached process can no longer be read.
var ErrGone = errors.New("osu! process exited")

type Region struct {
	Start int64
	Size  int64
	Exec  bool
	Name  string
}

type Process interface {
	io.ReaderAt
	Pid() int
	Alive() bool
	Close() error
	Maps() ([]Region, error)
}

func isOsuStableCmd(s string) bool {
	s = strings.ToLower(s)
	if strings.Contains(s, "osu!lazer") || strings.Contains(s, "osu!framework") {
		return false
	}
	return strings.Contains(s, "osu!.exe")
}

func ReadI32(r io.ReaderAt, addr int64) (int32, error) {
	var b [4]byte
	n, err := r.ReadAt(b[:], addr)
	if n < 4 {
		if err == nil {
			err = io.ErrUnexpectedEOF
		}
		return 0, err
	}
	return int32(binary.LittleEndian.Uint32(b[:])), nil
}

func ReadU32(r io.ReaderAt, addr int64) (uint32, error) {
	v, err := ReadI32(r, addr)
	return uint32(v), err
}

func ReadPtr32(r io.ReaderAt, addr int64) (int64, error) {
	v, err := ReadU32(r, addr)
	return int64(v), err
}

func ReadI8(r io.ReaderAt, addr int64) (byte, error) {
	var b [1]byte
	n, err := r.ReadAt(b[:], addr)
	if n < 1 {
		if err == nil {
			err = io.ErrUnexpectedEOF
		}
		return 0, err
	}
	return b[0], nil
}

func DerefI32(r io.ReaderAt, addr int64) (int32, error) {
	p, err := ReadPtr32(r, addr)
	if err != nil {
		return 0, err
	}
	return ReadI32(r, p)
}

func OpenOsu() (Process, error) {
	p, err := findOsuProcess()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNotRunning, err)
	}
	return p, nil
}
