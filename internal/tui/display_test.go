package tui

import (
	"strings"
	"testing"

	"github.com/pterm/pterm"
)

func TestTrimGraphicBlockDropsTrailingBlankFromMinus(t *testing.T) {
	big, err := pterm.DefaultBigText.WithLetters(
		pterm.NewLettersFromStringWithStyle("-39 ms", pterm.NewStyle(pterm.FgLightGreen, pterm.Bold)),
	).Srender()
	if err != nil {
		t.Fatal(err)
	}

	trimmed := trimGraphicBlock(big)
	lines := strings.Split(trimmed, "\n")
	if len(lines) != 5 {
		t.Fatalf("lines=%d want 5", len(lines))
	}
	if isBlankGraphicLine(lines[len(lines)-1]) {
		t.Fatalf("trailing line should not be blank: %q", lines[len(lines)-1])
	}
}

func TestRenderRecommendedOffsetTextPadding(t *testing.T) {
	boxed, err := renderRecommendedOffsetText(-39)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(boxed, "\n"), "\n")
	// top border, padded blank, 5 glyph rows, padded blank, bottom border
	if len(lines) != 9 {
		t.Fatalf("lines=%d want 9\n%s", len(lines), boxed)
	}
}
