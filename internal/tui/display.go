package tui

import (
	"fmt"
	"io"
	"math"
	"os"
	"strings"

	"github.com/pterm/pterm"
	"golang.org/x/term"
)

type Display struct {
	fancy  bool
	live   *liveBlock
	stdout io.Writer
	stderr io.Writer
}

type PlayStats struct {
	HitCount  int
	Errors    []float64
	CurOffset int
	Median    float64
	Recommend int
	MinHits   int
	HasOffset bool
	MapTitle  string
}

type Result struct {
	Hits          int
	Median        float64
	Mean          float64
	UnstableRate  float64
	CurrentOffset int
	Recommend     int
	Errors        []float64
	MapTitle      string
}

func Enabled(jsonOut bool) bool {
	if jsonOut {
		return false
	}
	return term.IsTerminal(int(os.Stderr.Fd()))
}

func New(fancy bool, stdout, stderr io.Writer) *Display {
	d := &Display{fancy: fancy, stdout: stdout, stderr: stderr}
	if fancy {
		d.live = &liveBlock{w: stderr}
		pterm.SetDefaultOutput(stderr)
	}
	return d
}

func (d *Display) Banner(version string, minHits int) {
	if !d.fancy {
		fmt.Fprintf(d.stderr, "osu-offset %s — watching for osu!.exe\n", version)
		fmt.Fprintf(d.stderr, "Need ≥ %d timed hits on a standard play. Ctrl+C to stop.\n", minHits)
		return
	}
	pterm.DefaultHeader.WithBackgroundStyle(pterm.NewStyle(pterm.BgLightBlue, pterm.FgBlack)).
		Println("osu-offset " + version)
	pterm.Info.Printfln("Need ≥ %d timed hits on a standard play. Ctrl+C to stop.", minHits)
}

func (d *Display) Waiting(msg string) {
	if !d.fancy {
		fmt.Fprintln(d.stderr, msg)
		return
	}
	pterm.Warning.Println(msg)
}

func (d *Display) Attached(pid int) {
	if !d.fancy {
		fmt.Fprintf(d.stderr, "attached to osu!.exe pid %d\n", pid)
		return
	}
	pterm.Success.Printfln("Attached to osu!.exe (pid %d)", pid)
}

func (d *Display) PlayStarted(mapTitle string) {
	if !d.fancy {
		if mapTitle != "" {
			fmt.Fprintf(d.stderr, "play started: %s\n", mapTitle)
		} else {
			fmt.Fprintln(d.stderr, "play started")
		}
		return
	}
	if mapTitle != "" {
		pterm.Info.Printfln("Playing: %s", mapTitle)
	}
}

func (d *Display) SkipMode(mode int32) {
	if !d.fancy {
		fmt.Fprintf(d.stderr, "\r  skipping non-standard mode %d          ", mode)
		return
	}
	d.live.Update(pterm.Warning.Sprintfln("Skipping non-standard mode %d", mode))
}

func (d *Display) UpdatePlay(s PlayStats) {
	if !d.fancy {
		d.printPlainPlay(s)
		return
	}
	d.live.Update(renderLive(s))
}

func (d *Display) PlayEndedShort(hitCount, minHits int, mapTitle string) {
	if !d.fancy {
		fmt.Fprintln(d.stderr)
		if mapTitle != "" {
			fmt.Fprintf(d.stderr, "play ended (%s) with %d hits (need %d); waiting for another map\n", mapTitle, hitCount, minHits)
		} else {
			fmt.Fprintf(d.stderr, "play ended with %d hits (need %d); waiting for another map\n", hitCount, minHits)
		}
		return
	}
	d.live.Clear()
	if mapTitle != "" {
		pterm.Warning.Printfln("Play ended (%s) with %d hits (need %d); waiting for another map", mapTitle, hitCount, minHits)
		return
	}
	pterm.Warning.Printfln("Play ended with %d hits (need %d); waiting for another map", hitCount, minHits)
}

func (d *Display) PlayEndedClear() {
	if !d.fancy {
		fmt.Fprintln(d.stderr)
		return
	}
	d.live.Clear()
}

func (d *Display) WatchingAnother() {
	if !d.fancy {
		fmt.Fprintln(d.stderr, "watching for another play")
		return
	}
	pterm.Info.Println("Watching for another play…")
}

func (d *Display) ProcessExited() {
	if !d.fancy {
		fmt.Fprintln(d.stderr, "osu!.exe exited; watching for it to come back …")
		return
	}
	pterm.Warning.Println("osu!.exe exited; watching for it to come back…")
}

func (d *Display) PrintResult(r Result) {
	if !d.fancy {
		d.printPlainResult(r)
		return
	}
	d.printFancyResult(r)
}

func (d *Display) printPlainPlay(s PlayStats) {
	if s.HitCount == 0 {
		return
	}
	if !s.HasOffset {
		fmt.Fprintf(d.stderr, "\r  %d hits  median %+.1f ms  (offset not readable yet)     ", s.HitCount, s.Median)
		return
	}
	fmt.Fprintf(d.stderr, "\r  %d hits  offset %d ms  median %+.1f ms  → Offset %d     ", s.HitCount, s.CurOffset, s.Median, s.Recommend)
}

func (d *Display) printPlainResult(r Result) {
	if r.MapTitle != "" {
		fmt.Printf("Map:                %s\n", r.MapTitle)
	}
	fmt.Printf("Hits:               %d\n", r.Hits)
	fmt.Printf("Median hit error:   %+.1f ms\n", r.Median)
	fmt.Printf("Mean hit error:     %+.1f ms\n", r.Mean)
	fmt.Printf("Unstable rate:      %.1f\n", r.UnstableRate)
	fmt.Printf("Current Offset:     %d ms\n", r.CurrentOffset)
	fmt.Printf("Recommended Offset: %d ms\n", r.Recommend)
	delta := r.Recommend - r.CurrentOffset
	switch {
	case delta == 0:
		fmt.Println("That matches your current Offset — leave it.")
	case r.Median < 0:
		fmt.Printf("Hits are ~%.1f ms early. Raise Offset by %d ms.\n", -round1(r.Median), delta)
	default:
		fmt.Printf("Hits are ~%.1f ms late. Lower Offset by %d ms.\n", round1(r.Median), -delta)
	}
	fmt.Println("Set it in Options → Audio → Offset.")
}

func (d *Display) printFancyResult(r Result) {
	chart := pterm.NewStyle(pterm.Bold).Sprint("Offset calibration\n") + renderFinishChart(r)
	summary := pterm.NewStyle(pterm.Bold).Sprint("Recommendation\n") + renderSummary(r)
	panels, _ := pterm.DefaultPanel.WithPanels(pterm.Panels{
		{{Data: chart}, {Data: summary}},
	}).Srender()

	big, err := renderRecommendedOffsetText(r.Recommend)
	if err != nil {
		fmt.Fprintln(d.stdout, fmt.Sprintf("Recommended Offset: %d ms", r.Recommend))
	} else {
		fmt.Fprintln(d.stdout)
		if r.MapTitle != "" {
			fmt.Fprintln(d.stdout, pterm.NewStyle(pterm.Bold, pterm.FgLightWhite).Sprint(r.MapTitle))
		}
		fmt.Fprintln(d.stdout, big)
	}
	fmt.Fprintln(d.stdout, panels)
	fmt.Fprintln(d.stdout, pterm.Info.Sprint("Set it in Options → Audio → Offset."))
}

func renderRecommendedOffsetText(recommend int) (string, error) {
	big, err := pterm.DefaultBigText.WithLetters(
		pterm.NewLettersFromStringWithStyle(fmt.Sprintf("%d ms", recommend), pterm.NewStyle(pterm.FgLightGreen, pterm.Bold)),
	).Srender()
	if err != nil {
		return "", err
	}
	big = trimGraphicBlock(big)
	return pterm.DefaultBox.WithTitle("Recommended Offset").
		WithTitleTopCenter().
		WithBoxStyle(pterm.NewStyle(pterm.FgLightGreen, pterm.Bold)).
		WithTopPadding(1).
		WithBottomPadding(1).
		Sprint(big), nil
}

func trimGraphicBlock(s string) string {
	lines := strings.Split(strings.TrimSuffix(s, "\n"), "\n")
	for len(lines) > 0 && isBlankGraphicLine(lines[0]) {
		lines = lines[1:]
	}
	for len(lines) > 0 && isBlankGraphicLine(lines[len(lines)-1]) {
		lines = lines[:len(lines)-1]
	}
	return strings.Join(lines, "\n")
}

func isBlankGraphicLine(s string) bool {
	return strings.TrimSpace(pterm.RemoveColorFromString(s)) == ""
}

func renderLive(s PlayStats) string {
	title := pterm.NewStyle(pterm.FgLightCyan, pterm.Bold).Sprint("Live play")
	progress := hitProgress(s.HitCount, s.MinHits, 30)

	var headline string
	if !s.HasOffset {
		headline = fmt.Sprintf("%s  %d hits  median %+.1f ms  (offset not readable yet)", title, s.HitCount, s.Median)
	} else {
		headline = fmt.Sprintf("%s  %d hits  offset %d ms  median %+.1f ms  → %d ms",
			title, s.HitCount, s.CurOffset, s.Median, s.Recommend)
	}

	span := histogramSpan(s.Errors)
	histRows := histogram(s.Errors, s.Median, 44, 5)
	var hist strings.Builder
	hist.WriteString(pterm.NewStyle(pterm.FgLightWhite, pterm.Bold).Sprint("Hit timing"))
	hist.WriteString("\n")
	for _, row := range histRows {
		hist.WriteString("  ")
		hist.WriteString(pterm.NewStyle(pterm.FgLightBlue).Sprint(row))
		hist.WriteString("\n")
	}
	hist.WriteString("  ")
	hist.WriteString(pterm.NewStyle(pterm.FgGray).Sprint(histogramLegend(span)))
	hist.WriteString("\n  ")
	hist.WriteString(pterm.NewStyle(pterm.FgYellow).Sprint(fmt.Sprintf("┊ median %+.1f ms", s.Median)))

	cal := renderCalibrationPreview(s.CurOffset, s.Median, s.Recommend, s.HasOffset, 44)

	lines := []string{headline}
	if s.MapTitle != "" {
		lines = append(lines, pterm.NewStyle(pterm.FgLightWhite).Sprint(s.MapTitle))
	}
	lines = append(lines, progress, "", hist.String(), "", cal)
	return strings.Join(lines, "\n")
}

func renderFinishChart(r Result) string {
	span := histogramSpan(r.Errors)
	histRows := histogram(r.Errors, r.Median, 48, 6)

	var b strings.Builder
	b.WriteString(pterm.NewStyle(pterm.Bold).Sprintln("Hit error distribution"))
	for _, row := range histRows {
		b.WriteString("  ")
		b.WriteString(pterm.NewStyle(pterm.FgLightBlue).Sprint(row))
		b.WriteString("\n")
	}
	b.WriteString("  ")
	b.WriteString(pterm.NewStyle(pterm.FgGray).Sprint(histogramLegend(span)))
	b.WriteString("\n  ")
	b.WriteString(pterm.NewStyle(pterm.FgYellow).Sprint(fmt.Sprintf("┊ median %+.1f ms", r.Median)))
	b.WriteString("\n\n")
	b.WriteString(renderCalibrationPreview(r.CurrentOffset, r.Median, r.Recommend, true, 48))
	b.WriteString("\n\n")
	b.WriteString(pterm.NewStyle(pterm.FgGray).Sprint("recommended = current − median"))
	return b.String()
}

func renderSummary(r Result) string {
	delta := r.Recommend - r.CurrentOffset
	var action string
	switch {
	case delta == 0:
		action = "Matches your current Offset — leave it."
	case r.Median < 0:
		action = fmt.Sprintf("Hits are ~%.1f ms early. Raise Offset by %d ms.", -round1(r.Median), delta)
	default:
		action = fmt.Sprintf("Hits are ~%.1f ms late. Lower Offset by %d ms.", round1(r.Median), -delta)
	}

	lines := []string{}
	if r.MapTitle != "" {
		lines = append(lines, fmt.Sprintf("Map:            %s", r.MapTitle))
	}
	lines = append(lines,
		fmt.Sprintf("Hits:           %d", r.Hits),
		fmt.Sprintf("Median error:   %+.1f ms", r.Median),
		fmt.Sprintf("Mean error:     %+.1f ms", r.Mean),
		fmt.Sprintf("Unstable rate:  %.1f", r.UnstableRate),
		fmt.Sprintf("Current Offset: %d ms", r.CurrentOffset),
		fmt.Sprintf("Recommend:      %d ms", r.Recommend),
		"",
		action,
	)
	return strings.Join(lines, "\n")
}

func renderCalibrationPreview(current int, median float64, recommend int, hasOffset bool, width int) string {
	if !hasOffset {
		return pterm.Warning.Sprint("Universal Offset not readable yet")
	}

	scale, curMark, recMark := offsetScale(current, recommend, width)
	labels := padLabels(current, recommend, width, curMark, recMark)

	var b strings.Builder
	b.WriteString(pterm.NewStyle(pterm.Bold).Sprintln("Offset calibration"))
	b.WriteString("  ")
	b.WriteString(pterm.NewStyle(pterm.FgGray).Sprint("current ●  recommended ◆"))
	b.WriteString("\n  ")
	b.WriteString(scale)
	b.WriteString("\n  ")
	b.WriteString(labels)
	b.WriteString("\n  ")
	b.WriteString(fmt.Sprintf("current %d ms  −  median %+.1f ms  =  %d ms",
		current, median, recommend))
	if recMark > curMark {
		b.WriteString("  ")
		b.WriteString(pterm.NewStyle(pterm.FgLightRed).Sprint("► lower Offset"))
	} else if recMark < curMark {
		b.WriteString("  ")
		b.WriteString(pterm.NewStyle(pterm.FgLightGreen).Sprint("► raise Offset"))
	} else {
		b.WriteString("  ")
		b.WriteString(pterm.NewStyle(pterm.FgLightGreen).Sprint("► no change"))
	}
	return b.String()
}

func padLabels(current, recommend, width, curMark, recMark int) string {
	line := make([]rune, width)
	for i := range line {
		line[i] = ' '
	}
	curLabel := fmt.Sprintf("%d", current)
	recLabel := fmt.Sprintf("%d", recommend)
	placeLabel(line, curMark, curLabel)
	if recommend != current {
		placeLabel(line, recMark, recLabel)
	}
	return string(line)
}

func placeLabel(line []rune, mark int, label string) {
	start := mark - len(label)/2
	if start < 0 {
		start = 0
	}
	if start+len(label) > len(line) {
		start = len(line) - len(label)
	}
	if start < 0 {
		return
	}
	for i, ch := range label {
		line[start+i] = ch
	}
}

func histogramSpan(errors []float64) float64 {
	half := 22.0
	span := half
	for _, e := range errors {
		span = math.Max(span, math.Abs(e)+2)
	}
	span = math.Ceil(span/5) * 5
	if span < 10 {
		span = 10
	}
	return span
}

func round1(v float64) float64 {
	return math.Round(v*10) / 10
}

func hitProgress(current, total, width int) string {
	if total <= 0 {
		total = 1
	}
	if width < 8 {
		width = 8
	}
	filled := current * width / total
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	return fmt.Sprintf("[%s] %d/%d hits", bar, current, total)
}

type liveBlock struct {
	w     io.Writer
	lines int
}

func (l *liveBlock) Update(s string) {
	s = strings.TrimRight(s, "\n")
	contentLines := strings.Split(s, "\n")
	n := len(contentLines)
	if l.lines > 0 {
		for i := 0; i < l.lines; i++ {
			fmt.Fprint(l.w, "\033[A\033[2K")
		}
	}
	for _, line := range contentLines {
		fmt.Fprintln(l.w, line)
	}
	l.lines = n
}

func (l *liveBlock) Clear() {
	if l.lines > 0 {
		for i := 0; i < l.lines; i++ {
			fmt.Fprint(l.w, "\033[A\033[2K")
		}
		l.lines = 0
	}
}
