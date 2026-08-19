//go:build !linux && !darwin && !windows

package stable

func runningOsuDirs() []string { return nil }
