package main

import (
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestProgressBarPhaseBehavior(t *testing.T) {
	tests := []struct {
		name       string
		phase      string
		progress   float64
		wantFilled int
	}{
		{name: "inhale begins empty", phase: "Inhale", progress: 0, wantFilled: 0},
		{name: "inhale fills", phase: "Inhale", progress: 0.75, wantFilled: 6},
		{name: "exhale begins full", phase: "Exhale", progress: 0, wantFilled: 8},
		{name: "exhale empties", phase: "Exhale", progress: 0.75, wantFilled: 2},
		{name: "hold full stays full", phase: "Hold full", progress: 0.75, wantFilled: 8},
		{name: "hold empty stays empty", phase: "Hold empty", progress: 0.25, wantFilled: 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bar := progressBar(test.phase, test.progress, 8)
			if got := strings.Count(bar, "█"); got != test.wantFilled {
				t.Errorf("filled cells = %d, want %d; bar = %q", got, test.wantFilled, bar)
			}
			if got := textWidth(bar); got != 8 {
				t.Errorf("bar width = %d, want 8", got)
			}
		})
	}
}

func TestDelayedFrameUsesOnlyFinalLiveState(t *testing.T) {
	practice, _ := findPractice("box")
	session, err := NewSession(practice, testStart)
	if err != nil {
		t.Fatal(err)
	}

	frame := strings.Join(advanceLiveFrame(session, testStart.Add(13*time.Second), 80), "\n")
	if session.CurrentPhase.Name != "Hold empty" {
		t.Fatalf("current phase = %q, want Hold empty", session.CurrentPhase.Name)
	}
	if !strings.Contains(frame, "HOLD EMPTY") {
		t.Errorf("frame %q does not contain final phase", frame)
	}
	for _, stale := range []string{"INHALE", "HOLD FULL", "EXHALE"} {
		if strings.Contains(frame, stale) {
			t.Errorf("frame %q contains stale phase %q", frame, stale)
		}
	}
}

func TestFourSevenEightShowsCycleProgress(t *testing.T) {
	practice, _ := findPractice("478")
	session, err := NewSession(practice, testStart)
	if err != nil {
		t.Fatal(err)
	}

	frame := strings.Join(advanceLiveFrame(session, testStart.Add(38*time.Second), 80), "\n")
	if !strings.Contains(frame, "breath 3/8") {
		t.Errorf("frame %q does not show breath 3/8", frame)
	}
}

func TestNarrowRenderingNeverExceedsWidth(t *testing.T) {
	practice, _ := findPractice("coherence")
	session, err := NewSession(practice, testStart)
	if err != nil {
		t.Fatal(err)
	}
	session.Pause(testStart.Add(time.Second))

	for _, width := range []int{1, 4, 8, 12, 20, 35} {
		t.Run(strconv.Itoa(width), func(t *testing.T) {
			lines := renderSession(session, testStart.Add(time.Hour), width)
			if len(lines) > 3 {
				t.Fatalf("rendered %d lines, want at most 3", len(lines))
			}
			for i, line := range lines {
				if got := textWidth(line); got > width {
					t.Errorf("line %d width = %d, want <= %d: %q", i, got, width, line)
				}
			}
		})
	}
}
