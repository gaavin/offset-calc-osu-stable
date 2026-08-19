//go:build linux

package osumem

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

type linuxProc struct {
	pid int
}

func (p linuxProc) Pid() int     { return p.pid }
func (p linuxProc) Close() error { return nil }

func (p linuxProc) ReadAt(b []byte, off int64) (int, error) {
	if len(b) == 0 {
		return 0, nil
	}
	local := []unix.Iovec{{Base: &b[0]}}
	local[0].SetLen(len(b))
	remote := []unix.RemoteIovec{{Base: uintptr(off), Len: len(b)}}
	n, err := unix.ProcessVMReadv(p.pid, local, remote, 0)
	if n == 0 && err != nil {
		return 0, err
	}
	if n < len(b) && err == nil {
		err = fmt.Errorf("short read at 0x%x", off)
	}
	return n, err
}

func (p linuxProc) Maps() ([]Region, error) {
	f, err := os.Open(fmt.Sprintf("/proc/%d/maps", p.pid))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []Region
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		addr, rest, ok := strings.Cut(fields[0], "-")
		if !ok {
			continue
		}
		start, err1 := strconv.ParseUint(addr, 16, 64)
		end, err2 := strconv.ParseUint(rest, 16, 64)
		if err1 != nil || err2 != nil || end <= start {
			continue
		}
		perms := fields[1]
		if !strings.Contains(perms, "r") {
			continue
		}
		name := ""
		if len(fields) >= 6 {
			name = strings.Join(fields[5:], " ")
		}
		out = append(out, Region{
			Start: int64(start),
			Size:  int64(end - start),
			Exec:  strings.Contains(perms, "x"),
			Name:  name,
		})
	}
	return out, sc.Err()
}

func findOsuProcess() (Process, error) {
	ents, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}
	for _, e := range ents {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		cmd, err := os.ReadFile("/proc/" + e.Name() + "/cmdline")
		if err != nil {
			continue
		}
		s := strings.ToLower(string(bytes.ReplaceAll(cmd, []byte{0}, []byte{' '})))
		if strings.Contains(s, "osu!lazer") || strings.Contains(s, "osu!framework") {
			continue
		}
		if !strings.Contains(s, "osu!.exe") {
			continue
		}
		return linuxProc{pid: pid}, nil
	}
	return nil, fmt.Errorf("no osu!.exe process")
}
