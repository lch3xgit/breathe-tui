package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestLineOutputContainsNoCursorControls(t *testing.T) {
	practice, _ := findPractice("coherence")
	var output bytes.Buffer
	lineOutput := LineOutput{Writer: &output}
	lineOutput.Started(practice)
	lineOutput.PhaseStarted(PhaseStart{Phase: practice.Phases[0]})
	lineOutput.Completed(practice, 1)

	if strings.Contains(output.String(), "\x1b[") {
		t.Errorf("non-interactive output contains ANSI cursor controls: %q", output.String())
	}
}

func TestTerminalCleanupIsIdempotent(t *testing.T) {
	var output bytes.Buffer
	restoreCalls := 0
	lifecycle := &terminalLifecycle{
		writer: &output,
		restoreRaw: func() error {
			restoreCalls++
			return nil
		},
	}

	if err := lifecycle.hideCursor(); err != nil {
		t.Fatal(err)
	}
	if err := lifecycle.restore(); err != nil {
		t.Fatal(err)
	}
	if err := lifecycle.restore(); err != nil {
		t.Fatal(err)
	}

	if restoreCalls != 1 {
		t.Errorf("raw restore calls = %d, want 1", restoreCalls)
	}
	if got := strings.Count(output.String(), ansiShowCursor); got != 1 {
		t.Errorf("show-cursor writes = %d, want 1; output = %q", got, output.String())
	}
	if got := strings.Count(output.String(), ansiReset); got != 1 {
		t.Errorf("style resets = %d, want 1; output = %q", got, output.String())
	}
	if !strings.HasSuffix(output.String(), "\r\n") {
		t.Errorf("cleanup output %q does not end with a clean new line", output.String())
	}
}

func TestWordmarkRequiresEnoughTerminalSpace(t *testing.T) {
	lines := strings.Split(wordmark, "\n")
	if len(lines) != 3 {
		t.Fatalf("wordmark has %d lines, want 3", len(lines))
	}
	width := 0
	for _, line := range lines {
		if lineWidth := textWidth(line); lineWidth > width {
			width = lineWidth
		}
	}
	if !wordmarkFits(width, 3) {
		t.Error("wordmark does not fit at its exact dimensions")
	}
	if wordmarkFits(width-1, 3) {
		t.Error("wordmark fits in a terminal that is too narrow")
	}
	if wordmarkFits(width, 2) {
		t.Error("wordmark fits in a terminal that is too short")
	}
}
