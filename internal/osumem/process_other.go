//go:build !linux && !windows

package osumem

import (
	"fmt"
	"runtime"
)

func findOsuProcess() (Process, error) {
	return nil, fmt.Errorf("reading osu!stable memory is not supported on %s", runtime.GOOS)
}
