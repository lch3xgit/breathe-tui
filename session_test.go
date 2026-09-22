package main

import (
	"testing"
	"time"
)

var testStart = time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)

func TestAdvanceAtExactDeadline(t *testing.T) {
	practice, _ := findPractice("calm")
	session, err := NewSession(practice, testStart)
	if err != nil {
		t.Fatal(err)
	}

	starts := session.Advance(testStart.Add(4 * time.Second))
	if len(starts) != 1 {
		t.Fatalf("Advance() returned %d phase starts, want 1", len(starts))
	}
	if session.CurrentPhaseIndex != 1 || session.CurrentPhase.Name != "Exhale" {
		t.Errorf("current phase = %d %q, want 1 Exhale", session.CurrentPhaseIndex, session.CurrentPhase.Name)
	}
	if !session.PhaseStartedAt.Equal(testStart.Add(4 * time.Second)) {
		t.Errorf("phase start = %s, want %s", session.PhaseStartedAt, testStart.Add(4*time.Second))
	}
	if !session.PhaseDeadline.Equal(testStart.Add(10 * time.Second)) {
		t.Errorf("deadline = %s, want %s", session.PhaseDeadline, testStart.Add(10*time.Second))
	}
}

func TestAdvanceCatchesUpWithoutDrift(t *testing.T) {
	practice, _ := findPractice("circular")
	session, err := NewSession(practice, testStart)
	if err != nil {
		t.Fatal(err)
	}

	starts := session.Advance(testStart.Add(7 * time.Second))
	if len(starts) != 3 {
		t.Fatalf("Advance() returned %d phase starts, want 3", len(starts))
	}
	if got, want := []string{starts[0].Phase.Name, starts[1].Phase.Name, starts[2].Phase.Name}, []string{"Exhale", "Inhale", "Exhale"}; !equalStrings(got, want) {
		t.Errorf("started phases = %v, want %v", got, want)
	}
	if session.CompletedCycles != 1 {
		t.Errorf("completed cycles = %d, want 1", session.CompletedCycles)
	}
	if !session.PhaseStartedAt.Equal(testStart.Add(6 * time.Second)) {
		t.Errorf("phase start = %s, want original schedule boundary at +6s", session.PhaseStartedAt)
	}
	if !session.PhaseDeadline.Equal(testStart.Add(8 * time.Second)) {
		t.Errorf("deadline = %s, want original schedule boundary at +8s", session.PhaseDeadline)
	}
}

func TestCycleCounting(t *testing.T) {
	practice, _ := findPractice("box")
	session, err := NewSession(practice, testStart)
	if err != nil {
		t.Fatal(err)
	}

	session.Advance(testStart.Add(16 * time.Second))
	if session.CompletedCycles != 1 {
		t.Errorf("completed cycles = %d, want 1", session.CompletedCycles)
	}
	if session.CurrentPhaseIndex != 0 || session.State != SessionRunning {
		t.Errorf("after one cycle index/state = %d/%v, want 0/running", session.CurrentPhaseIndex, session.State)
	}
}

func TestDurationTargetCompletesAtCycleBoundary(t *testing.T) {
	practice := Practice{
		Name: "Test", Slug: "test",
		Phases:         []Phase{{Name: "Inhale", Duration: 2 * time.Second}, {Name: "Exhale", Duration: 2 * time.Second}},
		TargetDuration: 5 * time.Second,
	}
	session, err := NewSession(practice, testStart)
	if err != nil {
		t.Fatal(err)
	}

	session.Advance(testStart.Add(5 * time.Second))
	if session.State != SessionRunning || session.CompletedCycles != 1 {
		t.Fatalf("at target state/cycles = %v/%d, want running/1", session.State, session.CompletedCycles)
	}

	starts := session.Advance(testStart.Add(8 * time.Second))
	if session.State != SessionCompleted {
		t.Errorf("at next cycle boundary state = %v, want completed", session.State)
	}
	if session.CompletedCycles != 2 {
		t.Errorf("completed cycles = %d, want 2", session.CompletedCycles)
	}
	if len(starts) != 1 || starts[0].Phase.Name != "Exhale" {
		t.Errorf("phase starts before completion = %v, want only Exhale", starts)
	}
}

func TestFourSevenEightCompletesAfterExactlyEightCycles(t *testing.T) {
	practice, _ := findPractice("478")
	cycleDuration := 19 * time.Second

	before, err := NewSession(practice, testStart)
	if err != nil {
		t.Fatal(err)
	}
	before.Advance(testStart.Add(8*cycleDuration - time.Nanosecond))
	if before.State != SessionRunning || before.CompletedCycles != 7 {
		t.Errorf("just before boundary state/cycles = %v/%d, want running/7", before.State, before.CompletedCycles)
	}

	atBoundary, err := NewSession(practice, testStart)
	if err != nil {
		t.Fatal(err)
	}
	atBoundary.Advance(testStart.Add(8 * cycleDuration))
	if atBoundary.State != SessionCompleted || atBoundary.CompletedCycles != 8 {
		t.Errorf("at boundary state/cycles = %v/%d, want completed/8", atBoundary.State, atBoundary.CompletedCycles)
	}

	atBoundary.Advance(testStart.Add(20 * cycleDuration))
	if atBoundary.CompletedCycles != 8 {
		t.Errorf("cycles after completion = %d, want 8", atBoundary.CompletedCycles)
	}
}

func TestPauseFreezesPhaseAndSessionProgress(t *testing.T) {
	practice, _ := findPractice("calm")
	session, err := NewSession(practice, testStart)
	if err != nil {
		t.Fatal(err)
	}

	pauseTime := testStart.Add(2 * time.Second)
	session.Pause(pauseTime)
	if session.State != SessionPaused {
		t.Fatalf("state = %v, want paused", session.State)
	}
	if got := session.PhaseRemaining(testStart.Add(time.Hour)); got != 2*time.Second {
		t.Errorf("phase remaining while paused = %s, want 2s", got)
	}
	if got := session.PhaseProgress(testStart.Add(time.Hour)); got != 0.5 {
		t.Errorf("phase progress while paused = %v, want 0.5", got)
	}
	if got := session.SessionRemaining(testStart.Add(time.Hour)); got != 298*time.Second {
		t.Errorf("session remaining while paused = %s, want 4m58s", got)
	}

	pausedAt := session.PausedAt
	if starts := session.Pause(testStart.Add(2 * time.Hour)); starts != nil {
		t.Errorf("duplicate Pause() returned %v, want nil", starts)
	}
	if !session.PausedAt.Equal(pausedAt) {
		t.Error("duplicate Pause() changed the pause timestamp")
	}
}

func TestResumeShiftsDeadlines(t *testing.T) {
	practice, _ := findPractice("calm")
	session, err := NewSession(practice, testStart)
	if err != nil {
		t.Fatal(err)
	}

	session.Pause(testStart.Add(2 * time.Second))
	if !session.Resume(testStart.Add(12 * time.Second)) {
		t.Fatal("Resume() = false, want true")
	}
	if session.State != SessionRunning {
		t.Errorf("state = %v, want running", session.State)
	}
	if !session.StartedAt.Equal(testStart.Add(10 * time.Second)) {
		t.Errorf("session start = %s, want +10s", session.StartedAt)
	}
	if !session.PhaseStartedAt.Equal(testStart.Add(10 * time.Second)) {
		t.Errorf("phase start = %s, want +10s", session.PhaseStartedAt)
	}
	if !session.PhaseDeadline.Equal(testStart.Add(14 * time.Second)) {
		t.Errorf("phase deadline = %s, want +14s", session.PhaseDeadline)
	}
	if session.Resume(testStart.Add(13 * time.Second)) {
		t.Error("duplicate Resume() = true, want false")
	}
}

func TestPauseTimeDoesNotCountTowardCompletion(t *testing.T) {
	practice := Practice{
		Name: "Test", Slug: "test",
		Phases:         []Phase{{Name: "Inhale", Duration: time.Second}, {Name: "Exhale", Duration: time.Second}},
		TargetDuration: 3 * time.Second,
	}
	session, err := NewSession(practice, testStart)
	if err != nil {
		t.Fatal(err)
	}

	session.Pause(testStart.Add(500 * time.Millisecond))
	session.Resume(testStart.Add(10500 * time.Millisecond))
	session.Advance(testStart.Add(4 * time.Second))
	if session.State != SessionRunning {
		t.Fatalf("state at original completion time = %v, want running", session.State)
	}
	if got, want := session.ScheduledEnd(), testStart.Add(14*time.Second); !got.Equal(want) {
		t.Fatalf("scheduled end = %s, want %s", got, want)
	}

	session.Advance(testStart.Add(14 * time.Second))
	if session.State != SessionCompleted || session.CompletedCycles != 2 {
		t.Errorf("state/cycles at shifted completion = %v/%d, want completed/2", session.State, session.CompletedCycles)
	}
}

func TestMultiplePausesDoNotAccumulateDrift(t *testing.T) {
	practice, _ := findPractice("calm")
	session, err := NewSession(practice, testStart)
	if err != nil {
		t.Fatal(err)
	}

	session.Pause(testStart.Add(time.Second))
	session.Resume(testStart.Add(4 * time.Second))
	session.Pause(testStart.Add(6 * time.Second))
	session.Resume(testStart.Add(11 * time.Second))

	if got, want := session.StartedAt, testStart.Add(8*time.Second); !got.Equal(want) {
		t.Errorf("session start = %s, want %s", got, want)
	}
	if got, want := session.PhaseDeadline, testStart.Add(12*time.Second); !got.Equal(want) {
		t.Errorf("phase deadline = %s, want %s", got, want)
	}
	if got, want := session.ScheduledEnd(), testStart.Add(308*time.Second); !got.Equal(want) {
		t.Errorf("scheduled end = %s, want %s", got, want)
	}

	session.Advance(testStart.Add(12 * time.Second))
	if session.CurrentPhase.Name != "Exhale" {
		t.Errorf("phase at shifted deadline = %q, want Exhale", session.CurrentPhase.Name)
	}
	if got, want := session.PhaseDeadline, testStart.Add(18*time.Second); !got.Equal(want) {
		t.Errorf("next deadline = %s, want %s", got, want)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
