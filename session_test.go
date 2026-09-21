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
