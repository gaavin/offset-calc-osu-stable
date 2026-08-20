package osumem

import (
	"fmt"
	"runtime"
	"time"
)

const (
	StatusPlaying      = 2
	StatusResultScreen = 7

	offsetReadRetryInterval = 200 * time.Millisecond
	offsetReadTimeout       = 15 * time.Second
)

const (
	sigStatus   = "48 83 F8 04 73 1E"
	sigRulesets = "7D 15 A1 ?? ?? ?? ?? 85 C0"
	sigReplay   = "55 8B EC 80 3D ?? ?? ?? ?? 00 75 26 80 3D"
	sigBase     = "F8 01 74 04 83 65"
)

// Reader attaches to a running osu!stable process and reads the current
// play's hit-error list (same List<int> the error bar uses).
type Reader struct {
	proc         Process
	statusPtr    int64
	rulesetsAddr int64
	replayAddr   int64
	baseAddr     int64
	configPtr    int64
	offsetIndex  int32
	arrayStart   int64
	configTried  time.Time
	baseScanAt   time.Time
}

func Attach(p Process) (*Reader, error) {
	statusPat, err := Scan(p, sigStatus)
	if err != nil {
		return nil, fmt.Errorf("status signature: %w", err)
	}
	rulesets, err := Scan(p, sigRulesets)
	if err != nil {
		return nil, fmt.Errorf("rulesets signature: %w", err)
	}
	replay, _ := Scan(p, sigReplay)
	base, baseErr := Scan(p, sigBase)
	configPtr, offsetIndex, cfgErr := resolveConfig(p)
	arrayStart := int64(0x8)
	if runtime.GOOS != "windows" {
		arrayStart = 0xC
	}
	rd := &Reader{
		proc:         p,
		statusPtr:    statusPat - 0x4,
		rulesetsAddr: rulesets,
		replayAddr:   replay,
		baseAddr:     base,
		configPtr:    configPtr,
		offsetIndex:  offsetIndex,
		arrayStart:   arrayStart,
	}
	if cfgErr != nil {
		rd.configTried = time.Now()
	}
	if baseErr != nil {
		// Leave baseScanAt zero so Beatmap() retries soon; attach must not
		// fail just because optional beatmap metadata is unavailable yet.
		rd.baseAddr = 0
	}
	return rd, nil
}

func (r *Reader) Close() error { return r.proc.Close() }

func (r *Reader) Pid() int { return r.proc.Pid() }

func (r *Reader) Alive() bool { return r.proc.Alive() }

func (r *Reader) Offset() (int32, error) {
	if r.configPtr == 0 || r.offsetIndex < 0 {
		if !r.configTried.IsZero() && time.Since(r.configTried) < 2*time.Second {
			return 0, fmt.Errorf("configuration not ready")
		}
		r.configTried = time.Now()
		ptr, idx, err := resolveConfig(r.proc)
		if err != nil {
			return 0, err
		}
		r.configPtr = ptr
		r.offsetIndex = idx
	}
	return readConfigInt(r.proc, r.configPtr, r.offsetIndex)
}

// OffsetWithRetry polls Offset until it succeeds or timeout elapses.
// Use at play end when the bindable may not be readable yet on first attach.
func (r *Reader) OffsetWithRetry(timeout time.Duration) (int32, error) {
	if timeout <= 0 {
		timeout = offsetReadTimeout
	}
	deadline := time.Now().Add(timeout)
	var lastErr error
	for {
		cur, err := r.Offset()
		if err == nil {
			return cur, nil
		}
		lastErr = err
		if time.Now().After(deadline) {
			return 0, fmt.Errorf("read offset: %w", lastErr)
		}
		time.Sleep(offsetReadRetryInterval)
	}
}

func (r *Reader) Status() (int32, error) {
	return DerefI32(r.proc, r.statusPtr)
}

func (r *Reader) WatchingReplay() bool {
	if r.replayAddr == 0 {
		return false
	}
	p, err := ReadPtr32(r.proc, r.replayAddr+0x46)
	if err != nil || p == 0 {
		return false
	}
	b, err := ReadI8(r.proc, p)
	return err == nil && b == 1
}

func (r *Reader) ruleset() (int64, error) {
	p, err := ReadPtr32(r.proc, r.rulesetsAddr-0xb)
	if err != nil || p == 0 {
		return 0, fmt.Errorf("rulesets ptr")
	}
	return ReadPtr32(r.proc, p+0x4)
}

func (r *Reader) HitErrors() ([]int32, error) {
	ruleset, err := r.ruleset()
	if err != nil || ruleset == 0 {
		return nil, err
	}
	for _, gpOff := range []int64{0x64, 0x68} {
		errors, err := r.hitErrorsAt(ruleset, gpOff)
		if err == nil && len(errors) > 0 {
			return errors, nil
		}
	}
	return r.hitErrorsAt(ruleset, 0x64)
}

func (r *Reader) Mode() (int32, error) {
	score, err := r.scoreBase()
	if err != nil {
		return -1, err
	}
	return ReadI32(r.proc, score+0x64)
}

func (r *Reader) scoreBase() (int64, error) {
	ruleset, err := r.ruleset()
	if err != nil || ruleset == 0 {
		return 0, fmt.Errorf("ruleset")
	}
	gp, err := ReadPtr32(r.proc, ruleset+0x64)
	if err != nil || gp == 0 {
		gp, err = ReadPtr32(r.proc, ruleset+0x68)
	}
	if err != nil || gp == 0 {
		return 0, fmt.Errorf("gameplay base")
	}
	return ReadPtr32(r.proc, gp+0x38)
}

func (r *Reader) hitErrorsAt(ruleset, gameplayOff int64) ([]int32, error) {
	gp, err := ReadPtr32(r.proc, ruleset+gameplayOff)
	if err != nil || gp == 0 {
		return nil, fmt.Errorf("gameplay base")
	}
	score, err := ReadPtr32(r.proc, gp+0x38)
	if err != nil || score == 0 {
		return nil, fmt.Errorf("score base")
	}
	list, err := ReadPtr32(r.proc, score+0x38)
	if err != nil || list == 0 {
		return nil, fmt.Errorf("hit error list")
	}
	items, err := ReadPtr32(r.proc, list+0x4)
	if err != nil || items == 0 {
		return nil, fmt.Errorf("hit error items")
	}
	arrLen, err := ReadI32(r.proc, items+0x4)
	if err != nil {
		return nil, err
	}
	size, err := ReadI32(r.proc, list+0xc)
	if err != nil {
		return nil, err
	}
	if size < 0 || size > arrLen {
		size, err = ReadI32(r.proc, list+0x8)
		if err != nil {
			return nil, err
		}
	}
	if size < 0 {
		return nil, fmt.Errorf("negative hit error count")
	}
	if size > 65536 {
		return nil, fmt.Errorf("hit error list too long (%d)", size)
	}
	if arrLen >= 0 && size > arrLen {
		size = arrLen
	}
	starts := []int64{r.arrayStart}
	if r.arrayStart == 0x8 {
		starts = append(starts, 0xC)
	} else {
		starts = append(starts, 0x8)
	}
	var lastErr error
	for _, start := range starts {
		out := make([]int32, 0, size)
		ok := true
		for i := int32(0); i < size; i++ {
			v, err := ReadI32(r.proc, items+start+int64(i)*4)
			if err != nil {
				lastErr = err
				ok = false
				break
			}
			if v < -10000 || v > 10000 {
				ok = false
				lastErr = fmt.Errorf("implausible hit error %d", v)
				break
			}
			out = append(out, v)
		}
		if ok {
			r.arrayStart = start
			return out, nil
		}
		if len(out) > 0 && start == r.arrayStart {
			r.arrayStart = start
			return out, nil
		}
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("empty hit error list")
}
