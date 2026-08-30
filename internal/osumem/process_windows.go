//go:build windows

package osumem

import (
	"fmt"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

type winProc struct {
	pid    uint32
	handle windows.Handle
}

func (p winProc) Pid() int { return int(p.pid) }

func (p winProc) Close() error {
	return windows.CloseHandle(p.handle)
}

func (p winProc) Alive() bool {
	var code uint32
	if err := windows.GetExitCodeProcess(p.handle, &code); err != nil {
		return false
	}
	return code == windows.STILL_ACTIVE
}

func (p winProc) ReadAt(b []byte, off int64) (int, error) {
	if len(b) == 0 {
		return 0, nil
	}
	var n uintptr
	err := windows.ReadProcessMemory(p.handle, uintptr(off), &b[0], uintptr(len(b)), &n)
	return int(n), err
}

type memBasicInfo struct {
	BaseAddress       uintptr
	AllocationBase    uintptr
	AllocationProtect uint32
	RegionSize        uintptr
	State             uint32
	Protect           uint32
	Type              uint32
}

const (
	memCommit    = 0x1000
	pageNoAccess = 0x01
	pageGuard    = 0x100
	pageExec     = 0x10
	pageExecRead = 0x20
	pageExecRW   = 0x40
	pageExecWC   = 0x80
)

func (p winProc) Maps() ([]Region, error) {
	var out []Region
	var addr uintptr
	var info memBasicInfo
	kernel32 := windows.NewLazySystemDLL("kernel32.dll")
	vq := kernel32.NewProc("VirtualQueryEx")
	for {
		r1, _, e := vq.Call(
			uintptr(p.handle),
			addr,
			uintptr(unsafe.Pointer(&info)),
			unsafe.Sizeof(info),
		)
		if r1 == 0 {
			if addr == 0 {
				return nil, e
			}
			break
		}
		if info.State == memCommit && info.Protect&pageNoAccess == 0 && info.Protect&pageGuard == 0 {
			exec := info.Protect&(pageExec|pageExecRead|pageExecRW|pageExecWC) != 0
			out = append(out, Region{
				Start: int64(info.BaseAddress),
				Size:  int64(info.RegionSize),
				Exec:  exec,
			})
		}
		next := info.BaseAddress + info.RegionSize
		if next <= addr {
			break
		}
		addr = next
	}
	return out, nil
}

func findOsuProcess() (Process, error) {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(snap)
	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	if err := windows.Process32First(snap, &entry); err != nil {
		return nil, err
	}
	for {
		name := windows.UTF16ToString(entry.ExeFile[:])
		if strings.EqualFold(name, "osu!.exe") {
			h, err := windows.OpenProcess(
				windows.PROCESS_VM_READ|windows.PROCESS_QUERY_INFORMATION|windows.SYNCHRONIZE,
				false,
				entry.ProcessID,
			)
			if err != nil {
				return nil, err
			}
			return winProc{pid: entry.ProcessID, handle: h}, nil
		}
		if err := windows.Process32Next(snap, &entry); err != nil {
			break
		}
	}
	return nil, fmt.Errorf("no osu!.exe process")
}
