package hits

import (
	"math"
	"sort"
)

func UnstableRate(errors []float64) float64 {
	if len(errors) == 0 {
		return 0
	}
	mean := Mean(errors)
	var ss float64
	for _, e := range errors {
		d := e - mean
		ss += d * d
	}
	return 10 * math.Sqrt(ss/float64(len(errors)))
}

// SuggestedOffset is currentOffset - medianHitError.
// osu! reports positive error when the hit is late, so a late average
// means the global Offset should go down.
func SuggestedOffset(currentOffset, median float64) float64 {
	return currentOffset - median
}

func Median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	s := append([]float64(nil), values...)
	sort.Float64s(s)
	n := len(s)
	mid := n / 2
	if n%2 == 0 {
		return (s[mid-1] + s[mid]) / 2
	}
	return s[mid]
}

func Mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var s float64
	for _, v := range values {
		s += v
	}
	return s / float64(len(values))
}

func RoundOffset(v float64) int {
	return int(math.Round(v))
}

func Int32ToFloat(in []int32) []float64 {
	out := make([]float64, len(in))
	for i, v := range in {
		out[i] = float64(v)
	}
	return out
}
