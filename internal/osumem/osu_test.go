package osumem

import (
	"encoding/binary"
	"math"
	"testing"
	"time"
)

func putF64(p memProc, addr int64, v float64) {
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], math.Float64bits(v))
	p.mem[addr] = append([]byte(nil), b[:]...)
}

func setupOffsetConfig(p memProc, bindableAddr int64, offsetValue float64) (configPtr int64, offsetIndex int32) {
	const (
		config int64 = 0x2000
		table  int64 = 0x3000
		key    int64 = 0x4000
	)
	putI32(p, config+0x8, int32(table))
	putI32(p, config+0x1c, 1)
	putI32(p, table+0x8, int32(key))
	putI32(p, key+4, 6)
	raw := make([]byte, 12)
	for i, r := range []rune("Offset") {
		binary.LittleEndian.PutUint16(raw[i*2:], uint16(r))
	}
	p.mem[key+8] = raw
	if bindableAddr != 0 {
		putI32(p, table+0x8+0x4, int32(bindableAddr))
		putF64(p, bindableAddr+0x4, offsetValue)
	}
	return config, 0
}

func TestOffsetWithRetry_succeedsWhenBindableReadyLater(t *testing.T) {
	p := memProc{mem: map[int64][]byte{}}
	configPtr, offsetIndex := setupOffsetConfig(p, 0, -30)
	const bindable int64 = 0x5000

	rd := &Reader{
		proc:        p,
		configPtr:   configPtr,
		offsetIndex: offsetIndex,
	}

	go func() {
		time.Sleep(300 * time.Millisecond)
		putI32(p, 0x3000+0x8+0x4, int32(bindable))
		putF64(p, bindable+0x4, -30)
	}()

	cur, err := rd.OffsetWithRetry(2 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if cur != -30 {
		t.Fatalf("offset %d", cur)
	}
}

func TestOffsetWithRetry_timesOut(t *testing.T) {
	p := memProc{mem: map[int64][]byte{}}
	configPtr, offsetIndex := setupOffsetConfig(p, 0, -30)

	rd := &Reader{
		proc:        p,
		configPtr:   configPtr,
		offsetIndex: offsetIndex,
	}

	start := time.Now()
	_, err := rd.OffsetWithRetry(500 * time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if time.Since(start) < 400*time.Millisecond {
		t.Fatalf("returned too quickly: %v", time.Since(start))
	}
}

func TestReadConfigIntBindableMissing(t *testing.T) {
	p := memProc{mem: map[int64][]byte{}}
	configPtr, offsetIndex := setupOffsetConfig(p, 0, -30)
	_, err := readConfigInt(p, configPtr, offsetIndex)
	if err == nil || err.Error() != "config bindable" {
		t.Fatalf("got %v", err)
	}
}

func setupOffsetConfigWithSig(p memProc, bindableAddr int64, offsetValue float64) (sigAddr, configPtr int64, offsetIndex int32) {
	configPtr, offsetIndex = setupOffsetConfig(p, bindableAddr, offsetValue)
	const (
		sig int64 = 0x1000
		mid int64 = 0x1100
	)
	putI32(p, sig, int32(mid))
	putI32(p, mid, int32(configPtr))
	return sig, configPtr, offsetIndex
}

func TestOffsetFollowsLiveConfigPointer(t *testing.T) {
	p := memProc{mem: map[int64][]byte{}}
	sigAddr, configPtr, offsetIndex := setupOffsetConfigWithSig(p, 0x5000, -30)

	rd := &Reader{
		proc:          p,
		configSigAddr: sigAddr,
		configPtr:     0x1, // not a valid ConfigManager — must not be used
		offsetIndex:   offsetIndex,
	}

	cur, err := rd.Offset()
	if err != nil {
		t.Fatal(err)
	}
	if cur != -30 {
		t.Fatalf("offset %d", cur)
	}
	if rd.configPtr != configPtr {
		t.Fatalf("configPtr 0x%x want 0x%x", rd.configPtr, configPtr)
	}
	last, ok := rd.LastOffset()
	if !ok || last != -30 {
		t.Fatalf("LastOffset %d ok=%v", last, ok)
	}
}

func TestOffsetReloadsIndexWhenSlotMoves(t *testing.T) {
	p := memProc{mem: map[int64][]byte{}}
	sigAddr, configPtr, _ := setupOffsetConfigWithSig(p, 0x5000, -30)

	rd := &Reader{
		proc:          p,
		configSigAddr: sigAddr,
		configPtr:     configPtr,
		offsetIndex:   99, // wrong cached index
	}

	cur, err := rd.Offset()
	if err != nil {
		t.Fatal(err)
	}
	if cur != -30 {
		t.Fatalf("offset %d", cur)
	}
	if rd.offsetIndex != 0 {
		t.Fatalf("index %d", rd.offsetIndex)
	}
}

func TestFinishOffsetFallsBackToLast(t *testing.T) {
	rd := &Reader{
		proc:          memProc{mem: map[int64][]byte{}},
		lastOffset:    -30,
		hasLastOffset: true,
	}
	cur, err := rd.FinishOffset()
	if err != nil {
		t.Fatal(err)
	}
	if cur != -30 {
		t.Fatalf("offset %d", cur)
	}
}
