package tui

import (
	"fmt"
	"math"
	"strings"
)

const blocks = "▁▂▃▄▅▆▇█"

var blockRunes = []rune(blocks)

func histogram(errors []float64, median float64, width, height int) []string {
	if width < 8 {
		width = 8
	}
	if height < 2 {
		height = 2
	}
	if len(errors) == 0 {
		return []string{strings.Repeat(" ", width)}
	}

	half := float64(width) / 2
	span := half
	for _, e := range errors {
		span = math.Max(span, math.Abs(e)+2)
	}
	span = math.Ceil(span/5) * 5
	if span < 10 {
		span = 10
	}

	bins := make([]int, width)
	for _, e := range errors {
		col := int(math.Round((e / span) * half))
		col += width / 2
		if col < 0 {
			col = 0
		}
		if col >= width {
			col = width - 1
		}
		bins[col]++
	}

	maxCount := 1
	for _, c := range bins {
		if c > maxCount {
			maxCount = c
		}
	}

	rows := make([]string, height)
	for row := height - 1; row >= 0; row-- {
		threshold := float64(maxCount) * float64(row+1) / float64(height)
		var b strings.Builder
		for col, c := range bins {
			ch := ' '
			if float64(c) >= threshold && c > 0 {
				level := int(math.Ceil(float64(c) / float64(maxCount) * float64(len(blockRunes)-1)))
				if level < 0 {
					level = 0
				}
				if level >= len(blockRunes) {
					level = len(blockRunes) - 1
				}
				ch = blockRunes[level]
			}
			if col == width/2 {
				if ch == ' ' {
					ch = '│'
				}
			}
			b.WriteRune(ch)
		}
		rows[row] = b.String()
	}

	mark := medianColumn(median, span, width)
	if mark >= 0 && mark < width {
		for i := range rows {
			rs := []rune(rows[i])
			if rs[mark] == ' ' || rs[mark] == '│' {
				rs[mark] = '┊'
			}
			rows[i] = string(rs)
		}
	}
	return rows
}

func medianColumn(median, span float64, width int) int {
	half := float64(width) / 2
	col := int(math.Round((median / span) * half))
	col += width / 2
	if col < 0 || col >= width {
		return -1
	}
	return col
}

func histogramLegend(span float64) string {
	return fmt.Sprintf("%+.0f ms early %s %+.0f ms late", -span, strings.Repeat("─", 7), span)
}

func offsetScale(current, recommended int, width int) (line string, curMark, recMark int) {
	if width < 12 {
		width = 12
	}
	lo := current
	hi := current
	if recommended < lo {
		lo = recommended
	}
	if recommended > hi {
		hi = recommended
	}
	padding := 5
	lo -= padding
	hi += padding
	if lo == hi {
		hi = lo + 1
	}

	marks := make([]rune, width)
	for i := range marks {
		marks[i] = '─'
	}

	pos := func(v int) int {
		if hi == lo {
			return width / 2
		}
		p := int(math.Round(float64(v-lo) / float64(hi-lo) * float64(width-1)))
		if p < 0 {
			return 0
		}
		if p >= width {
			return width - 1
		}
		return p
	}

	curMark = pos(current)
	recMark = pos(recommended)
	marks[curMark] = '●'
	if recMark != curMark {
		marks[recMark] = '◆'
	} else {
		marks[recMark] = '◎'
	}

	return string(marks), curMark, recMark
}
