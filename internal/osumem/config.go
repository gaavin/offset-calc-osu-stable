package osumem

import (
	"fmt"
	"math"
	"unicode/utf16"
)

const (
	sigConfiguration = "8D 45 EC 50 8B 0D ?? ?? ?? ?? 8B D7 39 09 E8 ?? ?? ?? ?? 85 C0 74 ?? 8B 4D EC"
	configPatternOff = 0x6
)

func readPointer(r Process, addr int64) (int64, error) {
	p, err := ReadPtr32(r, addr)
	if err != nil {
		return 0, err
	}
	if p == 0 {
		return 0, fmt.Errorf("null pointer at 0x%x", addr)
	}
	return ReadPtr32(r, p)
}

func readSharpString(r Process, addr int64) (string, error) {
	if addr == 0 {
		return "", nil
	}
	// osu!stable is 32-bit .NET: MethodTable at 0, length at +4, UTF-16 at +8.
	const lenOff int64 = 4
	length, err := ReadI32(r, addr+lenOff)
	if err != nil {
		return "", err
	}
	if length <= 0 {
		return "", nil
	}
	if length > 4096 {
		return "", fmt.Errorf("string length %d", length)
	}
	raw := make([]byte, length*2)
	if _, err := r.ReadAt(raw, addr+lenOff+4); err != nil {
		return "", err
	}
	u16 := make([]uint16, length)
	for i := range u16 {
		u16[i] = uint16(raw[i*2]) | uint16(raw[i*2+1])<<8
	}
	return string(utf16.Decode(u16)), nil
}

func findConfigOffsetIndex(r Process, configPtr int64) (int32, error) {
	tableBase, err := ReadPtr32(r, configPtr+0x8)
	if err != nil || tableBase == 0 {
		return -1, fmt.Errorf("config table base")
	}
	size, err := ReadI32(r, configPtr+0x1c)
	if err != nil {
		return -1, err
	}
	if size < 0 || size > 512 {
		return -1, fmt.Errorf("config dictionary size %d", size)
	}
	for i := int32(0); i < size; i++ {
		slot := tableBase + 0x8 + int64(i)*0x10
		keyAddr, err := ReadPtr32(r, slot)
		if err != nil || keyAddr == 0 {
			continue
		}
		key, err := readSharpString(r, keyAddr)
		if err != nil || key != "Offset" {
			continue
		}
		return i, nil
	}
	return -1, fmt.Errorf("Offset key not found in config")
}

func readConfigInt(r Process, configPtr int64, index int32) (int32, error) {
	tableBase, err := ReadPtr32(r, configPtr+0x8)
	if err != nil || tableBase == 0 {
		return 0, fmt.Errorf("config table base")
	}
	slot := tableBase + 0x8 + int64(index)*0x10
	bindable, err := ReadPtr32(r, slot+0x4)
	if err != nil || bindable == 0 {
		return 0, fmt.Errorf("config bindable")
	}
	v, err := ReadF64(r, bindable+0x4)
	if err != nil {
		return 0, err
	}
	return int32(math.Round(v)), nil
}

func configAt(r Process, sigAddr int64) (configPtr int64, offsetIndex int32, err error) {
	configPtr, err = readPointer(r, sigAddr)
	if err != nil || configPtr == 0 {
		return 0, -1, fmt.Errorf("configuration pointer")
	}
	offsetIndex, err = findConfigOffsetIndex(r, configPtr)
	if err != nil {
		return 0, -1, err
	}
	return configPtr, offsetIndex, nil
}

func resolveConfig(r Process) (sigAddr, configPtr int64, offsetIndex int32, err error) {
	addr, err := Scan(r, sigConfiguration)
	if err != nil {
		return 0, 0, -1, fmt.Errorf("configuration signature: %w", err)
	}
	addr += configPatternOff
	configPtr, offsetIndex, err = configAt(r, addr)
	return addr, configPtr, offsetIndex, err
}
