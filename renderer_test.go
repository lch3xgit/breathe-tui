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

func TestFramedLayoutHasStablePhaseGeometry(t *testing.T) {
	practice, _ := findPractice("box")
	tests := []struct {
		name  string
		at    time.Duration
		label string
	}{
		{name: "inhale", at: 0, label: "INHALE"},
		{name: "hold full", at: 4 * time.Second, label: "HOLD FULL"},
		{name: "exhale", at: 8 * time.Second, label: "EXHALE"},
		{name: "hold empty", at: 12 * time.Second, label: "HOLD EMPTY"},
	}

	const width = 80
	var barStart, countdownStart int
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			session, err := NewSession(practice, testStart)
			if err != nil {
				t.Fatal(err)
			}
			frame := advanceLiveFrame(session, testStart.Add(test.at), width)
			line := frame[1]
			if !strings.HasPrefix(line, "│ ") {
				t.Fatalf("middle line = %q, want left frame marker", line)
			}
			barByte := strings.IndexAny(line, "█░")
			countdownByte := strings.Index(line, formatCountdown(session.PhaseRemaining(testStart.Add(test.at))))
			labelByte := strings.Index(line, test.label)
			if barByte < 0 || labelByte < 0 || countdownByte < 0 {
				t.Fatalf("middle line %q is missing bar, label, or countdown", line)
			}
			if !(barByte < countdownByte && countdownByte < labelByte) {
				t.Fatalf("middle line %q does not order bar, countdown, phase label", line)
			}
			gotBar := textWidth(line[:barByte])
			gotCountdown := textWidth(line[:countdownByte])
			if test.at == 0 {
				barStart, countdownStart = gotBar, gotCountdown
			}
			if gotBar != barStart || gotCountdown != countdownStart {
				t.Errorf("positions = bar %d, countdown %d; want %d, %d", gotBar, gotCountdown, barStart, countdownStart)
			}
			if got, want := phaseCue(session, testStart.Add(test.at)), formatCountdown(session.PhaseRemaining(testStart.Add(test.at)))+" · "+test.label; got != want {
				t.Errorf("phase cue = %q, want %q", got, want)
			}
		})
	}
}

func TestFramedHeaderRulesAndPausedPresentation(t *testing.T) {
	practice, _ := findPractice("box")
	session, err := NewSession(practice, testStart)
	if err != nil {
		t.Fatal(err)
	}
	session.Pause(testStart.Add(time.Second))
	frame := renderSession(session, testStart.Add(time.Hour), preferredFrameWidth)
	if !strings.HasPrefix(frame[0], "┌─") || !strings.Contains(frame[0], "PAUSED") {
		t.Errorf("header = %q, want framed PAUSED header", frame[0])
	}
	if !strings.Contains(frame[2], "space resume") {
		t.Errorf("controls = %q, want space resume", frame[2])
	}
	if !strings.Contains(frame[0], "─") || textWidth(frame[0]) != preferredFrameWidth {
		t.Errorf("header = %q, want width %d with decorative rule", frame[0], preferredFrameWidth)
	}
}

func TestWideFramesUsePreferredCapAndStableGeometry(t *testing.T) {
	practice, _ := findPractice("box")
	session, err := NewSession(practice, testStart)
	if err != nil {
		t.Fatal(err)
	}
	capped := renderSession(session, testStart, preferredFrameWidth)
	for _, width := range []int{80, 120, 200} {
		frame := renderSession(session, testStart, width)
		if strings.Join(frame, "\n") != strings.Join(capped, "\n") {
			t.Errorf("width %d frame differs from capped geometry:\n%q\nwant\n%q", width, frame, capped)
		}
		for _, line := range frame {
			if got := textWidth(line); got > preferredFrameWidth {
				t.Errorf("width %d frame line has width %d, cap %d", width, got, preferredFrameWidth)
			}
		}
	}
}

func TestFramedRenderingNeverExceedsWidth(t *testing.T) {
	practice, _ := findPractice("478")
	session, err := NewSession(practice, testStart)
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []int{1, 2, 8, 16, 27, 28, 44, 80} {
		t.Run(strconv.Itoa(width), func(t *testing.T) {
			frame := renderSession(session, testStart, width)
			for _, line := range frame {
				if got := textWidth(line); got > width {
					t.Errorf("line width = %d, want <= %d: %q", got, width, line)
				}
			}
		})
	}
}

func TestSummaryUsesActiveTimeAndCompletedDuration(t *testing.T) {
	practice, _ := findPractice("box")
	session, err := NewSession(practice, testStart)
	if err != nil {
		t.Fatal(err)
	}
	session.Pause(testStart.Add(10 * time.Second))
	session.Resume(testStart.Add(110 * time.Second))
	if got, want := sessionSummary(session, testStart.Add(142*time.Second), false), "breathe · BOX BREATH · ended after 00:42"; got != want {
		t.Errorf("early summary = %q, want %q", got, want)
	}
	session.Advance(session.ScheduledEnd())
	if got, want := sessionSummary(session, session.ScheduledEnd(), true), "breathe · BOX BREATH · complete · 05:04"; got != want {
		t.Errorf("completion summary = %q, want %q", got, want)
	}
}
