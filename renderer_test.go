package main

import (
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestBracketedMeterPhaseBehavior(t *testing.T) {
	const interiorWidth = preferredMeterInteriorWidth
	tests := []struct {
		name       string
		phase      string
		progress   float64
		wantFilled int
	}{
		{name: "inhale begins empty", phase: "Inhale", progress: 0, wantFilled: 0},
		{name: "inhale fills", phase: "Inhale", progress: 0.75, wantFilled: 15},
		{name: "exhale begins full", phase: "Exhale", progress: 0, wantFilled: 20},
		{name: "exhale empties", phase: "Exhale", progress: 0.75, wantFilled: 5},
		{name: "hold full stays full", phase: "Hold full", progress: 0.75, wantFilled: 20},
		{name: "hold empty stays empty", phase: "Hold empty", progress: 0.25, wantFilled: 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			meter := breathMeter(test.phase, test.progress, interiorWidth)
			if !strings.HasPrefix(meter, "[ ") || !strings.HasSuffix(meter, " ]") {
				t.Fatalf("meter %q is missing brackets or inner padding", meter)
			}
			if got := strings.Count(meter, "░"); got != test.wantFilled {
				t.Errorf("filled cells = %d, want %d; meter = %q", got, test.wantFilled, meter)
			}
			if got, want := textWidth(meter), interiorWidth+meterFixedWidth; got != want {
				t.Errorf("meter width = %d, want %d", got, want)
			}
			track := meterTrack(meter)
			wantTrack := strings.Repeat("░", test.wantFilled) + strings.Repeat(" ", interiorWidth-test.wantFilled)
			if track != wantTrack {
				t.Errorf("track = %q, want contiguous left fill %q", track, wantTrack)
			}
			if strings.Contains(meter, "|") {
				t.Errorf("meter contains stale ASCII fill: %q", meter)
			}
		})
	}
}

func TestBracketedMeterProgressIsMonotonic(t *testing.T) {
	const interiorWidth = preferredMeterInteriorWidth
	previousInhale := -1
	previousExhale := interiorWidth + 1
	for step := 0; step <= 20; step++ {
		progress := float64(step) / 20
		inhale := breathMeter("Inhale", progress, interiorWidth)
		exhale := breathMeter("Exhale", progress, interiorWidth)
		inhaleFilled := strings.Count(inhale, "░")
		exhaleFilled := strings.Count(exhale, "░")
		if inhaleFilled < previousInhale {
			t.Errorf("inhale moved backward at %.2f: %d after %d", progress, inhaleFilled, previousInhale)
		}
		if exhaleFilled > previousExhale {
			t.Errorf("exhale refilled at %.2f: %d after %d", progress, exhaleFilled, previousExhale)
		}
		previousInhale = inhaleFilled
		previousExhale = exhaleFilled
		for _, meter := range []string{inhale, exhale} {
			track := meterTrack(meter)
			filled := strings.Count(track, "░")
			if want := strings.Repeat("░", filled) + strings.Repeat(" ", interiorWidth-filled); track != want {
				t.Errorf("meter contains a gap: %q", meter)
			}
		}
	}
}

func TestBracketedMeterShrinksForNarrowFrames(t *testing.T) {
	if preferredMeterInteriorWidth != 20 {
		t.Fatalf("preferred meter interior = %d, want 20", preferredMeterInteriorWidth)
	}
	if got := textWidth(breathMeter("Hold empty", 0, preferredMeterInteriorWidth)); got != 24 {
		t.Fatalf("preferred meter width = %d, want 24", got)
	}
	if got := boundedMeterInteriorWidth(100); got != preferredMeterInteriorWidth {
		t.Errorf("preferred meter interior = %d, want %d", got, preferredMeterInteriorWidth)
	}
	for _, available := range []int{1, 2, 8, 20, 100} {
		interiorWidth := boundedMeterInteriorWidth(available)
		if interiorWidth > available || interiorWidth > preferredMeterInteriorWidth {
			t.Errorf("interior width %d exceeds available %d", interiorWidth, available)
		}
		if interiorWidth == 0 {
			continue
		}
		meter := breathMeter("Hold empty", 0, interiorWidth)
		if !strings.HasPrefix(meter, "[ ") || !strings.HasSuffix(meter, " ]") {
			t.Errorf("narrow meter %q is missing brackets or padding", meter)
		}
	}
}

func meterTrack(meter string) string {
	runes := []rune(meter)
	if len(runes) < meterFixedWidth {
		return ""
	}
	return string(runes[2 : len(runes)-2])
}

func TestPausedFrameRetainsFrozenMeter(t *testing.T) {
	practice, _ := findPractice("box")
	session, err := NewSession(practice, testStart)
	if err != nil {
		t.Fatal(err)
	}
	session.Pause(testStart.Add(2 * time.Second))
	first := renderSession(session, testStart.Add(2*time.Second), preferredFrameWidth)[1]
	later := renderSession(session, testStart.Add(time.Hour), preferredFrameWidth)[1]
	if meterFromMiddleLine(first) != meterFromMiddleLine(later) {
		t.Errorf("paused meter changed from %q to %q", first, later)
	}
	if !strings.Contains(first, " 2.0s · INHALE") || !strings.Contains(later, " 2.0s · INHALE") {
		t.Errorf("paused phase cue changed: first %q, later %q", first, later)
	}
}

func TestNarrowFramedMeterRetainsBracketsAndSeparators(t *testing.T) {
	practice, _ := findPractice("box")
	session, err := NewSession(practice, testStart)
	if err != nil {
		t.Fatal(err)
	}
	line := renderSession(session, testStart, framedMinimum)[1]
	meter := meterFromMiddleLine(line)
	if !strings.HasPrefix(meter, "[ ") || !strings.HasSuffix(meter, " ]") {
		t.Fatalf("narrow framed meter %q is malformed in %q", meter, line)
	}
	if got := strings.Count(line, " · "); got != 2 {
		t.Errorf("narrow framed separator count = %d, want 2: %q", got, line)
	}
	if got := textWidth(line); got > framedMinimum {
		t.Errorf("narrow framed line width = %d, want <= %d: %q", got, framedMinimum, line)
	}
}

func meterFromMiddleLine(line string) string {
	start := strings.Index(line, "[")
	end := strings.Index(line, "]")
	if start < 0 || end < start {
		return ""
	}
	return line[start : end+1]
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
			barByte := strings.Index(line, "[")
			meterEnd := strings.Index(line, "]")
			countdownByte := strings.Index(line, formatCountdown(session.PhaseRemaining(testStart.Add(test.at))))
			labelByte := strings.Index(line, test.label)
			if barByte < 0 || meterEnd < barByte || labelByte < 0 || countdownByte < 0 {
				t.Fatalf("middle line %q is missing bar, label, or countdown", line)
			}
			if !(barByte < meterEnd && meterEnd < countdownByte && countdownByte < labelByte) {
				t.Fatalf("middle line %q does not order meter, countdown, phase label", line)
			}
			if got := line[meterEnd+1 : countdownByte]; got != " · " {
				t.Errorf("meter/countdown separator = %q, want %q", got, " · ")
			}
			if got := strings.Count(line, " · "); got != 2 {
				t.Errorf("middle line separator count = %d, want 2: %q", got, line)
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
